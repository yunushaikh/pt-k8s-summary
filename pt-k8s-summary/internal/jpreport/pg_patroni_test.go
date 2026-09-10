package jpreport

import (
	"strings"
	"testing"
	"time"
)

func TestFillPGPatroniFromSpec(t *testing.T) {
	sync := 10
	lease := 30
	port := 8008
	cr := &pgClusterYAML{}
	cr.Metadata.Name = "postgresql-vgl"
	cr.Metadata.Namespace = "everest"
	cr.Spec.Patroni = &pgPatroniSpec{
		SyncPeriodSeconds:          &sync,
		LeaderLeaseDurationSeconds: &lease,
		Port:                       &port,
	}
	cr.Spec.Patroni.DynamicConfiguration.PostgreSQL.Parameters = map[string]interface{}{
		"shared_buffers":         "1GB",
		"track_commit_timestamp": "on",
		"max_worker_processes":   float64(2),
		"work_mem":               "2MB",
	}
	row := buildPGRowTmpl(cr, time.Now())
	if !row.HasPatroni {
		t.Fatal("expected HasPatroni")
	}
	if row.PatroniSyncPeriod != "10" || row.PatroniLeaderLease != "30" || row.PatroniPort != "8008" {
		t.Fatalf("timing: sync=%s lease=%s port=%s", row.PatroniSyncPeriod, row.PatroniLeaderLease, row.PatroniPort)
	}
	if row.PatroniParamsFullEscaped == "" {
		t.Fatal("expected full escaped params")
	}
	if !strings.Contains(row.PatroniParamsFullEscaped, "max_worker_processes = 2") {
		t.Fatalf("full %q", row.PatroniParamsFullEscaped)
	}
	if !row.PatroniParamsTruncated {
		t.Fatal("expected truncated with 4 params")
	}
	if !strings.Contains(row.PatroniParamsSnippet, "+1 more") {
		t.Fatalf("snippet %q", row.PatroniParamsSnippet)
	}
	if row.PatroniParamsModalID == "" {
		t.Fatal("expected modal id")
	}
}

func TestFillPGPatroniAbsent(t *testing.T) {
	cr := &pgClusterYAML{}
	cr.Metadata.Name = "x"
	row := buildPGRowTmpl(cr, time.Now())
	if row.HasPatroni {
		t.Fatal("expected no patroni")
	}
}

func TestFillPGPatroniFewParamsNotTruncated(t *testing.T) {
	cr := &pgClusterYAML{}
	cr.Metadata.Name = "pg"
	cr.Metadata.Namespace = "ns"
	cr.Spec.Patroni = &pgPatroniSpec{}
	cr.Spec.Patroni.DynamicConfiguration.PostgreSQL.Parameters = map[string]interface{}{
		"shared_buffers": "128M",
		"work_mem":       "2MB",
	}
	row := buildPGRowTmpl(cr, time.Now())
	if row.PatroniParamsTruncated {
		t.Fatalf("unexpected truncate: %q", row.PatroniParamsSnippet)
	}
	if row.PatroniParamsFullEscaped == "" || row.PatroniParamsSnippet == "—" {
		t.Fatal("expected params content")
	}
}
