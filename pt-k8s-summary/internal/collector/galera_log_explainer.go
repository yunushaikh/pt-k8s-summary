package collector

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Per docs: https://docs.percona.com/percona-toolkit/pt-galera-log-explainer.html
const galeraExplainerTimeout = 90 * time.Second

// pxcMemberPodNameRE matches PXC statefulset member pods, e.g. pxc-db-pxc-0.
var pxcMemberPodNameRE = regexp.MustCompile(`(?i).*-pxc-\d+$`)

// findPXCMysqldErrorLogPaths returns absolute paths to var/lib/mysql/mysqld-error.log
// for each PXC MySQL member pod in the dump (excludes haproxy/proxysql and operator by pattern).
func findPXCMysqldErrorLogPaths(dumpRoot string) ([]string, error) {
	ents, err := os.ReadDir(dumpRoot)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, nsEnt := range ents {
		if !nsEnt.IsDir() {
			continue
		}
		nsName := nsEnt.Name()
		nsPath := filepath.Join(dumpRoot, nsName)
		pods, err := os.ReadDir(nsPath)
		if err != nil {
			continue
		}
		for _, pEnt := range pods {
			if !pEnt.IsDir() {
				continue
			}
			podName := pEnt.Name()
			if !pxcMemberPodNameRE.MatchString(podName) {
				continue
			}
			abs := filepath.Join(nsPath, podName, "var", "lib", "mysql", "mysqld-error.log")
			if st, err := os.Stat(abs); err == nil && !st.IsDir() {
				out = append(out, abs)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// galeraLogCoverage describes one member log so a near-empty timeline can be read correctly:
// a quiet window with no state transitions looks identical to a broken parse otherwise.
type galeraLogCoverage struct {
	Pod       string
	First     string
	Last      string
	HasEvents bool
}

var galeraLogTimestampRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)`)

// podNameFromMysqldErrorLogPath maps <ns>/<pod>/var/lib/mysql/mysqld-error.log to <pod>.
func podNameFromMysqldErrorLogPath(p string) string {
	d := filepath.Dir(p) // …/var/lib/mysql
	for i := 0; i < 3; i++ {
		d = filepath.Dir(d)
	}
	if base := filepath.Base(d); base != "." && base != string(filepath.Separator) {
		return base
	}
	return filepath.Base(p)
}

// logTimeSpan returns the first and last log timestamps, streaming so large logs stay cheap.
func logTimeSpan(path string) (first, last string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			head := line
			if len(head) > 32 {
				head = head[:32]
			}
			if m := galeraLogTimestampRE.FindStringSubmatch(head); m != nil {
				if first == "" {
					first = m[1]
				}
				last = m[1]
			}
		}
		if err != nil {
			return first, last
		}
	}
}

// summarizeGaleraLogs pairs each scanned member log with the timeline output. A log counts as
// having events when the explainer printed a column for it; the tool omits files entirely when
// it recognizes nothing in them.
func summarizeGaleraLogs(paths []string, explainerOut string) []galeraLogCoverage {
	var out []galeraLogCoverage
	for _, p := range paths {
		pod := podNameFromMysqldErrorLogPath(p)
		first, last := logTimeSpan(p)
		// The identifier row carries the full path; the "current path" row is left-truncated,
		// so also try the tail as a fallback.
		hasEvents := strings.Contains(explainerOut, p)
		if !hasEvents {
			if tail := pod + "/var/lib/mysql/" + filepath.Base(p); strings.Contains(explainerOut, tail) {
				hasEvents = true
			}
		}
		out = append(out, galeraLogCoverage{Pod: pod, First: first, Last: last, HasEvents: hasEvents})
	}
	return out
}

// renderGaleraCoverageHTML shows which member logs fed the timeline and the window each covers.
func renderGaleraCoverageHTML(cov []galeraLogCoverage) string {
	if len(cov) == 0 {
		return ""
	}
	esc := html.EscapeString
	quiet := 0
	for _, c := range cov {
		if !c.HasEvents {
			quiet++
		}
	}
	var b strings.Builder
	b.WriteString(`<p class="pxc-gle-meta" style="color:#64748b;margin:0.6rem 0 0.25rem 0;">`)
	b.WriteString(esc(fmt.Sprintf("Member logs scanned: %d · with recognized events: %d", len(cov), len(cov)-quiet)))
	if quiet > 0 {
		b.WriteString(` — the timeline only lists nodes whose logs contain events it recognizes (SST/IST, view changes, state transitions, restarts). A log with none is usually a quiet window or one filled with repeated warnings, not a parsing failure.`)
	}
	b.WriteString(`</p>`)
	b.WriteString(`<table class="pxc-inner-table"><thead><tr><th>Member log</th><th>Log covers</th><th>In timeline</th></tr></thead><tbody>`)
	for _, c := range cov {
		span := "—"
		if c.First != "" && c.Last != "" {
			span = c.First + " → " + c.Last
		}
		b.WriteString(`<tr><td><code>` + esc(c.Pod) + `</code></td><td>` + esc(span) + `</td><td>`)
		if c.HasEvents {
			b.WriteString(`<span class="unsafe-flags-ok">yes</span>`)
		} else {
			b.WriteString(`<span class="status-muted">no recognized events</span>`)
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}

// runPTGaleraLogExplainer runs: pt-galera-log-explainer [ --since=RFC3339 ] [ --pxc-operator ] --no-color list --all <paths>
//
// We do not use --merge-by-directory: for paths like .../pxc-N/var/lib/mysql/mysqld-error.log, that
// flag merges on the "mysql" directory and collapses the table to one column. Without it, the tool
// prints one side-by-side column per node (see Percona "list" examples with several log paths).
// since: if non-empty, only events at or after that instant are listed (RFC3339, per --since in docs).
// See: https://docs.percona.com/percona-toolkit/pt-galera-log-explainer.html
func runPTGaleraLogExplainer(logPaths []string, since string) (stdOut, stdErr string, err error) {
	if len(logPaths) == 0 {
		return "", "", nil
	}
	since = strings.TrimSpace(since)
	try := func(usePXCOp bool) (o, e string, runErr error) {
		ctx, cancel := context.WithTimeout(context.Background(), galeraExplainerTimeout)
		defer cancel()
		var args []string
		if since != "" {
			args = append(args, "--since="+since)
		}
		if usePXCOp {
			args = append(args, "--pxc-operator")
		}
		args = append(args, "--no-color", "list", "--all")
		args = append(args, logPaths...)
		cmd := exec.CommandContext(ctx, "pt-galera-log-explainer", args...)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr = cmd.Run()
		outS := strings.TrimSpace(stdout.String())
		errS := strings.TrimSpace(stderr.String())
		if runErr != nil {
			if errors.Is(runErr, exec.ErrNotFound) {
				return "", errS, fmt.Errorf("pt-galera-log-explainer not found in PATH (install Percona Toolkit)")
			}
			if errS != "" {
				return "", errS, fmt.Errorf("%w: %s", runErr, errS)
			}
			return "", errS, runErr
		}
		if outS == "" {
			if errS != "" {
				return "", errS, fmt.Errorf("no tabular output (stderr: %s)", errS)
			}
			return "", "", nil
		}
		return outS, errS, nil
	}

	out, serr, err := try(false)
	if err == nil {
		return out, serr, nil
	}
	combined := err.Error() + " " + serr
	if strings.Contains(combined, "could not find data") {
		if out2, serr2, err2 := try(true); err2 == nil {
			return out2, serr2, nil
		}
	}
	return out, serr, err
}
