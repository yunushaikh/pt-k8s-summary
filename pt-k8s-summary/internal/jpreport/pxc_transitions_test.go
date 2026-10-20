package jpreport

import (
	"strings"
	"testing"
)

func TestPXCPreviousTransitionsOrderAndDedup(t *testing.T) {
	conds := []pxcCRCondition{
		{
			LastTransitionTime: "2026-09-24T08:36:27Z",
			Message:            "smart update: failed to apply changes: failed to wait pod: pod prod1-pxc-1 is in failed phase",
			Reason:             "ErrorReconcile",
			Status:             "True",
			Type:               "error",
		},
		{
			LastTransitionTime: "2026-09-24T08:00:00Z",
			Status:             "True",
			Type:               "ready",
		},
		{
			LastTransitionTime: "2026-09-24T08:24:45Z",
			Message:            "smart update: failed to apply changes: failed to wait pod: pod prod1-pxc-2\n        is in failed phase",
			Reason:             "ErrorReconcile",
			Status:             "True",
			Type:               "error",
		},
		{
			LastTransitionTime: "2026-09-24T08:24:46Z",
			Status:             "True",
			Type:               "initializing",
		},
		// exact duplicate of the first error
		{
			LastTransitionTime: "2026-09-24T08:36:27Z",
			Message:            "smart update: failed to apply changes: failed to wait pod: pod prod1-pxc-1 is in failed phase",
			Reason:             "ErrorReconcile",
			Status:             "True",
			Type:               "error",
		},
	}
	got := pxcPreviousTransitions(conds)
	if len(got) != 3 {
		t.Fatalf("len=%d, want 3 (ready omitted, duplicate dropped): %#v", len(got), got)
	}
	wantTypes := []string{"error", "initializing", "error"}
	wantPods := []string{"prod1-pxc-2", "", "prod1-pxc-1"}
	for i, w := range wantTypes {
		if got[i].Type != w {
			t.Fatalf("[%d] type %q want %q", i, got[i].Type, w)
		}
		if wantPods[i] != "" && !strings.Contains(got[i].Message, wantPods[i]) {
			t.Fatalf("[%d] message %q missing %s", i, got[i].Message, wantPods[i])
		}
		if !got[i].IsError && w == "error" {
			t.Fatalf("[%d] expected error highlight", i)
		}
	}
	if strings.Contains(got[0].Message, "\n") {
		t.Fatalf("message not flattened: %q", got[0].Message)
	}
}
