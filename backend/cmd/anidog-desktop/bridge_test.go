//go:build desktop && nosqlite

package main

import (
	"path/filepath"
	"testing"

	"github.com/anidog/anidog-go/internal/config"
)

func TestDesktopBridgeInfoAndUnavailableDialog(t *testing.T) {
	root := t.TempDir()
	paths := config.DesktopPaths{
		DataRoot:  root,
		MediaRoot: filepath.Join(root, "media"),
		BTState:   filepath.Join(root, "torrent"),
	}
	bridge := NewDesktopBridge(paths)
	info := bridge.Info()
	if info.Platform == "" || info.DataRoot != paths.DataRoot || info.MediaRoot != paths.MediaRoot || info.BTState != paths.BTState {
		t.Fatalf("unexpected desktop info: %#v", info)
	}
	if _, err := bridge.SelectDirectory(paths.MediaRoot); err == nil {
		t.Fatal("directory dialog should require an initialized Wails context")
	}
}

func TestDesktopBridgeRejectsMissingPath(t *testing.T) {
	bridge := NewDesktopBridge(config.DesktopPaths{MediaRoot: t.TempDir()})
	if err := bridge.OpenPath(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing path should not be opened")
	}
}
