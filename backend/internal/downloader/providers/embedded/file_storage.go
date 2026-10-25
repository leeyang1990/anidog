package embedded

import (
	"context"
	"os"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// The file backend's optional WriterTo uses SEEK_DATA to skip sparse holes.
// Some network filesystems return EIO for that operation even when ordinary
// reads succeed. Expose the required ReaderAt contract so torrent's normal
// piece hashing reads all bytes, including holes, without that optimization.
// Do not retry WriterTo into the same hash: it may have written a prefix.
type portableFileStorage struct {
	storage.ClientImplCloser
}

// The library selects its file IO during package initialization. Capture the
// same setting once: classic IO is the supported backend for SMB deployments.
// mmap's ReaderAt cannot safely read beyond a retained, truncated file, so keep
// its existing WriterTo path for native local-storage clients.
var classicFileIO = os.Getenv("TORRENT_STORAGE_DEFAULT_FILE_IO") == "classic"

func newFileStorage(root string) storage.ClientImplCloser {
	client := storage.NewFile(root)
	if classicFileIO {
		return portableFileStorage{client}
	}
	return client
}

func (s portableFileStorage) OpenTorrent(ctx context.Context, info *metainfo.Info, hash metainfo.Hash) (storage.TorrentImpl, error) {
	opened, err := s.ClientImplCloser.OpenTorrent(ctx, info, hash)
	if err != nil {
		return opened, err
	}
	if piece := opened.Piece; piece != nil {
		opened.Piece = func(p metainfo.Piece) storage.PieceImpl {
			return portablePiece{piece(p)}
		}
	}
	if piece := opened.PieceWithHash; piece != nil {
		opened.PieceWithHash = func(p metainfo.Piece, hash g.Option[[]byte]) storage.PieceImpl {
			return portablePiece{piece(p, hash)}
		}
	}
	return opened, nil
}

// Embedding the mandatory interface deliberately excludes optional fast readers
// and self-hashers. Writes, completion bookkeeping, and real read errors retain
// the underlying file backend's behavior.
type portablePiece struct {
	storage.PieceImpl
}
