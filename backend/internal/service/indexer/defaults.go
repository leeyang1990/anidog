package indexer

import "net/http"

// NewDefaultIndexers keeps manual search and automatic orchestration on the
// same clients, including the application's hot-updated proxy transport.
func NewDefaultIndexers(client *http.Client) map[string]Indexer {
	mikan, dmhy := NewMikanIndexer(), NewDmhyIndexer()
	moe, nyaa := NewBangumiMoeIndexer(), NewNyaaIndexer()
	if client != nil {
		mikan.Client, dmhy.Client, moe.Client, nyaa.Client = client, client, client, client
	}
	return map[string]Indexer{"mikan": mikan, "dmhy": dmhy, "bangumimoe": moe, "nyaa": nyaa}
}
