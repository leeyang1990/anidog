package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const mikanFixtureHTML = `<!DOCTYPE html>
<html><body>
<table id="topic_list"><tbody>
<tr class="js-search-results-row" data-itemindex="0">
  <td><input type="checkbox" class="js-episode-select" /></td>
  <td>
    <a href="/Home/Episode/abc123" target="_blank" class="magnet-link-wrap">[LoliHouse] 葬送的芙莉莲 - 05 [WebRip 1080p HEVC-10bit AAC][简繁内封字幕]</a>
    <a data-clipboard-text="magnet:?xt=urn:btih:abc1234567890abc1234567890abc1234567890a&amp;tr=http%3a%2f%2ftest" class="js-magnet magnet-link">[复制磁连]</a>
  </td>
  <td>849.25 MB</td>
  <td>2026/05/02 00:24</td>
  <td><a href="/Download/20260502/abc.torrent"><img /></a></td>
  <td></td>
</tr>
</tbody></table>
</body></html>`

func TestMikanIndexer_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/Home/Search") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(mikanFixtureHTML))
	}))
	defer server.Close()

	m := &MikanIndexer{BaseURL: server.URL, Client: server.Client()}
	got, err := m.Search(context.Background(), "葬送的芙莉莲")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d", len(got))
	}
	c := got[0]
	if !strings.Contains(c.Title, "葬送的芙莉莲") {
		t.Errorf("Title: %q", c.Title)
	}
	if !strings.HasPrefix(c.MagnetURL, "magnet:?xt=urn:btih:abc") {
		t.Errorf("MagnetURL: %q", c.MagnetURL)
	}
	if c.InfoHash != "ABC1234567890ABC1234567890ABC1234567890A" {
		t.Errorf("InfoHash: %q", c.InfoHash)
	}
	if c.Size < 800*1024*1024 || c.Size > 900*1024*1024 {
		t.Errorf("Size out of range: %d", c.Size)
	}
	if c.SourceName != "mikan" {
		t.Errorf("SourceName: %q", c.SourceName)
	}
	if c.DetailURL == "" {
		t.Errorf("DetailURL empty")
	}
	if c.TorrentURL == "" || !strings.HasSuffix(c.TorrentURL, ".torrent") {
		t.Errorf("TorrentURL: %q", c.TorrentURL)
	}
}

const dmhyFixtureHTML = `<!DOCTYPE html>
<html><body>
<table id="topic_list"><tbody>
<tr class="">
  <td>2026/05/02 00:24<span style="display: none;">2026/05/02 00:24</span></td>
  <td><a class="sort-2">動畫</a></td>
  <td class="title">
    <a href="/topics/view/718129_xxx.html" target="_blank">[LoliHouse] 葬送的芙莉蓮 - 05 [1080p]</a>
  </td>
  <td>
    <a class="download-arrow arrow-magnet" title="磁力下載" href="magnet:?xt=urn:btih:KPXOBTP5NFHU357PAVEXPE4UYG2OSXHP"></a>
  </td>
  <td>809.9MB</td>
  <td><span class="btl_1">12</span></td>
  <td><span class="bts_1">3</span></td>
  <td>-</td>
  <td>user</td>
</tr>
</tbody></table>
</body></html>`

func TestDmhyIndexer_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/topics/list/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dmhyFixtureHTML))
	}))
	defer server.Close()

	d := &DmhyIndexer{BaseURL: server.URL, Client: server.Client()}
	got, err := d.Search(context.Background(), "Frieren")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d", len(got))
	}
	c := got[0]
	if !strings.Contains(c.Title, "葬送的芙莉蓮") {
		t.Errorf("Title: %q", c.Title)
	}
	if !strings.HasPrefix(c.MagnetURL, "magnet:?xt=urn:btih:KPXOBTP5") {
		t.Errorf("MagnetURL: %q", c.MagnetURL)
	}
	if c.SourceName != "dmhy" {
		t.Errorf("SourceName: %q", c.SourceName)
	}
	if c.Seeders != 12 {
		t.Errorf("Seeders: %d", c.Seeders)
	}
	if c.Size < 800*1024*1024 {
		t.Errorf("Size out of range: %d", c.Size)
	}
}

