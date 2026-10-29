// Package downloadrecord owns the persistence references that keep an attempt
// necessary after its transfer has ended. Scheduling and user deletion share
// this boundary; neither may infer that superseded means unreferenced.
package downloadrecord

import (
	"errors"

	"gorm.io/gorm"
)

var ErrReferenced = errors.New("下载记录仍被字幕证据、媒体映射或升级备份引用，已保留")

// Unreferenced is applied to the download table in a single DELETE or lookup.
// Foreign keys additionally protect evidence edges against concurrent inserts.
// Filesystem backup roots and legacy string episode identities are also roots.
func Unreferenced(db *gorm.DB) *gorm.DB {
	return EvidenceUnreferenced(db).Where("coalesce(download.backup_path, '') = ''").
		Where("NOT EXISTS (SELECT 1 FROM animeepisode ep WHERE ep.download_id = download.torrent_id OR (coalesce(download.info_hash, '') <> '' AND ep.download_id = download.info_hash))")
}

// Explicit media deletion may remove a library asset, but cannot remove the
// provenance of other durable facts. Check before touching downloader files.
func EvidenceUnreferenced(db *gorm.DB) *gorm.DB {
	return db.Where("NOT EXISTS (SELECT 1 FROM media_evidence e WHERE e.origin_download_id = download.id)").
		Where("NOT EXISTS (SELECT 1 FROM download reused WHERE reused.evidence_reused_from = download.id)")
}
