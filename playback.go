package playback

import (
	"database/sql/driver"
	"net/http"
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
}

func (p *Playback) HTTPTransport(transport http.RoundTripper) http.RoundTripper {
	return httpPlayback{
		Real:     transport,
		playback: p,
	}
}

func (p *Playback) Generated(key string, value interface{}) interface{} {
	recorder := newGeneratedRecorder(key, value)

	p.Run(recorder)

	return recorder.value
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
		if err == errPlaybackFailed {
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
