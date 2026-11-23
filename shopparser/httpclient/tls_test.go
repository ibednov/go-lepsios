package httpclient

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Guard: parser tree must never set InsecureSkipVerify.
func TestNoInsecureSkipVerifyInParserTree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..") // repo root from this package
	// Prefer walking from module: find go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			root = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
	parserRoot := root
	err = filepath.Walk(parserRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "InsecureSkipVerify") && !strings.Contains(path, "_test.go") {
			// allow comments that forbid it
			for _, line := range strings.Split(string(b), "\n") {
				trim := strings.TrimSpace(line)
				if strings.Contains(trim, "InsecureSkipVerify") && !strings.HasPrefix(trim, "//") && !strings.HasPrefix(trim, "*") {
					t.Errorf("%s sets InsecureSkipVerify", path)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
