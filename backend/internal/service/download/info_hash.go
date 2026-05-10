package download

import (
	"encoding/base32"
	"encoding/hex"
	"regexp"
	"strings"
)

// magnet:?xt=urn:btih:<40 hex> 或 <32 base32>
var reMagnetHash = regexp.MustCompile(`(?i)btih:([a-zA-Z0-9]{32,40})`)

// Mikan / dmhy 等 .torrent URL 中的 hash：/xxx/<40 hex>.torrent
var reURLPathHash = regexp.MustCompile(`(?i)/([a-f0-9]{40})\.torrent`)

// ExtractInfoHash 从 magnet 或 .torrent URL 提取 info hash，统一返回大写 40 字符 hex。
// 支持 magnet 的 base32（32 字符）形式，会自动转 hex。无法提取返回空字符串。
func ExtractInfoHash(url string) string {
	if url == "" {
		return ""
	}
	if m := reMagnetHash.FindStringSubmatch(url); m != nil {
		raw := strings.ToUpper(m[1])
		if len(raw) == 40 {
			return raw
		}
		if len(raw) == 32 {
			// base32 32 字符 = 20 字节 → 40 hex 字符
			decoded, err := base32.StdEncoding.DecodeString(raw)
			if err == nil && len(decoded) == 20 {
				return strings.ToUpper(hex.EncodeToString(decoded))
			}
		}
	}
	if m := reURLPathHash.FindStringSubmatch(url); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}
