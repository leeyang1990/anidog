package notification

import (
	"context"
	"testing"

	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupNotifSvc() *Service {
	db := testutil.InitTestDB()
	return NewService(db)
}

func TestCreateAndGet(t *testing.T) {
	svc := setupNotifSvc()
	ch := &model.NotificationChannel{
		Type:   "telegram",
		Name:   "Test TG",
		Config: `{"bot_token":"123","chat_id":"456"}`,
		Enabled: true,
	}
	if err := svc.Create(context.Background(), ch); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ch.ID == 0 {
		t.Fatal("ID should be set")
	}

	got, err := svc.Get(context.Background(), ch.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "Test TG" {
		t.Errorf("Name = %q; want Test TG", got.Name)
	}
}

func TestList(t *testing.T) {
	svc := setupNotifSvc()
	for _, name := range []string{"TG", "Bark"} {
		svc.Create(context.Background(), &model.NotificationChannel{
			Type: "telegram", Name: name, Config: "{}", Enabled: true,
		})
	}

	channels, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(channels) != 2 {
		t.Errorf("len = %d; want 2", len(channels))
	}
}

func TestUpdate(t *testing.T) {
	svc := setupNotifSvc()
	ch := &model.NotificationChannel{Type: "telegram", Name: "Old", Config: "{}", Enabled: true}
	svc.Create(context.Background(), ch)

	updated, err := svc.Update(context.Background(), ch.ID, map[string]interface{}{"name": "New"})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("Name = %q; want New", updated.Name)
	}
}

func TestDelete(t *testing.T) {
	svc := setupNotifSvc()
	ch := &model.NotificationChannel{Type: "bark", Name: "Del", Config: "{}", Enabled: true}
	svc.Create(context.Background(), ch)

	if err := svc.Delete(context.Background(), ch.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := svc.Get(context.Background(), ch.ID)
	if err == nil {
		t.Error("should not find deleted channel")
	}
}

func TestTest_InvalidChannel(t *testing.T) {
	svc := setupNotifSvc()
	err := svc.Test(context.Background(), 9999)
	if err == nil {
		t.Error("testing non-existent channel should fail")
	}
}
