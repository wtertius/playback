package playback

import (
	"flag"
)

const FlagPlaybackMode = "playback_mode"

func init() {
	flag.String(FlagPlaybackMode, "", "Playback mode")
}

type Mode string

const (
	ModeOff                     Mode = ""
	ModePlayback                Mode = "Playback"
	ModeRecord                  Mode = "Record"
	ModePlaybackOrRecord        Mode = "PlaybackOrRecord"
	ModePlaybackSuccessOrRecord Mode = "PlaybackSuccessOrRecord"
)

type SyncMode string

const (
	SyncModeDefault     SyncMode = ""
	SyncModeEveryChange SyncMode = "EveryTime"
)
