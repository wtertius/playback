package playback

import "net/http"

var Default = Playback{
	On:   true,
	Mode: ModePlaybackOrRecord,
	//Mode: ModeRecord,
}

type Playback struct {
	On   bool
	Mode Mode
}

func (p *Playback) HTTPTransport(transport http.RoundTripper) http.RoundTripper {
	return httpPlayback{
		Real:     transport,
		playback: p,
	}
}

type Recorder interface {
	Call() error
	Record() error
	Playback() error
}

func (p *Playback) Run(recorder Recorder) error {
	if !p.On {
		return recorder.Call()
	}

	switch p.Mode {
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

	return recorder.Record()
}
