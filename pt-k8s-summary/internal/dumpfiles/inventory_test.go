package dumpfiles

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInventory_MissingCore(t *testing.T) {
	root := t.TempDir()
	rows := BuildInventory(root, InvMissing, "", "")
	var missing int
	for _, r := range rows {
		if r.Status != InvMissing {
			t.Fatalf("key %s: want missing, got %s (%s)", r.Key, r.Status, r.RelPath)
		}
		missing++
	}
	if missing < 5 {
		t.Fatalf("expected several missing rows, got %d", missing)
	}
}

func TestBuildInventory_FoundNodesAndEvents(t *testing.T) {
	root := t.TempDir()
	nodes := []byte(`apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Node
  metadata:
    name: n1
`)
	if err := os.WriteFile(filepath.Join(root, "nodes.yaml"), nodes, 0o644); err != nil {
		t.Fatal(err)
	}
	ns := filepath.Join(root, "default")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	events := []byte(`apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Event
  metadata:
    name: e1
`)
	if err := os.WriteFile(filepath.Join(ns, "events.yaml"), events, 0o644); err != nil {
		t.Fatal(err)
	}
	rows := BuildInventory(root, InvFound, "nodes.yaml", "")
	byKey := map[string]InventoryRow{}
	for _, r := range rows {
		byKey[r.Key] = r
	}
	if byKey["nodes"].Status != InvFound || byKey["nodes"].RelPath != "nodes.yaml" {
		t.Fatalf("nodes: %+v", byKey["nodes"])
	}
	if byKey["events"].Status != InvFound {
		t.Fatalf("events: %+v", byKey["events"])
	}
	if !strings.Contains(byKey["events"].RelPath, "events.yaml") {
		t.Fatalf("events path: %q", byKey["events"].RelPath)
	}
}

func TestWarnInventoryGaps(t *testing.T) {
	rows := []InventoryRow{
		{Label: "nodes.yaml (Kubernetes Node list)", Status: InvMissing, Note: "not part of this cluster dump"},
		{Label: "pods.yaml (Pod list)", Status: InvUnreadable, Note: "permission denied"},
		{Label: "events.yaml (Kubernetes Event list)", Status: InvFound, RelPath: "ns/events.yaml"},
	}
	var buf bytes.Buffer
	WarnInventoryGaps(&buf, rows)
	out := buf.String()
	if !strings.Contains(out, "warning: dump file missing: nodes.yaml") {
		t.Fatalf("missing warning absent:\n%s", out)
	}
	if !strings.Contains(out, "warning: dump file unreadable: pods.yaml") {
		t.Fatalf("unreadable warning absent:\n%s", out)
	}
	if strings.Contains(out, "events.yaml") {
		t.Fatalf("should not warn for found files:\n%s", out)
	}
}
