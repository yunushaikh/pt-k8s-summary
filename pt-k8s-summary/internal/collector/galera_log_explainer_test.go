package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeMemberLog(t *testing.T, root, ns, pod, body string) string {
	t.Helper()
	dir := filepath.Join(root, ns, pod, "var", "lib", "mysql")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "mysqld-error.log")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSummarizeGaleraLogsMarksQuietNodes(t *testing.T) {
	root := t.TempDir()
	noisy := writeMemberLog(t, root, "ns", "db-pxc-0", strings.Join([]string{
		`2026-07-31T00:00:00.072189Z 1 [Warning] [MY-013360] [Server] deprecated`,
		`2026-07-31T06:44:06.421099Z 2 [Warning] [MY-013360] [Server] deprecated`,
		"",
	}, "\n"))
	active := writeMemberLog(t, root, "ns", "db-pxc-1", strings.Join([]string{
		`2026-07-31T01:00:01.408995Z 1 [Warning] [MY-013360] [Server] deprecated`,
		`2026-07-31T05:00:12.070417Z 0 [Warning] [MY-000000] [Galera] last inactive check`,
		"",
	}, "\n"))

	// The explainer omits files it recognizes nothing in, so only pxc-1 appears.
	out := "identifier   " + active + "\n2026-07-31T05:00:12.070417Z   inactive check more than 1.5s"

	cov := summarizeGaleraLogs([]string{noisy, active}, out)
	if len(cov) != 2 {
		t.Fatalf("want 2 rows, got %d", len(cov))
	}

	if cov[0].Pod != "db-pxc-0" {
		t.Fatalf("pod name: %q", cov[0].Pod)
	}
	if cov[0].HasEvents {
		t.Fatal("pxc-0 is absent from the timeline and must not be marked as having events")
	}
	if cov[0].First != "2026-07-31T00:00:00.072189Z" || cov[0].Last != "2026-07-31T06:44:06.421099Z" {
		t.Fatalf("pxc-0 span: %q → %q", cov[0].First, cov[0].Last)
	}

	if !cov[1].HasEvents {
		t.Fatal("pxc-1 appears in the timeline and must be marked as having events")
	}
	if cov[1].First != "2026-07-31T01:00:01.408995Z" {
		t.Fatalf("pxc-1 first: %q", cov[1].First)
	}
}

func TestRenderGaleraCoverageExplainsQuietLogs(t *testing.T) {
	h := renderGaleraCoverageHTML([]galeraLogCoverage{
		{Pod: "db-pxc-0", First: "2026-07-31T00:00:00Z", Last: "2026-07-31T06:44:06Z"},
		{Pod: "db-pxc-1", First: "2026-07-31T01:00:01Z", Last: "2026-07-31T06:44:29Z", HasEvents: true},
	})
	for _, want := range []string{
		"Member logs scanned: 2",
		"with recognized events: 1",
		"not a parsing failure",
		"db-pxc-0",
		"no recognized events",
	} {
		if !strings.Contains(h, want) {
			t.Fatalf("missing %q in:\n%s", want, h)
		}
	}
}

func TestRenderGaleraCoverageAllActiveHasNoCaveat(t *testing.T) {
	h := renderGaleraCoverageHTML([]galeraLogCoverage{
		{Pod: "db-pxc-0", First: "a", Last: "b", HasEvents: true},
	})
	if strings.Contains(h, "not a parsing failure") {
		t.Fatal("caveat should only appear when some log produced no events")
	}
}
