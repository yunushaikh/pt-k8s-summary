package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeCRVersionForURL(t *testing.T) {
	if got := sanitizeCRVersionForURL("1.19.0"); got != "1.19.0" {
		t.Fatalf("got %q want 1.19.0", got)
	}
	if got := sanitizeCRVersionForURL("v1.2.3-rc"); got != "1.2.3" {
		t.Fatalf("got %q want 1.2.3", got)
	}
}

func TestNormalizeOCIImageRef(t *testing.T) {
	cases := []struct{ in, want string }{
		{"docker.io/percona/haproxy:2.8.17", "percona/haproxy:2.8.17"},
		{"Percona/HAProxy:2.8.17", "percona/haproxy:2.8.17"},
		{"percona/haproxy:2.8.17@sha256:abc", "percona/haproxy:2.8.17"},
	}
	for _, c := range cases {
		if got := normalizeOCIImageRef(c.in); got != c.want {
			t.Errorf("normalizeOCIImageRef(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

func TestFlattenPodLogJSONLines(t *testing.T) {
	in := "{\"log\":\"hello world\\n\",\"file\":\"/var/lib/mysql/mysqld-error.log\"}\nplain line\n"
	out := flattenPodLogJSONLines(in)
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected decoded log in output: %q", out)
	}
	if strings.Contains(out, `{"log":`) {
		t.Fatalf("should strip JSON wrapper: %q", out)
	}
	if !strings.Contains(out, "plain line") {
		t.Fatalf("expected plain line preserved: %q", out)
	}
}

func TestBackupNameMatchesPodMetadata(t *testing.T) {
	if !backupNameMatchesPodMetadata(
		map[string]string{"percona.com/backup-name": "cron-cluster1-fs-pvc-1"},
		nil,
		"cron-cluster1-fs-pvc-1",
	) {
		t.Fatal("expected label match")
	}
	if !backupNameMatchesPodMetadata(
		nil,
		map[string]string{"percona.com/backup-name": "b2"},
		"b2",
	) {
		t.Fatal("expected annotation match")
	}
	if backupNameMatchesPodMetadata(map[string]string{"percona.com/backup-name": "x"}, nil, "y") {
		t.Fatal("expected no match")
	}
}

// TestReportSmokeFixture runs loaders against the parent cluster-dump directory when present
// (the layout used by `go run . -dump ..` from this module).
func TestReportSmokeFixture(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dumpRoot := filepath.Clean(filepath.Join(wd, ".."))
	pxcPath := filepath.Join(dumpRoot, "default", pxcFileName)
	if _, err := os.Stat(pxcPath); err != nil {
		t.Skip("fixture dump not found at", pxcPath, "— skipping smoke test")
	}

	now := time.Now().UTC()
	pods, err := loadPodLoader(dumpRoot)
	if err != nil {
		t.Fatalf("loadPodLoader: %v", err)
	}

	pxcRows, nFiles, err := loadPXCRowsFromDump(dumpRoot, now, pods, newCertifiedImageCache(false))
	if err != nil {
		t.Fatalf("loadPXCRowsFromDump: %v", err)
	}
	if len(pxcRows) == 0 {
		t.Fatalf("expected at least one PXC row from fixture, got 0 (files=%d)", nFiles)
	}

	backups, bFiles, err := loadBackupRowsFromDump(dumpRoot, now, pods)
	if err != nil {
		t.Fatalf("loadBackupRowsFromDump: %v", err)
	}
	if len(backups) == 0 {
		t.Fatalf("expected at least one backup row from fixture, got 0 (files=%d)", bFiles)
	}

	// At least one backup should resolve to a pod with logs in this repo’s sample dump.
	var withLog int
	for _, b := range backups {
		if b.HasPodLog {
			withLog++
		}
	}
	if withLog == 0 {
		t.Fatalf("expected at least one backup row with HasPodLog in fixture dump")
	}
	for _, b := range backups {
		if b.BackupManifestModalID == "" || b.BackupManifestEscaped == "" {
			t.Fatalf("backup %q: want non-empty manifest modal id and escaped YAML", b.Name)
		}
		if !strings.Contains(b.BackupManifestEscaped, "apiVersion:") || !strings.Contains(b.BackupManifestEscaped, "kind:") {
			t.Fatalf("backup %q: manifest should look like Kubernetes YAML", b.Name)
		}
	}
}
