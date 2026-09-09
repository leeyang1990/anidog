package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigureDesktopUsesLocalPersistentPaths(t *testing.T) {
	t.Setenv("ANIDOG_DESKTOP_BT_LISTEN_PORT", "")
	root := t.TempDir()
	dataRoot := filepath.Join(root, "data")
	mediaRoot := filepath.Join(root, "media")
	cfg := &Config{DownloaderType: "qbittorrent", MediaRoot: "/downloads"}

	paths, err := ConfigureDesktop(cfg, dataRoot, mediaRoot)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DownloaderType != "embedded" || cfg.BTListenPort != 0 {
		t.Fatalf("unexpected desktop engine config: type=%q port=%d", cfg.DownloaderType, cfg.BTListenPort)
	}
	if cfg.DatabaseURL != "sqlite://"+paths.Database {
		t.Fatalf("unexpected database url: %q", cfg.DatabaseURL)
	}
	if cfg.MediaRoot != mediaRoot || cfg.StreamDownloadDir != mediaRoot {
		t.Fatalf("desktop media roots are not aligned: %#v", cfg)
	}
	if _, err := os.Stat(paths.BTState); err != nil {
		t.Fatalf("bt state dir was not created: %v", err)
	}
	firstSecret := cfg.SecretKey
	if len(firstSecret) < 32 {
		t.Fatalf("desktop secret is too short: %q", firstSecret)
	}

	second := &Config{}
	if _, err := ConfigureDesktop(second, dataRoot, mediaRoot); err != nil {
		t.Fatal(err)
	}
	if second.SecretKey != firstSecret {
		t.Fatal("desktop secret was not persisted")
	}
	info, err := os.Stat(filepath.Join(dataRoot, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("desktop secret permissions are too broad: %o", info.Mode().Perm())
	}
}

func TestConfigureDesktopRejectsInvalidPort(t *testing.T) {
	t.Setenv("ANIDOG_DESKTOP_BT_LISTEN_PORT", "70000")
	_, err := ConfigureDesktop(&Config{}, filepath.Join(t.TempDir(), "data"), filepath.Join(t.TempDir(), "media"))
	if err == nil {
		t.Fatal("expected invalid desktop bt port to fail")
	}
}
