package playback

type Mode string

const (
	ModePlayback                Mode = "Playback"
	ModeRecord                  Mode = "Record"
	ModePlaybackOrRecord        Mode = "PlaybackOrRecord"
	ModePlaybackSuccessOrRecord Mode = "PlaybackSuccessOrRecord"
)
