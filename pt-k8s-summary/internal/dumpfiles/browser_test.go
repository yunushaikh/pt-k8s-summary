package dumpfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectBrowserFiles(t *testing.T) {
	root := t.TempDir()
	yaml := []byte(`apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Node
  metadata:
    name: n1
`)
	if err := os.WriteFile(filepath.Join(root, "nodes.yaml"), yaml, 0o644); err != nil {
		t.Fatal(err)
	}
	files := CollectBrowserFiles(root)
	if len(files) < 1 {
		t.Fatal("expected at least nodes.yaml")
	}
	found := false
	for _, f := range files {
		if f.RelPath == "nodes.yaml" && !f.Omitted && strings.Contains(f.ContentEscaped, "kind: Node") {
			found = true
		}
	}
	if !found {
		t.Fatalf("nodes.yaml not embedded: %+v", files)
	}
	groups := BrowserTreeGroups(files)
	if len(groups) < 1 {
		t.Fatal("expected tree groups")
	}
}
