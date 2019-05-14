package playback

import (
	"errors"
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"
)

var errCassetteGetFailed = errors.New("Cassette get failed")

type track struct {
	cursor  int
	records []*record
}
type cassette struct {
	file      *os.File
	tracks    map[RecordKind]map[string]*track
	playError bool
}

func (c *cassette) Reset() {
	c.playError = false
	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			keyTrack.cursor = 0
		}
	}
}

func (c *cassette) CheckPlay() bool {
	if c.playError {
		return false
	}

	for _, kindTracks := range c.tracks {
		for _, keyTrack := range kindTracks {
			if keyTrack.cursor != len(keyTrack.records) {
				return false
			}
		}
	}

	return true
}

func (c *cassette) Write(content string) error {
	_, err := c.file.WriteString(content)
	if err != nil {
		return err
	}

	err = c.file.Sync()
	if err != nil {
		return err
	}

	return nil
}

func (c *cassette) UnmarshalFromFile() ([]*record, error) {
	request, err := ioutil.ReadFile(c.casseteFile().Name())
	if err != nil {
		return nil, err
	}

	if len(request) == 0 {
		return nil, errPlaybackFailed
	}

	var records []*record
	err = yaml.Unmarshal(request, &records)
	if err != nil {
		return nil, err
	}

	c.tracks = nil

	for _, rec := range records {
		c.add(rec)
	}

	return records, nil
}

func (c *cassette) casseteFile() *os.File {
	return c.file
}

func (c *cassette) FileName() string {
	return c.file.Name()
}

func (c *cassette) Get(kind RecordKind, key string) (r *record, err error) {
	if c.tracks[kind] == nil || c.tracks[kind][key] == nil {
		c.playError = true
		return nil, errCassetteGetFailed
	}

	track := c.tracks[kind][key]
	if len(track.records) <= track.cursor {
		c.playError = true
		return nil, errCassetteGetFailed
	}

	rec := track.records[track.cursor]
	track.cursor++

	return rec, nil
}

func (c *cassette) Add(rec *record) error {
	c.add(rec)
	marshalled := yamlMarshal([]*record{rec})
	return c.Write(marshalled)
}

func (c *cassette) add(rec *record) error {
	if c.tracks == nil {
		c.tracks = make(map[RecordKind]map[string]*track, 5)
	}
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

	return nil
}
