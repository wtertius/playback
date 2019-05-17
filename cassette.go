package playback

import (
	"bufio"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

var errCassetteGetFailed = errors.New("Cassette get failed")
var errCassetteLocked = errors.New("Cassette locked")

type track struct {
	cursor  int
	records []*record
}

type Cassette struct {
	writer     Writer
	playback   *Playback
	tracks     map[RecordKind]map[string]*track
	err        error
	recID      uint64
	recordByID map[uint64]*record
	locked     bool
}

func newCassette(p *Playback) *Cassette {
	c := &Cassette{playback: p}
	c.reset()

	return c
}

func newCassetteFromFile(p *Playback, filename string) (*Cassette, error) {
	c := &Cassette{playback: p}
	c.writer = newNilNamed(PathTypeFile, filename)

	dump, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if len(dump) == 0 {
		return nil, ErrPlaybackFailed
	}

	var records []*record
	err = yaml.Unmarshal(dump, &records)
	if err != nil {
		return nil, err
	}

	c.reset()

	for _, rec := range records {
		// TODO mutex.Lock
		c.add(rec)
	}

	return c, nil
}

func (c *Cassette) Result(key string, value interface{}) interface{} {
	recorder := newResultRecorder(c, key, value)

	c.playback.Run(recorder)

	return recorder.value
}

func (c *Cassette) WithFile() (*Cassette, error) {
	var err error
	c.writer, err = c.playback.newFileForCassette()
	return c, err
}

func (c *Cassette) Rewind() {
	c.err = nil

	c.recordByID = make(map[uint64]*record, 10)

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}
func (c *Cassette) reset() {
	c.recID = 0
	c.err = nil
	c.recordByID = make(map[uint64]*record, 10)
	c.tracks = make(map[RecordKind]map[string]*track, 5)

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}

func (c *Cassette) Lock() {
	// TODO mu.Lock
	c.locked = true

	if c.writer != nil {
		c.writer.Sync()
	}
}

func (c *Cassette) Unlock() {
	// TODO mu.Lock
	c.locked = false
}

func (c *Cassette) Error() error {
	return c.err
}

func (c *Cassette) IsPlaybackSucceeded() bool {
	if c.playback.Mode != ModePlayback || c.err != nil {
		return false
	}

	for kind, kindTracks := range c.tracks {
		if kind == KindHTTPRequest {
			continue
		}

		for _, keyTrack := range kindTracks {
			if keyTrack.cursor != len(keyTrack.records) {
				return false
			}
		}
	}

	return true
}

func (c *Cassette) write(content string) error {
	if c.writer == nil || c.writer.ReadOnly() {
		return nil
	}

	_, err := io.WriteString(c.writer, content)
	if err != nil {
		return err
	}

	err = c.writer.Sync()
	if err != nil {
		return err
	}

	return nil
}

func (c *Cassette) Finalize() error {
	c.Lock()

	return c.writer.Close()
}

func (c *Cassette) PathType() PathType {
	if c.writer == nil {
		return PathTypeNil
	}

	return c.writer.Type()
}

func (c *Cassette) PathName() string {
	if c.writer == nil {
		return ""
	}

	return c.writer.Name()
}

func (c *Cassette) setID(rec *record) {
	if rec.ID == 0 {
		rec.setID(c.nextRecordID())
	}
}

func (c *Cassette) nextRecordID() uint64 {
	// TODO mutex.Lock ?
	c.recID++
	return c.recID
}

func (c *Cassette) HTTPRequest() (*http.Request, error) {
	rec, err := c.Get(KindHTTPRequest, "")
	if err != nil {
		return nil, err
	}

	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rec.RequestDump)))
	return req, err
}

func (c *Cassette) SetHTTPRequest(req *http.Request) {
	rec := (&httpPlayback{}).newRecord(req)
	rec.Kind = KindHTTPRequest
	rec.Key = ""
	c.Add(rec)
}

func (c *Cassette) Get(kind RecordKind, key string) (rec *record, err error) {
	// TODO mutex.RLock
	if c.tracks[kind] == nil || c.tracks[kind][key] == nil {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	track := c.tracks[kind][key]
	if len(track.records) <= track.cursor {
		c.err = errCassetteGetFailed
		return nil, errCassetteGetFailed
	}

	rec = track.records[track.cursor]
	track.cursor++

	return rec, nil
}

func (c *Cassette) Add(rec *record) error {
	// TODO mutex.Lock
	if c.locked {
		c.err = errCassetteLocked
		return errCassetteLocked
	}

	c.add(rec)
	marshalled := yamlMarshal([]*record{rec})
	return c.write(marshalled)
}

func (c *Cassette) add(rec *record) {
	c.setID(rec)
	if c.recordByID[rec.ID] != nil {
		*(c.recordByID[rec.ID]) = *rec
		return
	}

	c.recordByID[rec.ID] = rec

	if c.tracks[rec.Kind] == nil {
		c.tracks[rec.Kind] = make(map[string]*track, 5)
	}
	if c.tracks[rec.Kind][rec.Key] == nil {
		c.tracks[rec.Kind][rec.Key] = &track{
			cursor:  0,
			records: make([]*record, 0, 2),
		}
	}
	track := c.tracks[rec.Kind][rec.Key]
	track.records = append(track.records, rec)
}
