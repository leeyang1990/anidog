package mediacheck

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A real SOCKS handshake fixture records the hostname to prove DNS stays on
// the selected proxy route, then forwards to a loopback-only media server.
func socksFixture(t *testing.T) (string, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var proxied atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(10 * time.Second))
				header := make([]byte, 2)
				if _, err := io.ReadFull(conn, header); err != nil {
					return
				}
				methods := make([]byte, int(header[1]))
				if _, err := io.ReadFull(conn, methods); err != nil {
					return
				}
				conn.Write([]byte{5, 2}) // Require username/password authentication.
				if _, err := io.ReadFull(conn, header); err != nil || header[0] != 1 {
					return
				}
				user := make([]byte, int(header[1]))
				if _, err := io.ReadFull(conn, user); err != nil {
					return
				}
				if _, err := io.ReadFull(conn, header[:1]); err != nil {
					return
				}
				password := make([]byte, int(header[0]))
				if _, err := io.ReadFull(conn, password); err != nil {
					return
				}
				if string(user) != "fixture-user" || string(password) != "fixture-pass" {
					return
				}
				conn.Write([]byte{1, 0})
				req := make([]byte, 4)
				if _, err := io.ReadFull(conn, req); err != nil || req[1] != 1 {
					return
				}
				var host string
				switch req[3] {
				case 1:
					buf := make([]byte, 4)
					if _, err := io.ReadFull(conn, buf); err != nil {
						return
					}
					host = net.IP(buf).String()
				case 3:
					if _, err := io.ReadFull(conn, header[:1]); err != nil {
						return
					}
					buf := make([]byte, int(header[0]))
					if _, err := io.ReadFull(conn, buf); err != nil {
						return
					}
					host = string(buf)
				default:
					return
				}
				port := make([]byte, 2)
				if _, err := io.ReadFull(conn, port); err != nil {
					return
				}
				if host == "media.invalid" {
					proxied.Add(1)
					host = "127.0.0.1"
				} else if host == "127.0.0.1" {
					proxied.Add(1)
				} else {
					return
				}
				upstream, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))), time.Second)
				if err != nil {
					return
				}
				defer upstream.Close()
				conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 0})
				go func() { io.Copy(upstream, conn); upstream.Close() }()
				io.Copy(conn, upstream)
			}()
		}
	}()
	return "socks5h://fixture-user:fixture-pass@" + ln.Addr().String(), &proxied
}

func TestSOCKSBridgeProbeAndHTTPS(t *testing.T) {
	p := transportVideo(t, "mp4")
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, p) }))
	defer origin.Close()
	proxy, connections := socksFixture(t)
	ctx, closeBridge, err := PrepareTransport(context.Background(), proxy)
	if err != nil {
		t.Fatal(err)
	}
	defer closeBridge()
	remoteDNS := strings.Replace(origin.URL, "127.0.0.1", "media.invalid", 1) + "/video.mp4"
	if _, err := ProbeInput(ctx, remoteDNS, nil); err != nil {
		t.Fatal(err)
	}
	if connections.Load() == 0 {
		t.Fatal("SOCKS route did not receive the media request")
	}
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "tunneled") }))
	defer secure.Close()
	bridgeURL, _ := url.Parse(ctx.Value(mediaProxyKey{}).(string))
	transport := &http.Transport{Proxy: http.ProxyURL(bridgeURL), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	response, err := client.Get(secure.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || string(data) != "tunneled" {
		t.Fatalf("CONNECT: %q %v", data, err)
	}
	closeBridge()
	closeBridge()
	if conn, err := net.DialTimeout("tcp", bridgeURL.Host, time.Second); err == nil {
		conn.Close()
		t.Fatal("closed task left a proxy listener")
	}
}

func TestTransportRejectsUnsupportedRoutesWithoutSecrets(t *testing.T) {
	for _, raw := range []string{"https://user:secret@proxy.invalid", "socks4://user:secret@proxy.invalid", ":invalid"} {
		_, close, err := PrepareTransport(context.Background(), raw)
		close()
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("route validation: %v", err)
		}
	}
	cmd := MediaCommand(WithHTTPProxy(context.Background(), "socks5://user:secret@proxy.invalid:1080"), "ffprobe", "anything")
	if err := cmd.Run(); err == nil || strings.Contains(err.Error(), "secret") || !IsToolError(err) {
		t.Fatalf("unprepared route must fail closed: %v", err)
	}
}

func TestSOCKSBridgeClosesOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	route, closeBridge, err := PrepareTransport(ctx, "socks5://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer closeBridge()
	address, _ := url.Parse(route.Value(mediaProxyKey{}).(string))
	cancel()
	deadline := time.After(time.Second)
	for {
		conn, err := net.DialTimeout("tcp", address.Host, 50*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		select {
		case <-deadline:
			t.Fatal("cancelled task retained its proxy listener")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
