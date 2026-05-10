package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"golang.org/x/crypto/bcrypt"

	animesvc "github.com/anidog/anidog-go/internal/service/anime"
	authsvc "github.com/anidog/anidog-go/internal/service/auth"
	"github.com/anidog/anidog-go/internal/model"
	"github.com/anidog/anidog-go/internal/testutil"
)

func setupAnimeHandler() (*AnimeHandler, *animesvc.Service, string) {
	db := testutil.InitTestDB()
	authSvc := authsvc.New(db, "test-secret", 60*1e9)
	animeSvc := animesvc.New(db)

	hash, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	db.Exec("INSERT INTO user (username, password_hash, is_admin, is_active) VALUES (?, ?, ?, ?)", "admin", string(hash), true, true)
	token, _, _ := authSvc.CreateTokenPair("admin")

	return NewAnimeHandler(animeSvc, nil), animeSvc, token
}

func TestAnimeList(t *testing.T) {
	h, _, token := setupAnimeHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodGet, "/api/v1/anime", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestAnimeCreateAndGet(t *testing.T) {
	h, _, token := setupAnimeHandler()
	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	body := map[string]interface{}{
		"title":  "Frieren",
		"status": "ongoing",
	}
	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/anime/", body, token)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body = %s", w.Code, w.Body.String())
	}

	var created model.Anime
	json.NewDecoder(w.Body).Decode(&created)

	w = testutil.MakeRequest(router, http.MethodGet, "/api/v1/anime/"+uintToStr(created.ID), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("get: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestAnimeDelete(t *testing.T) {
	h, animeSvc, token := setupAnimeHandler()
	anime := &model.Anime{Title: "ToDelete", Status: model.AnimeStatusUnknown}
	animeSvc.Create(context.Background(), anime)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodDelete, "/api/v1/anime/"+uintToStr(anime.ID), nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func TestAnimeSubscribe(t *testing.T) {
	h, animeSvc, token := setupAnimeHandler()
	anime := &model.Anime{Title: "Sub", Status: model.AnimeStatusUnknown}
	animeSvc.Create(context.Background(), anime)

	router := testutil.SetupRouter()
	v1 := router.Group("/api/v1")
	h.RegisterRoutes(v1)

	w := testutil.MakeRequest(router, http.MethodPost, "/api/v1/anime/"+uintToStr(anime.ID)+"/subscribe", nil, token)
	if w.Code != http.StatusOK {
		t.Fatalf("subscribe: status = %d; body = %s", w.Code, w.Body.String())
	}
}

func uintToStr(u uint) string {
	return fmt.Sprintf("%d", u)
}
