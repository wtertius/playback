package playback

import "net/http"

var Default = Playback{
	On:   true,
	Mode: ModePlayback,
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
