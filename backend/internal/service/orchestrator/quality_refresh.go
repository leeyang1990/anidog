package orchestrator

import (
	"context"
	"github.com/anidog/anidog-go/internal/model"
)

// RefreshLibraryQuality updates projections only. A completion event must not
// become an alternate entry point for creating more downloads or upgrades.
func (o *Orchestrator) RefreshLibraryQuality(ctx context.Context, animeID uint) error {
	lock := o.animeLock(animeID)
	lock.Lock()
	defer lock.Unlock()
	var anime model.Anime
	if err := o.db.WithContext(ctx).First(&anime, animeID).Error; err != nil {
		return err
	}
	global, err := LoadGlobal(ctx, o.settingSvc)
	if err != nil {
		return err
	}
	pref, err := MergeWithAnime(global, &anime)
	if err != nil {
		return err
	}
	if err := o.checkMediaRootHealth(ctx, o.currentMediaRoot(ctx)); err != nil {
		return err
	}
	quality, err := o.auditLibraryQuality(ctx, &anime, pref)
	if err != nil {
		return err
	}
	expected := 0
	if anime.EpisodeCount != nil {
		expected = *anime.EpisodeCount
	}
	o.persistQualityState(ctx, &anime, quality, expected)
	return nil
}
