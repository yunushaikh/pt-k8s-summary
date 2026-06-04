package collector

import (
	"context"
	"errors"
	"fmt"
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
