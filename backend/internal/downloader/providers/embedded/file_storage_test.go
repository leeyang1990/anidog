package embedded

import (
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"io"
	"syscall"
	"testing"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type sparseSeekFailurePiece struct {
	storage.PieceImpl
	data      []byte
	readError error
	fastCalls int
}

func (p *sparseSeekFailurePiece) ReadAt(b []byte, off int64) (int, error) {
	if p.readError != nil {
		return 0, p.readError
	}
	return bytes.NewReader(p.data).ReadAt(b, off)
}

func (p *sparseSeekFailurePiece) WriteTo(w io.Writer) (int64, error) {
	p.fastCalls++
	n, err := w.Write([]byte("prefix before failed sparse seek"))
	if err != nil {
		return int64(n), err
	}
	return int64(n), syscall.EIO
}

type sparseSeekFailureClient struct {
	piece    *sparseSeekFailurePiece
	withHash bool
}

func (c sparseSeekFailureClient) Close() error { return nil }
func (c sparseSeekFailureClient) OpenTorrent(context.Context, *metainfo.Info, metainfo.Hash) (storage.TorrentImpl, error) {
	if c.withHash {
		return storage.TorrentImpl{PieceWithHash: func(metainfo.Piece, g.Option[[]byte]) storage.PieceImpl { return c.piece }}, nil
	}
	return storage.TorrentImpl{Piece: func(metainfo.Piece) storage.PieceImpl { return c.piece }}, nil
}

func TestPortableStorageHashesExactBytesWhenSparseFastPathFails(t *testing.T) {
	for _, withHash := range []bool{false, true} {
		name := "v1-piece"
		if withHash {
			name = "piece-with-hash"
		}
		t.Run(name, func(t *testing.T) {
			data := append(bytes.Repeat([]byte("actual payload"), 64), make([]byte, 256)...)
			want := sha1.Sum(data)
			info := &metainfo.Info{Name: "payload", PieceLength: int64(len(data)), Length: int64(len(data)), Pieces: want[:]}
			fixture := &sparseSeekFailurePiece{data: data}
			client := sparseSeekFailureClient{piece: fixture, withHash: withHash}
			legacy, err := storage.NewClient(client).OpenTorrent(context.Background(), info, metainfo.Hash{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := legacy.Piece(info.Piece(0)).WriteTo(io.Discard); !errors.Is(err, syscall.EIO) {
				t.Fatalf("fixture must reproduce sparse seek failure: %v", err)
			}
			fixture.fastCalls = 0
			portable, err := storage.NewClient(portableFileStorage{client}).OpenTorrent(context.Background(), info, metainfo.Hash{})
			if err != nil {
				t.Fatal(err)
			}
			hash := sha1.New()
			n, err := portable.Piece(info.Piece(0)).WriteTo(hash)
			if err != nil || n != int64(len(data)) || !bytes.Equal(hash.Sum(nil), want[:]) {
				t.Fatalf("piece hash lost bytes or included the failed fast-path prefix: n=%d err=%v hash=%x", n, err, hash.Sum(nil))
			}
			if fixture.fastCalls != 0 {
				t.Fatal("sparse fast path was invoked")
			}
		})
	}
}

func TestPortableStoragePropagatesRealReadFailure(t *testing.T) {
	data := []byte("payload")
	want := sha1.Sum(data)
	info := &metainfo.Info{Name: "payload", PieceLength: int64(len(data)), Length: int64(len(data)), Pieces: want[:]}
	client := portableFileStorage{sparseSeekFailureClient{piece: &sparseSeekFailurePiece{data: data, readError: syscall.EIO}}}
	opened, err := storage.NewClient(client).OpenTorrent(context.Background(), info, metainfo.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Piece(info.Piece(0)).WriteTo(io.Discard); !errors.Is(err, syscall.EIO) {
		t.Fatalf("real data read errors must prevent acceptance: %v", err)
	}
}
