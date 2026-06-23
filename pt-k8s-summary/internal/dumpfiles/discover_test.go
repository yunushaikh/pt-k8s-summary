package dumpfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindListYAMLFiles_PSPreferred(t *testing.T) {
	root := t.TempDir()
	ns := filepath.Join(root, "default")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ns, "perconaservermysqls.ps.percona.com.yaml")
	if err := os.WriteFile(path, []byte(`apiVersion: v1
kind: List
items:
- apiVersion: ps.percona.com/v1
  kind: PerconaServerMySQL
  metadata:
    name: ps-cluster1
    namespace: default
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindListYAMLFiles(root, PSClusterList)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != path {
		t.Fatalf("got %v want [%s]", got, path)
	}
}

func TestFindListYAMLFiles_PSKindFallback(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "custom-perconaservermysqls-export.yaml")
	if err := os.WriteFile(path, []byte(`apiVersion: v1
kind: List
items:
- kind: PerconaServerMySQL
  metadata:
    name: x
`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindListYAMLFiles(root, PSClusterList)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected fallback match, got %v", got)
	}
}
