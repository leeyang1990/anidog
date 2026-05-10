package rss

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupRSSCRUDSvc() *CRUDService {
	db := testutil.InitTestDB()
	return NewCRUDService(db)
}

func TestCreateAndGetFeed(t *testing.T) {
	svc := setupRSSCRUDSvc()
	feed := &model.RSSFeed{Name: "Mikan", URL: "https://mikan.example.com/rss", Enabled: true, Parser: "mikan"}
	if err := svc.CreateFeed(context.Background(), feed); err != nil {
		t.Fatalf("CreateFeed failed: %v", err)
	}

	got, err := svc.GetFeed(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("GetFeed failed: %v", err)
	}
	if got.Name != "Mikan" {
		t.Errorf("Name = %q; want Mikan", got.Name)
	}
}

func TestListFeeds(t *testing.T) {
	svc := setupRSSCRUDSvc()
	svc.CreateFeed(context.Background(), &model.RSSFeed{Name: "F1", URL: "https://a.com/rss", Enabled: true})
	svc.CreateFeed(context.Background(), &model.RSSFeed{Name: "F2", URL: "https://b.com/rss", Enabled: true})

	feeds, err := svc.ListFeeds(context.Background())
	if err != nil {
		t.Fatalf("ListFeeds failed: %v", err)
	}
	if len(feeds) != 2 {
		t.Errorf("len = %d; want 2", len(feeds))
	}
}

func TestUpdateFeed(t *testing.T) {
	svc := setupRSSCRUDSvc()
	feed := &model.RSSFeed{Name: "Old", URL: "https://old.com/rss", Enabled: true}
	svc.CreateFeed(context.Background(), feed)

	updated, err := svc.UpdateFeed(context.Background(), feed.ID, map[string]interface{}{"name": "New"})
	if err != nil {
		t.Fatalf("UpdateFeed failed: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("Name = %q; want New", updated.Name)
	}
}

func TestDeleteFeed(t *testing.T) {
	svc := setupRSSCRUDSvc()
	feed := &model.RSSFeed{Name: "Del", URL: "https://del.com/rss", Enabled: true}
	svc.CreateFeed(context.Background(), feed)

	if err := svc.DeleteFeed(context.Background(), feed.ID); err != nil {
		t.Fatalf("DeleteFeed failed: %v", err)
	}
	_, err := svc.GetFeed(context.Background(), feed.ID)
	if err == nil {
		t.Error("should not find deleted feed")
	}
}

func TestRuleCRUD(t *testing.T) {
	svc := setupRSSCRUDSvc()
	feed := &model.RSSFeed{Name: "Feed", URL: "https://feed.com/rss", Enabled: true}
	svc.CreateFeed(context.Background(), feed)

	rule := &model.RSSRule{Name: "Rule1", Keyword: "芙莉莲", Include: true, Enabled: true, RSSFeedID: feed.ID}
	if err := svc.CreateRule(context.Background(), rule); err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}

	rules, err := svc.ListRules(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("ListRules failed: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("len = %d; want 1", len(rules))
	}

	updated, err := svc.UpdateRule(context.Background(), rule.ID, map[string]interface{}{"keyword": "间谍"})
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}
	if updated.Keyword != "间谍" {
		t.Errorf("Keyword = %q; want 间谍", updated.Keyword)
	}

	if err := svc.DeleteRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}
}

func TestGetEntries(t *testing.T) {
	svc := setupRSSCRUDSvc()
	feed := &model.RSSFeed{Name: "Feed", URL: "https://feed.com/rss", Enabled: true}
	svc.CreateFeed(context.Background(), feed)

	entries, err := svc.GetEntries(context.Background(), feed.ID)
	if err != nil {
		t.Fatalf("GetEntries failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty feed should have 0 entries; got %d", len(entries))
	}
}
