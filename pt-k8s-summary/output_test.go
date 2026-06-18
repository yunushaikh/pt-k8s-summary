package main

import (
	"strings"
	"testing"
)

func TestReportFileURL(t *testing.T) {
	got := reportFileURL("/tmp/report files/summary.html")
	if !strings.Contains(got, "file://") {
		t.Fatalf("expected file URL, got %q", got)
	}
	if !strings.Contains(got, "report%20files") {
		t.Fatalf("expected encoded space in URL, got %q", got)
	}
}

func TestTerminalHyperlink(t *testing.T) {
	got := terminalHyperlink("file:///tmp/a.html", "/tmp/a.html")
	if !strings.Contains(got, "file:///tmp/a.html") {
		t.Fatalf("missing URL in hyperlink: %q", got)
	}
	if !strings.Contains(got, "/tmp/a.html") {
		t.Fatalf("missing label in hyperlink: %q", got)
	}
}
