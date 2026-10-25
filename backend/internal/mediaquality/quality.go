// Package mediaquality defines quality labels shared by discovery and media
// acceptance. These values must not depend on an indexer, transport or database.
package mediaquality

import "strings"

// Height returns the required vertical resolution, or zero for an unknown label.
// Callers decide whether an unknown value is an error; it never grants acceptance.
func Height(quality string) int {
	switch strings.ToLower(quality) {
	case "480p":
		return 480
	case "576p":
		return 576
	case "720p":
		return 720
	case "900p":
		return 900
	case "1080p", "fhd":
		return 1080
	case "1440p":
		return 1440
	case "2160p", "4k", "uhd":
		return 2160
	case "4320p", "8k":
		return 4320
	}
	return 0
}
