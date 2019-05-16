package playback

import (
	"context"
	"database/sql/driver"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/spf13/viper"
)

func Default() *Playback {
	return &Playback{
		Mode:            Mode(viper.GetString(FlagPlaybackMode)),
		ExcludeHeaderRE: regexp.MustCompile("-Trace$|id$"),
		Debounce:        2 * time.Second,
	}
}

type Playback struct {
	Mode            Mode
	ExcludeHeaderRE *regexp.Regexp
	Debounce        time.Duration
	fileMask        string
	withFile        bool
	Error           error
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

func (p *Playback) NewContext(ctx context.Context) context.Context {
	c, err := p.NewCassette()
	if err != nil {
		p.Error = err
	}

	return context.WithValue(ctx, contextKey, c)
}

func (p *Playback) NewContextWithCassette(ctx context.Context, cassette *Cassette) context.Context {
	return context.WithValue(ctx, contextKey, cassette)
}

func (p *Playback) CassetteFromFile(filename string) (*Cassette, error) {
	c, err := newCassetteFromFile(p, filename)
	return c, err
}

func (p *Playback) WithFile() *Playback {
	// TODO Lock
	p.withFile = true
	return p
}

func (p *Playback) newFileForCassette() (*os.File, error) {
	return ioutil.TempFile("", p.fileMask)
}

func (p *Playback) SetMode(mode Mode) {
	// TODO Lock
	p.Mode = mode
}

func (p *Playback) HTTPTransport(transport http.RoundTripper) http.RoundTripper {
	return httpPlayback{
		Real:     transport,
		playback: p,
	}
}

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

type Recorder interface {
	Call() error
	Record() error
	Playback() error
}

func (p *Playback) Run(recorder Recorder) error {
	switch p.Mode {
	case ModeOff:
		return recorder.Call()
	case ModePlayback:
		return recorder.Playback()

	case ModePlaybackOrRecord:
		err := recorder.Playback()
		if err == ErrPlaybackFailed {
			return recorder.Record()
		}
		return err

	case ModePlaybackSuccessOrRecord:
		err := recorder.Playback()
		if err != nil {
			return recorder.Record()
		}
		return err

	case ModeRecord:
		return recorder.Record()

	}

	return recorder.Call()
}
