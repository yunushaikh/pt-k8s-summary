package jpreport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSTSIndexAndCheckComponent(t *testing.T) {
	root := t.TempDir()
	ns := filepath.Join(root, "everest")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `apiVersion: v1
kind: List
items:
- apiVersion: apps/v1
  kind: StatefulSet
  metadata:
    name: mysql-test-haproxy
    namespace: everest
  spec:
    replicas: 3
  status:
    replicas: 3
    readyReplicas: 2
- apiVersion: apps/v1
  kind: StatefulSet
  metadata:
    name: mysql-test-pxc
    namespace: everest
  spec:
    replicas: 1
  status:
    replicas: 1
    readyReplicas: 1
`
	if err := os.WriteFile(filepath.Join(ns, "statefulsets.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	idx, err := loadSTSIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	match := idx.checkComponent("everest", "mysql-test", "pxc", 1)
	if match.Mismatch || match.STSMissing || match.CompareNote != "match" {
		t.Fatalf("pxc match: %+v", match)
	}
	if match.STSReplicas != "1" || match.STSReady != "1" {
		t.Fatalf("pxc replicas: %+v", match)
	}
	bad := idx.checkComponent("everest", "mysql-test", "haproxy", 2)
	if !bad.Mismatch || bad.CompareNote == "match" {
		t.Fatalf("expected haproxy mismatch: %+v", bad)
	}
	if bad.STSReplicas != "3" || bad.STSReady != "2" {
		t.Fatalf("haproxy replicas: %+v", bad)
	}
	missing := idx.checkComponent("everest", "mysql-test", "proxysql", 1)
	if !missing.STSMissing || missing.CompareNote != "STS not in dump" {
		t.Fatalf("expected missing STS: %+v", missing)
	}
}
