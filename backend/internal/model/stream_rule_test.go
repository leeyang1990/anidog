package model

import "testing"

func TestToInt_Float64(t *testing.T) {
	got, ok := toInt(float64(6))
	if !ok || got != 6 {
		t.Errorf("toInt(float64(6)) = %d, %v; want 6, true", got, ok)
	}
}

func TestToInt_Int(t *testing.T) {
	got, ok := toInt(7)
	if !ok || got != 7 {
		t.Errorf("toInt(7) = %d, %v; want 7, true", got, ok)
	}
}

func TestToInt_StringValid(t *testing.T) {
	got, ok := toInt("8")
	if !ok || got != 8 {
		t.Errorf("toInt(\"8\") = %d, %v; want 8, true", got, ok)
	}
}

func TestToInt_StringInvalid(t *testing.T) {
	_, ok := toInt("abc")
	if ok {
		t.Error("toInt(\"abc\") should return false")
	}
}

func TestToInt_StringEmpty(t *testing.T) {
	_, ok := toInt("")
	if ok {
		t.Error("toInt(\"\") should return false")
	}
}

func TestToInt_UnknownType(t *testing.T) {
	_, ok := toInt([]int{1})
	if ok {
		t.Error("toInt([]int{1}) should return false")
	}
}

func TestMapKazumiRule_BasicFields(t *testing.T) {
	data := map[string]interface{}{
		"name":          "test-rule",
		"version":       "2.0",
		"api":           float64(6),
		"baseURL":       "https://example.com",
		"searchURL":     "https://example.com/search/@keyword",
		"usePost":       true,
		"searchList":    "//div[@class='list']",
		"searchName":    "//h3",
		"searchResult":  "//a",
		"chapterResult": "//div[@class='ep']",
	}
	mapped := MapKazumiRule(data)

	if mapped["name"] != "test-rule" {
		t.Errorf("name = %v; want test-rule", mapped["name"])
	}
	if mapped["api_level"] != 6 {
		t.Errorf("api_level = %v; want 6", mapped["api_level"])
	}
	if mapped["base_url"] != "https://example.com" {
		t.Errorf("base_url = %v", mapped["base_url"])
	}
	if mapped["search_list_xpath"] != "//div[@class='list']" {
		t.Errorf("search_list_xpath = %v", mapped["search_list_xpath"])
	}
}

func TestMapKazumiRule_IgnoredFields(t *testing.T) {
	data := map[string]interface{}{
		"type":             "ignore-me",
		"useNativePlayer":  true,
		"useLegacyParser":  true,
		"adBlocker":        true,
	}
	mapped := MapKazumiRule(data)

	if _, exists := mapped["type"]; exists {
		t.Error("type should be ignored")
	}
	if _, exists := mapped["use_native_player"]; exists {
		t.Error("useNativePlayer should be ignored")
	}
	if _, exists := mapped["use_legacy_parser"]; exists {
		t.Error("useLegacyParser should be ignored")
	}
}

func TestMapKazumiRule_AntiCrawlerConfig(t *testing.T) {
	data := map[string]interface{}{
		"antiCrawlerConfig": map[string]interface{}{
			"cookie": "test=1",
		},
	}
	mapped := MapKazumiRule(data)

	val, ok := mapped["anti_crawler_config"].(string)
	if !ok {
		t.Fatal("anti_crawler_config should be a JSON string")
	}
	if val == "" {
		t.Error("anti_crawler_config should not be empty")
	}
}

func TestMapKazumiRule_ApiLevelString(t *testing.T) {
	data := map[string]interface{}{
		"api": "7",
	}
	mapped := MapKazumiRule(data)
	if mapped["api_level"] != 7 {
		t.Errorf("api_level = %v; want 7", mapped["api_level"])
	}
}

func TestMapKazumiRule_ApiLevelInvalid(t *testing.T) {
	data := map[string]interface{}{
		"api": "not-a-number",
	}
	mapped := MapKazumiRule(data)
	if mapped["api_level"] != 6 {
		t.Errorf("api_level = %v; want 6 (default)", mapped["api_level"])
	}
}

func TestBangumiAnimeWithStatus(t *testing.T) {
	a := BangumiAnimeWithStatus{
		BangumiAnime: BangumiAnime{
			ID:     1,
			Name:   "test",
			NameCN: "测试",
			Rating: 8.5,
		},
		IsSubscribed: true,
		LocalID:      10,
	}
	if a.ID != 1 || a.Name != "test" || !a.IsSubscribed || a.LocalID != 10 {
		t.Errorf("BangumiAnimeWithStatus embedding failed: %+v", a)
	}
}

func TestSubStatus(t *testing.T) {
	s := SubStatus{IsSubscribed: true, LocalID: 5}
	if !s.IsSubscribed || s.LocalID != 5 {
		t.Errorf("SubStatus failed: %+v", s)
	}
}
