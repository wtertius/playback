package playback

import (
	"net/http"

	"github.com/spf13/viper"
)

func Default() *Playback {
	return &Playback{
		defaultMode: Mode(viper.GetString(FlagPlaybackMode)),
	}
}

type Playback struct {
	Error error

	defaultMode Mode
	fileMask    string
	withFile    bool
	cassettes   map[string]*Cassette
}

type Option func(*Playback)

func New(opts ...Option) *Playback {
	p := &Playback{
		fileMask:  FileMask,
		cassettes: make(map[string]*Cassette),
	}

	return p
}

func (p *Playback) NewCassette() (*Cassette, error) {
	cassette := newCassette(p)

	if p.defaultMode == ModeOff {
		return cassette, nil
	}

	if p.withFile {
		return cassette.WithFile()
	}

	return cassette, nil
}

func (p *Playback) CassetteFromFile(filename string) (*Cassette, error) {
	return newCassetteFromFile(p, filename)
}

func (p *Playback) CassetteFromYAML(yamlBody []byte) (*Cassette, error) {
	return newCassetteFromYAML(p, yamlBody)
}

func (p *Playback) Mode() Mode {
	// TODO mutex.RLock
	return p.defaultMode
}

func (p *Playback) WithFile() *Playback {
	// TODO Lock
	p.withFile = true
	return p
}

func (p *Playback) SetDefaultMode(mode Mode) *Playback {
	// TODO Lock
	p.defaultMode = mode

	return p
}

func (p *Playback) HTTPTransport(transport http.RoundTripper) http.RoundTripper {
	return httpPlayback{
		Real:     transport,
		playback: p,
	}
}

/* FIXME Remove or repair
func (p *Playback) SQLRows(query string, args []driver.NamedValue, f func() (driver.Rows, error)) (driver.Rows, error) {
	recorder := newSQLRowsRecorder(query, args, f)

	p.Run(recorder)

	return recorder.rows, recorder.err
}

func (p *Playback) SQLResult(query string, args []driver.NamedValue, f func() (driver.Result, error)) (driver.Result, error) {
	recorder := newSQLResultRecorder(query, args, f)

	p.Run(recorder)

	return recorder.result, recorder.err
}
*/

func (p *Playback) generateID() string {
	id := RandStringRunes(6)
	if p.cassettes[id] != nil {
		return p.generateID()
	}

	return id
}

func (p *Playback) List() map[string]*Cassette {
	return p.cassettes
}

func (p *Playback) Add(cassette *Cassette) {
	p.cassettes[cassette.ID] = cassette
}

func (p *Playback) Get(cassetteID string) *Cassette {
	return p.cassettes[cassetteID]
}

type Recorder interface {
	Call() error
	Record() error
	Playback() error
}
