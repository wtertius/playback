package playback

import (
	"net/http"
	"sync"

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
	debug       bool
	logger      Logger
	fileMask    string
	withFile    bool
	cassettes   map[string]*Cassette

	mu sync.RWMutex
}

type Option func(*Playback)

func New(opts ...Option) *Playback {
	p := &Playback{
		fileMask:  FileMask,
		cassettes: make(map[string]*Cassette),
		logger:    &defaultLogger{},
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
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.defaultMode
}

func (p *Playback) WithFile() *Playback {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.withFile = true
	return p
}

func (p *Playback) SetDefaultMode(mode Mode) *Playback {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.defaultMode = mode

	return p
}

func (p *Playback) SetDebug(debug bool) *Playback {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.debug = debug

	return p
}

func (p *Playback) Debug() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.debug
}

func (p *Playback) SetLogger(logger Logger) *Playback {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger = logger

	return p
}

func (p *Playback) getLogger() Logger {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.logger
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
	p.mu.RLock()
	defer p.mu.RUnlock()

	id := RandStringRunes(6)
	if p.cassettes[id] != nil {
		return p.generateID()
	}

	return id
}

func (p *Playback) List() map[string]*Cassette {
	p.mu.RLock()
	defer p.mu.RUnlock()

	list := make(map[string]*Cassette, len(p.cassettes))
	for id, cassette := range p.cassettes {
		list[id] = cassette
	}

	return list
}

func (p *Playback) Add(cassette *Cassette) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cassettes[cassette.ID] = cassette
}

func (p *Playback) Get(cassetteID string) *Cassette {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.cassettes[cassetteID]
}

type Recorder interface {
	Call() error
	Record() error
	Playback() error
}
