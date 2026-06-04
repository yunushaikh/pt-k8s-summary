package collector

import (
	"strings"
	"testing"
)

func TestGatherEventsSectionHTML(t *testing.T) {
	root := "/home/yunus/projects/cursor/report_v1/cluster-dump"
	h, err := gatherEventsSectionHTML(root)
	if err != nil {
		t.Fatal(err)
	}
	if h == "" {
		t.Fatal("expected non-empty HTML")
	}
	if !strings.Contains(h, "dump-ev-filter") {
		t.Fatal("missing filter input")
	}
}
