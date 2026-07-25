package orchestrator

import (
	"context"
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

func TestLoadGlobalUpgradesLegacyBTStreamPriority(t *testing.T) {
	db := testutil.InitTestDB()
	svc := setting.NewService(nil, db)
	if err := svc.Set(context.Background(), "download.priority", `["bt","stream"]`); err != nil {
		t.Fatal(err)
	}

	got := LoadGlobal(context.Background(), svc).Priority
	if len(got) != 3 || got[0] != SourceBT || got[1] != SourceMikan || got[2] != SourceStream {
		t.Fatalf("upgraded priority = %v, want [bt mikan stream]", got)
	}
}
