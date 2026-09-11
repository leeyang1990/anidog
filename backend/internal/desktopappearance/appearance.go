// Package desktopappearance contains optional native desktop presentation only.
package desktopappearance

import "runtime"

type State struct {
	Platform           string `json:"platform"`
	Mode               string `json:"mode"`
	ReduceTransparency bool   `json:"reduce_transparency"`
	ReduceMotion       bool   `json:"reduce_motion"`
}

func stateFromFlags(flags int) State {
	mode := "none"
	switch flags & 3 {
	case 1:
		mode = "liquid"
	case 2:
		mode = "vibrancy"
	case 3:
		mode = "solid"
	}
	return State{Platform: runtime.GOOS, Mode: mode, ReduceTransparency: flags&4 != 0, ReduceMotion: flags&8 != 0}
}
