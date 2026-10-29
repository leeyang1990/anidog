package embedded

import (
	"fmt"
	"strings"
	"time"

	atorrent "github.com/anacrolix/torrent"
)

// contentIdentity separates a persistent provider receipt from the protocol's
// 20-byte handshake key. A hybrid uses v1 by default; a pure v2 uses all 32
// bytes. Restored tasks retain their original receipt when metadata adds an
// alias, so controls, database snapshots and the manifest never change keys.
func contentIdentity(spec *atorrent.TorrentSpec, retained string) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("BT 元数据缺少内容身份")
	}
	v1, v2 := "", ""
	if !spec.InfoHash.IsZero() {
		v1 = strings.ToUpper(spec.InfoHash.HexString())
	}
	if spec.InfoHashV2.Ok {
		value := spec.InfoHashV2.Value
		if value.ToShort().IsZero() {
			return "", fmt.Errorf("BT v2 握手身份不能为零")
		}
		v2 = strings.ToUpper(value.HexString())
	}
	if retained = strings.ToUpper(strings.TrimSpace(retained)); retained != "" {
		if retained == v1 || retained == v2 {
			return retained, nil
		}
		return "", fmt.Errorf("BT 恢复身份与元数据不一致")
	}
	if v1 != "" {
		return v1, nil
	}
	if v2 != "" {
		return v2, nil
	}
	return "", fmt.Errorf("BT 元数据缺少有效的 v1 或 v2 内容身份")
}

type addRequest struct {
	URL       string
	SourceURL string
	SavePath  string
	Identity  string
	Paused    bool
	AddedAt   time.Time
	Persist   bool
}
