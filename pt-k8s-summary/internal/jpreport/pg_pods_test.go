package jpreport

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePatroniRoleFromLog(t *testing.T) {
	pod := "postgresql-wfo-instance1-pmhh-0"
	content := `
2026-09-10 04:56:00,000 INFO: no action. I am (postgresql-wfo-instance1-pmhh-0), a secondary, and following a leader (old-leader)
2026-09-10 04:57:00,000 INFO: no action. I am (postgresql-wfo-instance1-pmhh-0), the leader with the lock
`
	role, follow := parsePatroniRoleFromLog(content, pod)
	if role != "Leader" || follow != "" {
		t.Fatalf("got role=%q follow=%q", role, follow)
	}

	sec := "postgresql-wfo-instance1-l5gd-0"
	content2 := `2026-09-10 04:57:06,261 INFO: no action. I am (postgresql-wfo-instance1-l5gd-0), a secondary, and following a leader (postgresql-wfo-instance1-pmhh-0)`
	role, follow = parsePatroniRoleFromLog(content2, sec)
	if role != "Secondary" || follow != "postgresql-wfo-instance1-pmhh-0" {
		t.Fatalf("got role=%q follow=%q", role, follow)
	}
}

func TestParsePatroniRoleIgnoresOtherPods(t *testing.T) {
	content := `INFO: no action. I am (other-pod-0), the leader with the lock`
	role, _ := parsePatroniRoleFromLog(content, "my-pod-0")
	if role != "" {
		t.Fatalf("expected empty, got %q", role)
	}
}

func TestPgRoleFromPodLabel(t *testing.T) {
	p := &podItem{}
	p.Metadata.Labels = map[string]string{"postgres-operator.crunchydata.com/role": "primary"}
	if got := pgRoleFromPodLabel(p); got != "Leader" {
		t.Fatalf("got %q", got)
	}
	p.Metadata.Labels["postgres-operator.crunchydata.com/role"] = "replica"
	if got := pgRoleFromPodLabel(p); got != "Secondary" {
		t.Fatalf("got %q", got)
	}
	p.Metadata.Labels["postgres-operator.crunchydata.com/role"] = "pgbouncer"
	if got := pgRoleFromPodLabel(p); got != "—" {
		t.Fatalf("pgbouncer should not have instance role, got %q", got)
	}
}

func TestPgRoleFromPatroniLogsReadsDump(t *testing.T) {
	root := t.TempDir()
	ns, pod := "everest", "postgresql-wfo-instance1-pmhh-0"
	dir := filepath.Join(root, ns, pod)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "2026-09-10 04:57:09,264 INFO: no action. I am (postgresql-wfo-instance1-pmhh-0), the leader with the lock\n"
	if err := os.WriteFile(filepath.Join(dir, "logs.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	role, follow := pgRoleFromPatroniLogs(root, ns, pod)
	if role != "Leader" || follow != "" {
		t.Fatalf("got role=%q follow=%q", role, follow)
	}
}
