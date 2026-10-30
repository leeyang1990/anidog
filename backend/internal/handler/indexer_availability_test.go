package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anidog/anidog-go/internal/service/indexer"
	"github.com/gin-gonic/gin"
)

type availabilityIndexer struct{ failed bool }

func (i availabilityIndexer) Name() string { return "fixture" }
func (i availabilityIndexer) Search(context.Context, string) ([]indexer.Candidate, error) {
	if i.failed {
		return nil, errors.New("private request context must not appear in response")
	}
	return nil, nil
}
func TestIndexerSearchExposesAvailabilityWithoutPrivateError(t *testing.T) {
	for _, failed := range []bool{false, true} {
		h := &IndexerHandler{indexers: map[string]indexer.Indexer{"fixture": availabilityIndexer{failed}}}
		rr := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rr)
		c.Request = httptest.NewRequest(http.MethodPost, "/indexer/search", strings.NewReader(`{"keyword":"Example","indexers":["fixture"]}`))
		c.Request.Header.Set("Content-Type", "application/json")
		h.Search(c)
		if rr.Code != http.StatusOK {
			t.Fatal(rr.Code, rr.Body.String())
		}
		var got struct {
			Status string `json:"search_status"`
			Detail string `json:"detail"`
			Total  int    `json:"total"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		want := "complete"
		if failed {
			want = "unavailable"
		}
		if got.Status != want || got.Total != 0 || (got.Detail != "") != failed || strings.Contains(rr.Body.String(), "private request") {
			t.Fatal(rr.Body.String())
		}
	}
}
