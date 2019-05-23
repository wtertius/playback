package playback

import (
	"io/ioutil"
	"net/http"

	"github.com/spf13/viper"
)

func Default() *Playback {
	return &Playback{
		defaultMode: Mode(viper.GetString(FlagPlaybackMode)),
	}
}

type Playback struct {
	defaultMode Mode
	fileMask    string
	withFile    bool
	Error       error
}

type Option func(*Playback)

func New(opts ...Option) *Playback {
	p := &Playback{
		fileMask: FileMask,
	}

	return p
}

func (p *Playback) NewCassette() (*Cassette, error) {
	cassette := newCassette(p)
	if p.withFile {
		return cassette.WithFile()
	}

	return cassette, nil
}

func (p *Playback) CassetteFromFile(filename string) (*Cassette, error) {
	c, err := newCassetteFromFile(p, filename)
	return c, err
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

func (p *Playback) newFileForCassette() (*file, error) {
	f, err := ioutil.TempFile("", p.fileMask)
	return &file{f}, err
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

type Recorder interface {
	Call() error
	Record() error
	Playback() error
}
