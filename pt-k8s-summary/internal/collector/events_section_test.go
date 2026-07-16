package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatherEventsSectionHTML_coreV1AndEventsV1(t *testing.T) {
	root := t.TempDir()
	ns := filepath.Join(root, "ns1")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	coreV1 := `apiVersion: v1
kind: EventList
items:
- apiVersion: v1
  kind: Event
  type: Warning
  reason: Failed
  message: core v1 message
  count: 3
  lastTimestamp: "2026-07-16T10:00:00Z"
  involvedObject:
    kind: Pod
    name: core-pod
    namespace: ns1
  metadata:
    namespace: ns1
    creationTimestamp: "2026-07-16T09:00:00Z"
`
	eventsV1 := `apiVersion: events.k8s.io/v1
kind: EventList
items:
- apiVersion: events.k8s.io/v1
  kind: Event
  type: Normal
  reason: Pulling
  note: events.k8s.io note
  deprecatedCount: 8
  deprecatedLastTimestamp: "2026-07-16T11:00:00Z"
  regarding:
    kind: Pod
    name: everest-pod
    namespace: ns1
  metadata:
    namespace: ns1
    creationTimestamp: "2026-07-16T09:30:00Z"
`
	if err := os.WriteFile(filepath.Join(ns, "events.yaml"), []byte(coreV1), 0o644); err != nil {
		t.Fatal(err)
	}
	ns2 := filepath.Join(root, "ns2")
	if err := os.MkdirAll(ns2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ns2, "events.yaml"), []byte(eventsV1), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := gatherEventsSectionHTML(root)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Fatal("expected non-empty HTML")
	}
	for _, want := range []string{
		"dump-ev-filter",
		"core v1 message",
		"events.k8s.io note",
		"Pod/core-pod",
		"Pod/everest-pod",
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("missing %q in events HTML", want)
		}
	}
}
