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
	ModePlayback                Mode = "playback"
	ModeRecord                  Mode = "record"
	ModePlaybackOrRecord        Mode = "playback_or_record"
	ModePlaybackSuccessOrRecord Mode = "playback_success_or_record"
)

type SyncMode string

const (
	SyncModeDefault     SyncMode = ""
	SyncModeEveryChange SyncMode = "EveryTime"
)
