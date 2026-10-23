package orchestrator

import (
	"context"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/service/mediapolicy"
	"reflect"
	"testing"

	"github.com/anidog/anidog-go/internal/service/setting"
	"github.com/anidog/anidog-go/internal/testutil"
)

func TestDefaultPriorityUsesBTThenMikanThenStream(t *testing.T) {
	got := Defaults().Priority
	if len(got) != 3 || got[0] != SourceBT || got[1] != SourceMikan || got[2] != SourceStream {
		t.Fatalf("default priority = %v, want [bt mikan stream]", got)
	}
}

func TestDiscoveryAndAcceptanceResolveIdenticalMediaGoals(t *testing.T) {
	for _, override := range []string{"", "[]", `["traditional"]`, "broken", "null"} {
		db := testutil.InitTestDB()
		svc := setting.NewService(nil, db)
		if err := svc.Set(context.Background(), "download.languages", `["simplified"]`); err != nil {
			t.Fatal(err)
		}
		a := model.Anime{Title: "goal", OverrideLanguages: &override}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
		global, err := LoadGlobal(context.Background(), svc)
		if err != nil {
			t.Fatal(err)
		}
		discovery, discoveryErr := MergeWithAnime(global, &a)
		acceptance, acceptanceErr := mediapolicy.Load(context.Background(), db, &a.ID)
		if (discoveryErr != nil) != (acceptanceErr != nil) {
			t.Fatalf("%q: %v vs %v", override, discoveryErr, acceptanceErr)
		}
		if discoveryErr == nil && (discovery.Quality != acceptance.Quality || !reflect.DeepEqual(discovery.Languages, acceptance.Languages) || discovery.AllowPreferenceFallback != acceptance.AllowFallback) {
			t.Fatalf("%q: search and acceptance disagree", override)
		}
	}
}

func TestGlobalPolicyReadFailureDoesNotReturnDefaults(t *testing.T) {
	db := testutil.InitTestDB()
	svc := setting.NewService(nil, db)
	conn, _ := db.DB()
	conn.Close()
	if _, err := LoadGlobal(context.Background(), svc); err == nil {
		t.Fatal("DB outage became default settings")
	}
}

func TestLoadGlobalUpgradesLegacyBTStreamPriority(t *testing.T) {
	db := testutil.InitTestDB()
	svc := setting.NewService(nil, db)
	if err := svc.Set(context.Background(), "download.priority", `["bt","stream"]`); err != nil {
		t.Fatal(err)
	}

	pref, err := LoadGlobal(context.Background(), svc)
	if err != nil {
		t.Fatal(err)
	}
	got := pref.Priority
	if len(got) != 3 || got[0] != SourceBT || got[1] != SourceMikan || got[2] != SourceStream {
		t.Fatalf("upgraded priority = %v, want [bt mikan stream]", got)
	}
}
