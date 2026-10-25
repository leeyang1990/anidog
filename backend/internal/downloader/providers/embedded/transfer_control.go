package embedded

import atorrent "github.com/anacrolix/torrent"

// setDownloadDemand is the single translation from AniDog's active/queued/
// paused intent to the engine's piece demand and permission. The library's
// DisallowDataDownload does not withdraw pending piece demand. Denying traffic
// first leaves peer scheduling with inconsistent pending-piece bookkeeping.
// Withdraw demand before denying data, and re-publish it only after enabling
// data. CancelPieces changes priorities; it does not discard retained bytes.
func setDownloadDemand(torrent *atorrent.Torrent, enabled bool) {
	if enabled {
		torrent.AllowDataDownload()
		if torrent.Info() != nil {
			torrent.DownloadAll()
		}
		return
	}
	if torrent.Info() != nil {
		torrent.CancelPieces(0, torrent.NumPieces())
	}
	torrent.DisallowDataDownload()
}
