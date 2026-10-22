package mediacheck

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

// RequestHeaders keeps the browser's actual media request, including an absent
// Referer. Range and conditional/cache headers describe one browser read, not
// the complete transfer. Never copy hop-by-hop or proxy authentication headers.
func RequestHeaders(headers http.Header) http.Header {
	clean := make(http.Header)
	for _, name := range []string{"User-Agent", "Referer", "Origin", "Cookie", "Accept", "Accept-Language"} {
		value := headers.Get(name)
		if value != "" && !strings.ContainsAny(value, "\r\n\x00") {
			clean.Set(name, value)
		}
	}
	return clean
}

func headerInputArgs(headers http.Header) []string {
	headers = RequestHeaders(headers)
	args := InputArgs(headers.Get("Referer"), headers.Get("User-Agent"))
	keys := make([]string, 0, len(headers))
	for name := range headers {
		if name != "Referer" && name != "User-Agent" {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	var extra strings.Builder
	for _, name := range keys {
		extra.WriteString(name + ": " + headers.Get(name) + "\r\n")
	}
	if extra.Len() > 0 {
		args = append(args, "-headers", extra.String())
	}
	return args
}

func MediaRequestArgs(tool, input, kind string, headers http.Header) []string {
	headers = RequestHeaders(headers)
	cookies := (&http.Request{Header: headers}).Cookies()
	headers.Del("Cookie")
	args := headerInputArgs(headers)
	// FFmpeg's cookie jar matches the original media host. Raw Cookie headers
	// would otherwise leak into cross-host HLS segments and redirects.
	if u, err := url.Parse(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		var scoped strings.Builder
		for _, cookie := range cookies {
			cookie.Domain, cookie.Path = "", ""
			// FFmpeg compares its Host header, including an explicit port.
			scoped.WriteString(cookie.String() + "; path=/; domain=" + u.Host + ";\n")
		}
		if scoped.Len() > 0 {
			args = append(args, "-cookies", scoped.String())
		}
	}
	return hlsInputArgs(tool, input, kind, args)
}
