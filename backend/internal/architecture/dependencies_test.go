package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const internal = "github.com/anidog/anidog-go/internal/"

type dependencyRule struct {
	directory string
	forbidden []string
}

// These are ownership boundaries, not a snapshot of today's import graph.
// Database access in application services remains allowed; execution adapters
// and measurement libraries must not acquire scheduling or HTTP API authority.
var boundaries = []dependencyRule{
	{"mediaquality", []string{internal, "gorm.io", "github.com/gin-gonic/gin"}},
	{"service/titleparse", []string{internal, "gorm.io", "github.com/gin-gonic/gin"}},
	{"service/mediacheck", []string{internal, "gorm.io", "github.com/gin-gonic/gin"}},
	{"downloader", []string{internal + "service", internal + "handler", internal + "middleware", internal + "model", internal + "ws", "gorm.io", "github.com/gin-gonic/gin"}},
	{"service/mediapolicy", []string{internal + "service", internal + "downloader", internal + "handler", "github.com/gin-gonic/gin"}},
	{"service/failure", []string{internal + "service", internal + "downloader", internal + "handler", "gorm.io", "github.com/gin-gonic/gin"}},
	{"service/download", []string{internal + "service/indexer", internal + "service/orchestrator", internal + "service/rss", internal + "service/bangumi", internal + "handler", "github.com/gin-gonic/gin"}},
}

func TestDownloadDependencyBoundaries(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate architecture checks")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	for _, rule := range boundaries {
		t.Run(rule.directory, func(t *testing.T) {
			violations, err := dependencyViolations(filepath.Join(root, rule.directory), rule.forbidden)
			if err != nil {
				t.Fatal(err)
			}
			for _, violation := range violations {
				t.Error(violation)
			}
		})
	}
}

func dependencyViolations(root string, forbidden []string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Parse all platform files, independent of the current build tags. A
		// Windows-only import must not bypass a Linux architecture check.
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range file.Imports {
			dependency, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return err
			}
			for _, prefix := range forbidden {
				prefix = strings.TrimSuffix(prefix, "/")
				if dependency == prefix || strings.HasPrefix(dependency, prefix+"/") {
					violations = append(violations, fmt.Sprintf("%s imports %s across its ownership boundary", path, dependency))
				}
			}
		}
		return nil
	})
	return violations, err
}

func TestDependencyChecksIncludeNestedAndOtherPlatformFiles(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"valid.go":           "package fixture\nimport \"strings\"\n// import \"blocked/library\"\n",
		"valid_prefix.go":    "package fixture\nimport \"blocked/libraryish\"\n",
		"adapter_windows.go": "//go:build windows\n\npackage fixture\nimport transport \"blocked/library\"\n",
		"nested/child.go":    "package fixture\nimport _ \"blocked/library/subpackage\"\n",
		"allowed_test.go":    "package fixture\nimport \"blocked/library\"\n",
	}
	for name, source := range fixtures {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	violations, err := dependencyViolations(root, []string{"blocked/library"})
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 || !strings.Contains(violations[0], "adapter_windows.go") || !strings.Contains(violations[1], "child.go") {
		t.Fatalf("wrong dependency violations: %v", violations)
	}
}

func TestEmbeddedDownloadControlHasSingleOwner(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate architecture checks")
	}
	root := filepath.Join(filepath.Dir(file), "..", "downloader", "providers", "embedded")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		violations, err := downloadControlViolations(path, nil)
		if err != nil {
			return err
		}
		for _, name := range violations {
			t.Errorf("%s: %s bypasses setDownloadDemand; piece demand and permission must change together", path, name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func downloadControlViolations(path string, source any) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, source, 0)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Recv == nil && fn.Name.Name == "setDownloadDemand" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok {
				switch selector.Sel.Name {
				case "AllowDataDownload", "DisallowDataDownload", "DownloadAll", "DownloadPieces", "CancelPieces":
					violations = append(violations, fn.Name.Name+"."+selector.Sel.Name)
				}
			}
			return true
		})
	}
	return violations, nil
}

func TestDownloadControlCheckRejectsScatteredPermissionChanges(t *testing.T) {
	source := `package fixture
func setDownloadDemand() { torrent.DisallowDataDownload() }
func pause() { torrent.DisallowDataDownload() }
func resume() { torrent.AllowDataDownload(); torrent.DownloadAll() }
`
	violations, err := downloadControlViolations("fixture.go", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 3 {
		t.Fatalf("scattered controls were not rejected: %v", violations)
	}
}
