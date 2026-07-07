package dumpfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindNodesYAMLLegacy(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "nodes.yaml")
	writeNodeList(t, legacy)
	abs, rel, err := FindNodesYAML(root)
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
	writeNodeList(t, nodes)
	abs, rel, err := FindNodesYAML(root)
	if err != nil {
		t.Fatal(err)
	}
	if abs != nodes || rel != "cluster-scope/nodes.yaml" {
		t.Fatalf("got %q, %q; want %q, cluster-scope/nodes.yaml", abs, rel, nodes)
	}
}

func TestFindNodesYAMLSkipsCSINodes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "csinodes.yaml"), []byte(`apiVersion: v1
kind: List
items:
- kind: CSINode
  metadata:
    name: node1
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := FindNodesYAML(root); err == nil {
		t.Fatal("expected error when only csinodes.yaml present")
	}
}

func TestFindNodesYAMLPrefersClusterScope(t *testing.T) {
	root := t.TempDir()
	writeNodeList(t, filepath.Join(root, "nodes.yaml"))
	scope := filepath.Join(root, "cluster-scope")
	if err := os.MkdirAll(scope, 0o755); err != nil {
		t.Fatal(err)
	}
	preferred := filepath.Join(scope, "nodes.yaml")
	writeNodeList(t, preferred)
	abs, rel, err := FindNodesYAML(root)
	if err != nil {
		t.Fatal(err)
	}
	if abs != preferred || rel != "cluster-scope/nodes.yaml" {
		t.Fatalf("got %q, %q; want cluster-scope path", abs, rel)
	}
}

func TestFindErrorsTxt(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cluster-dump")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	errFile := filepath.Join(nested, "errors.txt")
	if err := os.WriteFile(errFile, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	abs, rel, ok := FindErrorsTxt(root)
	if !ok || abs != errFile || rel != "cluster-dump/errors.txt" {
		t.Fatalf("got %q, %q, %v", abs, rel, ok)
	}
}

func writeNodeList(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Node
  metadata:
    name: node1
  status:
    conditions: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
}
