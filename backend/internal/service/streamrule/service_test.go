package streamrule

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupStreamRuleSvc() *Service {
	db := testutil.InitTestDB()
	return NewService(db, nil) // no searcher for CRUD tests
}

func TestCreateAndGet(t *testing.T) {
	svc := setupStreamRuleSvc()
	rule := &model.StreamRule{
		Name:              "test-rule",
		BaseURL:           "https://example.com",
		SearchURL:         "https://example.com/search",
		SearchListXPath:   "//div",
		SearchNameXPath:   "//h3",
		SearchResultXPath: "//a",
		ChapterResultXPath: "//div",
		Enabled:           true,
	}
	if err := svc.Create(context.Background(), rule); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := svc.Get(context.Background(), rule.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "test-rule" {
		t.Errorf("Name = %q; want test-rule", got.Name)
	}
}

func TestList(t *testing.T) {
	svc := setupStreamRuleSvc()
	for i := 0; i < 3; i++ {
		svc.Create(context.Background(), &model.StreamRule{
			Name:              "rule",
			BaseURL:           "https://example.com",
			SearchURL:         "https://example.com/search",
			SearchListXPath:   "//div",
			SearchNameXPath:   "//h3",
			SearchResultXPath: "//a",
			ChapterResultXPath: "//div",
			Enabled:           true,
		})
	}

	rules, err := svc.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(rules) != 3 {
		t.Errorf("len = %d; want 3", len(rules))
	}
}

func TestList_FilterEnabled(t *testing.T) {
	svc := setupStreamRuleSvc()
	svc.Create(context.Background(), &model.StreamRule{
		Name: "enabled1", BaseURL: "https://a.com", SearchURL: "/s",
		SearchListXPath: "//div", SearchNameXPath: "//h3", SearchResultXPath: "//a",
		ChapterResultXPath: "//div", Enabled: true,
	})
	svc.Create(context.Background(), &model.StreamRule{
		Name: "disabled1", BaseURL: "https://b.com", SearchURL: "/s",
		SearchListXPath: "//div", SearchNameXPath: "//h3", SearchResultXPath: "//a",
		ChapterResultXPath: "//div", Enabled: false,
	})

	enabled := true
	rules, _ := svc.List(context.Background(), &enabled)
	for _, r := range rules {
		if !r.Enabled {
			t.Errorf("got disabled rule: %s", r.Name)
		}
	}
}

func TestUpdate(t *testing.T) {
	svc := setupStreamRuleSvc()
	rule := &model.StreamRule{
		Name: "old", BaseURL: "https://a.com", SearchURL: "/s",
		SearchListXPath: "//div", SearchNameXPath: "//h3", SearchResultXPath: "//a",
		ChapterResultXPath: "//div", Enabled: true,
	}
	svc.Create(context.Background(), rule)

	updated, err := svc.Update(context.Background(), rule.ID, map[string]interface{}{"name": "new"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("Name = %q; want new", updated.Name)
	}
}

func TestDelete(t *testing.T) {
	svc := setupStreamRuleSvc()
	rule := &model.StreamRule{
		Name: "del", BaseURL: "https://a.com", SearchURL: "/s",
		SearchListXPath: "//div", SearchNameXPath: "//h3", SearchResultXPath: "//a",
		ChapterResultXPath: "//div", Enabled: true,
	}
	svc.Create(context.Background(), rule)

	if err := svc.Delete(context.Background(), rule.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := svc.Get(context.Background(), rule.ID)
	if err == nil {
		t.Error("should not find deleted rule")
	}
}

func TestImportKazumiRules(t *testing.T) {
	svc := setupStreamRuleSvc()
	rawRules := []map[string]interface{}{
		{
			"name":           "kazumi-rule",
			"baseURL":        "https://kazumi.example.com",
			"searchURL":      "https://kazumi.example.com/search",
			"searchList":     "//div[@class='list']",
			"searchName":     "//h3",
			"searchResult":   "//a",
			"chapterResult":  "//div[@class='ep']",
		},
	}

	result := svc.ImportKazumiRules(context.Background(), rawRules)
	if result.Imported != 1 {
		t.Errorf("Imported = %d; want 1", result.Imported)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d; want 0", result.Failed)
	}
}

func TestExport(t *testing.T) {
	svc := setupStreamRuleSvc()
	svc.Create(context.Background(), &model.StreamRule{
		Name: "export-rule", BaseURL: "https://a.com", SearchURL: "/s",
		SearchListXPath: "//div", SearchNameXPath: "//h3", SearchResultXPath: "//a",
		ChapterResultXPath: "//div", Enabled: true,
	})

	rules, err := svc.Export(context.Background())
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("len = %d; want 1", len(rules))
	}
}

func TestTestRule_NoSearcher(t *testing.T) {
	svc := setupStreamRuleSvc()
	_, err := svc.TestRule(context.Background(), 1, "test")
	if err == nil {
		t.Error("TestRule without searcher should fail")
	}
}
