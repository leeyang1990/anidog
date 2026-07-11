package anime

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupAnimeSvc() *Service {
	db := testutil.InitTestDB()
	return New(db)
}

func TestCreateAndGet(t *testing.T) {
	svc := setupAnimeSvc()
	anime := &model.Anime{Title: "Frieren", Status: model.AnimeStatusOngoing}
	if err := svc.Create(context.Background(), anime); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if anime.ID == 0 {
		t.Fatal("ID should be set after create")
	}

	got, err := svc.Get(context.Background(), anime.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "Frieren" {
		t.Errorf("Title = %q; want Frieren", got.Title)
	}
}

func TestList(t *testing.T) {
	svc := setupAnimeSvc()
	for _, title := range []string{"A", "B", "C"} {
		svc.Create(context.Background(), &model.Anime{Title: title, Status: model.AnimeStatusUnknown})
	}

	animes, total, err := svc.List(context.Background(), "", false, 1, 2)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d; want 3", total)
	}
	if len(animes) != 2 {
		t.Errorf("len = %d; want 2", len(animes))
	}
}

func TestList_FilterByStatus(t *testing.T) {
	svc := setupAnimeSvc()
	svc.Create(context.Background(), &model.Anime{Title: "Ongoing", Status: model.AnimeStatusOngoing})
	svc.Create(context.Background(), &model.Anime{Title: "Finished", Status: model.AnimeStatusFinished})

	animes, total, _ := svc.List(context.Background(), model.AnimeStatusOngoing, false, 1, 10)
	if total != 1 {
		t.Errorf("total = %d; want 1", total)
	}
	if len(animes) != 1 || animes[0].Title != "Ongoing" {
		t.Errorf("wrong results: %+v", animes)
	}
}

func TestUpdate(t *testing.T) {
	svc := setupAnimeSvc()
	anime := &model.Anime{Title: "Old", Status: model.AnimeStatusUnknown}
	svc.Create(context.Background(), anime)

	updated, err := svc.Update(context.Background(), anime.ID, map[string]interface{}{"title": "New"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "New" {
		t.Errorf("Title = %q; want New", updated.Title)
	}
}

func TestDelete(t *testing.T) {
	svc := setupAnimeSvc()
	anime := &model.Anime{Title: "ToDelete", Status: model.AnimeStatusUnknown}
	svc.Create(context.Background(), anime)

	if err := svc.Delete(context.Background(), anime.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := svc.Get(context.Background(), anime.ID)
	if err == nil {
		t.Error("should not find deleted anime")
	}
}

func TestSubscribeUnsubscribe(t *testing.T) {
	svc := setupAnimeSvc()
	anime := &model.Anime{Title: "Sub", Status: model.AnimeStatusUnknown, IsSubscribed: false}
	svc.Create(context.Background(), anime)

	sub, err := svc.Subscribe(context.Background(), anime.ID)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if !sub.IsSubscribed {
		t.Error("should be subscribed")
	}

	unsub, err := svc.Unsubscribe(context.Background(), anime.ID)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
	if unsub.IsSubscribed {
		t.Error("should be unsubscribed")
	}
}

func TestFindByBangumiID(t *testing.T) {
	svc := setupAnimeSvc()
	bid := 12345
	svc.Create(context.Background(), &model.Anime{Title: "BG", Status: model.AnimeStatusUnknown, BangumiID: &bid})

	found, err := svc.FindByBangumiID(context.Background(), 12345)
	if err != nil {
		t.Fatalf("FindByBangumiID failed: %v", err)
	}
	if found.Title != "BG" {
		t.Errorf("Title = %q; want BG", found.Title)
	}

	_, err = svc.FindByBangumiID(context.Background(), 99999)
	if err == nil {
		t.Error("should not find non-existent bangumi ID")
	}
}

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Error("nil is not NotFound")
	}
}

func TestGetSubscriptionMap(t *testing.T) {
	svc := setupAnimeSvc()
	bid1 := 100
	svc.Create(context.Background(), &model.Anime{Title: "A", Status: model.AnimeStatusUnknown, BangumiID: &bid1, IsSubscribed: true})

	m := svc.GetSubscriptionMap(context.Background())
	if len(m) != 1 {
		t.Fatalf("map len = %d; want 1", len(m))
	}
	if !m[100].IsSubscribed {
		t.Error("anime 100 should be subscribed")
	}
}

func TestCreateFromBangumi(t *testing.T) {
	svc := setupAnimeSvc()
	detail := &model.BangumiAnime{
		Name:     "葬送的芙莉莲",
		NameCN:   "Frieren",
		ImageURL: "https://img.example.com/frieren.jpg",
		Rating:   9.1,
		EpsCount: 28,
	}
	anime, err := svc.CreateFromBangumi(context.Background(), 40001, detail)
	if err != nil {
		t.Fatalf("CreateFromBangumi failed: %v", err)
	}
	if anime.Title != "Frieren" {
		t.Errorf("Title = %q; want Frieren", anime.Title)
	}
	if !anime.IsSubscribed {
		t.Error("should be subscribed")
	}
	if anime.BangumiID == nil || *anime.BangumiID != 40001 {
		t.Error("BangumiID mismatch")
	}
}

func TestCreateFromBangumi_NilDetail(t *testing.T) {
	svc := setupAnimeSvc()
	anime, err := svc.CreateFromBangumi(context.Background(), 40002, nil)
	if err != nil {
		t.Fatalf("CreateFromBangumi with nil failed: %v", err)
	}
	if anime.Title != "Bangumi:40002" {
		t.Errorf("Title = %q; want Bangumi:40002", anime.Title)
	}
}

func TestEpisodeCRUD(t *testing.T) {
	svc := setupAnimeSvc()
	anime := &model.Anime{Title: "Eps", Status: model.AnimeStatusUnknown}
	svc.Create(context.Background(), anime)

	ep := &model.AnimeEpisode{EpisodeNumber: 1, Title: ptrStr("EP1")}
	if err := svc.CreateEpisode(context.Background(), anime.ID, ep); err != nil {
		t.Fatalf("CreateEpisode failed: %v", err)
	}

	eps, err := svc.ListEpisodes(context.Background(), anime.ID)
	if err != nil {
		t.Fatalf("ListEpisodes failed: %v", err)
	}
	if len(eps) != 1 || eps[0].EpisodeNumber != 1 {
		t.Errorf("episodes = %+v", eps)
	}

	if err := svc.DeleteEpisode(context.Background(), anime.ID, ep.ID); err != nil {
		t.Fatalf("DeleteEpisode failed: %v", err)
	}
}

func ptrStr(s string) *string { return &s }
