package embedded

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	g "github.com/anacrolix/generics"
	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// A storage adapter may keep the supplied content identity after the factory
// returns. Metadata publication must not mutate that retained hash buffer.
type hashViewStorage struct{}

func (hashViewStorage) OpenTorrent(context.Context, *metainfo.Info, metainfo.Hash) (storage.TorrentImpl, error) {
	return storage.TorrentImpl{PieceWithHash: func(_ metainfo.Piece, hash g.Option[[]byte]) storage.PieceImpl {
		return hashViewPiece{hash: hash.UnwrapOr(nil)}
	}}, nil
}

type hashViewPiece struct{ hash []byte }

func (p hashViewPiece) ReadAt(b []byte, off int64) (int, error) {
	if off >= int64(len(p.hash)) {
		return 0, io.EOF
	}
	n := copy(b, p.hash[off:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}
func (hashViewPiece) WriteAt([]byte, int64) (int, error) {
	return 0, fmt.Errorf("identity fixture never transfers payload")
}
func (hashViewPiece) MarkComplete() error            { return nil }
func (hashViewPiece) MarkNotComplete() error         { return nil }
func (hashViewPiece) Completion() storage.Completion { return storage.Completion{Ok: true} }

func TestV2StorageIdentityIsStableDuringMetadataUpdates(t *testing.T) {
	mi, _, _ := versionedFixture(t, t.TempDir(), false)
	cfg := atorrent.NewDefaultClientConfig()
	cfg.DataDir = t.TempDir()
	cfg.ListenPort = 0
	cfg.ListenHost = func(network string) string {
		if strings.HasSuffix(network, "6") {
			return "::1"
		}
		return "127.0.0.1"
	}
	cfg.NoDHT = true
	cfg.DisableTrackers = true
	cfg.NoDefaultPortForwarding = true
	cfg.DisablePEX = true
	cfg.DisableWebtorrent = true
	cfg.DisableWebseeds = true
	cfg.DefaultStorage = hashViewStorage{}
	client, err := atorrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	spec, err := atorrent.TorrentSpecFromMetaInfoErr(mi)
	if err != nil {
		t.Fatal(err)
	}
	spec.DisableInitialPieceCheck = true
	spec.DisallowDataDownload = true
	tor, _, err := client.AddTorrentSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	piece := tor.Piece(0)
	retained := piece.Storage()
	var expected []byte
	for _, layer := range mi.PieceLayers {
		expected = append([]byte(nil), layer[:32]...)
	}
	start := make(chan struct{})
	failures := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for range 2000 {
			if err := tor.MergeSpec(&atorrent.TorrentSpec{PieceLayers: mi.PieceLayers}); err != nil {
				failures <- err
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for range 2000 {
			for _, view := range []storage.Piece{retained, piece.Storage()} {
				var actual [32]byte
				n, err := view.ReadAt(actual[:], 0)
				if err != nil || n != 32 || !bytes.Equal(actual[:], expected) {
					failures <- fmt.Errorf("storage identity changed: %x (%d, %v)", actual, n, err)
					return
				}
			}
		}
	}()
	close(start)
	workers.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}
