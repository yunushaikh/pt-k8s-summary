package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindNodesYAML(t *testing.T) {
	root := t.TempDir()
	if _, _, err := findNodesYAML(root); err == nil {
		t.Fatal("expected error when nodes.yaml missing")
	}

	legacy := filepath.Join(root, "nodes.yaml")
	if err := os.WriteFile(legacy, []byte("items: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, rel, err := findNodesYAML(root)
	if err != nil {
		t.Fatal(err)
	}
	if abs != legacy || rel != "nodes.yaml" {
		t.Fatalf("got %q, %q; want %q, nodes.yaml", abs, rel, legacy)
	}
}

func TestFindNodesYAMLClusterScope(t *testing.T) {
	root := t.TempDir()
	scopeDir := filepath.Join(root, "cluster-scope")
	if err := os.MkdirAll(scopeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	nodes := filepath.Join(scopeDir, "nodes.yaml")
	if err := os.WriteFile(nodes, []byte("items: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, rel, err := findNodesYAML(root)
	if err != nil {
		t.Fatal(err)
	}
	if abs != nodes || rel != "cluster-scope/nodes.yaml" {
		t.Fatalf("got %q, %q; want %q, cluster-scope/nodes.yaml", abs, rel, nodes)
	}
}

func TestPullKnownFlagsDoubleDash(t *testing.T) {
	got := pullKnownFlags([]string{"dump.tar.gz", "--layout", "grouped"})
	want := []string{"-layout", "grouped", "dump.tar.gz"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v; want %v", got, want)
		}
	}
}