const bangumiMoeFixtureJSON = `{"torrents":[{
  "_id":"abc", "category_tag_id":"x",
  "title":"[LoliHouse] 葬送的芙莉莲 - 05 [1080p]",
  "tag_ids":[], "comments":0, "downloads":8, "finished":0, "leechers":2, "seeders":5,
  "publish_time":"2026-05-01T16:24:25.109Z",
  "magnet":"magnet:?xt=urn:btih:53eee0cdfd694f4df7ef0549779394c1b4e95cef",
  "infoHash":"53eee0cdfd694f4df7ef0549779394c1b4e95cef",
  "size":"849.25 MB"
}]}`

func TestBangumiMoeIndexer_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/torrent/search" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(bangumiMoeFixtureJSON))
	}))
	defer server.Close()

	b := &BangumiMoeIndexer{BaseURL: server.URL, Client: server.Client()}
	got, err := b.Search(context.Background(), "Frieren")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d", len(got))
	}
	c := got[0]
	if !strings.Contains(c.Title, "葬送的芙莉莲") {
		t.Errorf("Title: %q", c.Title)
	}
	if c.Seeders != 5 || c.Leechers != 2 {
		t.Errorf("Seeders/Leechers: %d/%d", c.Seeders, c.Leechers)
	}
	if c.InfoHash != "53EEE0CDFD694F4DF7EF0549779394C1B4E95CEF" {
		t.Errorf("InfoHash: %q", c.InfoHash)
	}
	if c.SourceName != "bangumimoe" {
		t.Errorf("SourceName: %q", c.SourceName)
	}
}

const nyaaFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:atom="http://www.w3.org/2005/Atom" xmlns:nyaa="https://nyaa.si/xmlns/nyaa" version="2.0">
  <channel>
    <title>Nyaa test</title>
    <item>
      <title>[SubsPlease] Frieren - 05 (1080p) [ABC12345].mkv</title>
      <link>https://nyaa.si/download/12345.torrent</link>
      <guid isPermaLink="true">https://nyaa.si/view/12345</guid>
      <pubDate>Tue, 05 May 2026 20:50:27 -0000</pubDate>
      <nyaa:seeders>135</nyaa:seeders>
      <nyaa:leechers>22</nyaa:leechers>
      <nyaa:infoHash>2fd2133f28fbd29021b775c17c8e1cf5aceb9a15</nyaa:infoHash>
      <nyaa:size>1.3 GiB</nyaa:size>
    </item>
  </channel>
</rss>`

func TestNyaaIndexer_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Write([]byte(nyaaFixtureXML))
	}))
	defer server.Close()

	n := &NyaaIndexer{BaseURL: server.URL, Client: server.Client()}
	got, err := n.Search(context.Background(), "Frieren")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 result, got %d", len(got))
	}
	c := got[0]
	if !strings.Contains(c.Title, "Frieren") {
		t.Errorf("Title: %q", c.Title)
	}
	if c.InfoHash != "2FD2133F28FBD29021B775C17C8E1CF5ACEB9A15" {
		t.Errorf("InfoHash: %q", c.InfoHash)
	}
	if !strings.HasPrefix(c.MagnetURL, "magnet:?xt=urn:btih:2fd2") {
		t.Errorf("MagnetURL should be synthesized from infoHash: %q", c.MagnetURL)
	}
	if c.Seeders != 135 {
		t.Errorf("Seeders: %d", c.Seeders)
	}
	if c.Size < 1_000_000_000 {
		t.Errorf("Size: %d", c.Size)
	}
	if c.SourceName != "nyaa" {
		t.Errorf("SourceName: %q", c.SourceName)
	}
}

func TestParseHumanSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"849.25 MB", 849 * 1024 * 1024}, // 近似
		{"1.3 GiB", 1_395_864_371},
		{"500KB", 500 * 1024},
		{"", 0},
		{"-", 0},
	}
	for _, tt := range tests {
		got := parseHumanSize(tt.in)
		// 允许 ±5% 误差（浮点）
		diff := got - tt.want
		if diff < 0 {
			diff = -diff
		}
		if tt.want > 0 && diff*20 > tt.want {
			t.Errorf("parseHumanSize(%q) = %d, want ~%d", tt.in, got, tt.want)
		}
		if tt.want == 0 && got != 0 {
			t.Errorf("parseHumanSize(%q) = %d, want 0", tt.in, got)
		}
	}
}
