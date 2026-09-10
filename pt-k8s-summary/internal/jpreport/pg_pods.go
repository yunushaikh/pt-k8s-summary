package jpreport

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PGPodRowTmpl is one PostgreSQL workload pod (instance or pgBouncer) for kubectl-style tables.
type PGPodRowTmpl struct {
	Name              string
	Namespace         string
	Kind              string // PostgreSQL | pgBouncer | workload
	RolePod           string // Leader | Secondary | — (from pods.yaml label)
	RolePatroni       string // Leader | Secondary | — (from Patroni log lines)
	RolePatroniFollow string // leader pod name when secondary (from log)
	RoleMismatch      bool   // both roles set and disagree
	Ready             string
	Status            string
	Restarts          string
	Age               string
	PodIP             string
	Node              string
	PostgresVersion   string
}

// LoadPGWorkloadPodRows returns sorted postgres + pgBouncer pods from merged pods.yaml.
func LoadPGWorkloadPodRows(pods *PodLoader, dumpRoot string, now time.Time) []PGPodRowTmpl {
	if pods == nil {
		return nil
	}
	var rows []PGPodRowTmpl
	for i := range pods.all {
		p := &pods.all[i]
		ns := strings.TrimSpace(p.Metadata.Namespace)
		name := strings.TrimSpace(p.Metadata.Name)
		if ns == "" || name == "" {
			continue
		}
		if OperatorKindForPod(pods, ns, name) != "pg-workload" {
			continue
		}
		row := podItemToRow(p, now, dumpRoot)
		kind := pgWorkloadKind(p)
		rolePod := pgRoleFromPodLabel(p)
		rolePat, follow := "", ""
		if kind == "PostgreSQL" {
			rolePat, follow = pgRoleFromPatroniLogs(dumpRoot, ns, name)
		}
		mismatch := rolePod != "—" && rolePat != "—" && rolePod != rolePat
		rows = append(rows, PGPodRowTmpl{
			Name:              row.Name,
			Namespace:         ns,
			Kind:              kind,
			RolePod:           rolePod,
			RolePatroni:       dashIfEmpty(rolePat),
			RolePatroniFollow: follow,
			RoleMismatch:      mismatch,
			Ready:             row.Ready,
			Status:            row.Status,
			Restarts:          row.Restarts,
			Age:               row.Age,
			PodIP:             row.PodIP,
			Node:              row.Node,
			PostgresVersion:   pgPostgresVersionFromPod(p),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if pi, pj := pgRoleSortKey(rows[i]), pgRoleSortKey(rows[j]); pi != pj {
			return pi < pj
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func pgRoleSortKey(r PGPodRowTmpl) int {
	// Leader first, then Secondary, then unknown instance, then pgBouncer/other.
	switch {
	case r.RolePod == "Leader" || r.RolePatroni == "Leader":
		return 0
	case r.RolePod == "Secondary" || r.RolePatroni == "Secondary":
		return 1
	case r.Kind == "PostgreSQL":
		return 2
	case r.Kind == "pgBouncer":
		return 3
	default:
		return 4
	}
}

func pgWorkloadKind(p *podItem) string {
	if p == nil {
		return "—"
	}
	l := p.Metadata.Labels
	if l != nil {
		switch strings.TrimSpace(l["postgres-operator.crunchydata.com/role"]) {
		case "pgbouncer":
			return "pgBouncer"
		case "primary", "replica":
			return "PostgreSQL"
		}
		if strings.TrimSpace(l["postgres-operator.crunchydata.com/data"]) == "postgres" {
			return "PostgreSQL"
		}
	}
	n := strings.ToLower(p.Metadata.Name)
	if strings.Contains(n, "pgbouncer") {
		return "pgBouncer"
	}
	if strings.Contains(n, "-instance") {
		return "PostgreSQL"
	}
	return "workload"
}

// pgRoleFromPodLabel maps Crunchy/Percona PG operator role labels to Leader/Secondary.
func pgRoleFromPodLabel(p *podItem) string {
	if p == nil || p.Metadata.Labels == nil {
		return "—"
	}
	switch strings.TrimSpace(p.Metadata.Labels["postgres-operator.crunchydata.com/role"]) {
	case "primary":
		return "Leader"
	case "replica":
		return "Secondary"
	default:
		return "—"
	}
}

// Patroni heartbeat lines in database container logs (latest match wins).
var (
	patroniLeaderRE    = regexp.MustCompile(`(?i)I am \(([^)]+)\), the leader with the lock`)
	patroniSecondaryRE = regexp.MustCompile(`(?i)I am \(([^)]+)\), a secondary, and following a leader \(([^)]+)\)`)
	patroniPrimaryRE   = regexp.MustCompile(`(?i)I am \(([^)]+)\), a primary\b`)
)

// pgRoleFromPatroniLogs scans instance pod dump logs for the latest Patroni role line.
func pgRoleFromPatroniLogs(dumpRoot, namespace, podName string) (role, follow string) {
	content := readPGPodLogPlain(dumpRoot, namespace, podName)
	if content == "" {
		return "", ""
	}
	return parsePatroniRoleFromLog(content, podName)
}

func parsePatroniRoleFromLog(content, podName string) (role, follow string) {
	want := strings.TrimSpace(podName)
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if m := patroniSecondaryRE.FindStringSubmatch(line); m != nil {
			if want == "" || m[1] == want {
				return "Secondary", strings.TrimSpace(m[2])
			}
			continue
		}
		if m := patroniLeaderRE.FindStringSubmatch(line); m != nil {
			if want == "" || m[1] == want {
				return "Leader", ""
			}
			continue
		}
		if m := patroniPrimaryRE.FindStringSubmatch(line); m != nil {
			if want == "" || m[1] == want {
				return "Leader", ""
			}
		}
	}
	return "", ""
}

func readPGPodLogPlain(dumpRoot, namespace, podName string) string {
	ns := strings.TrimSpace(namespace)
	pn := strings.TrimSpace(podName)
	if ns == "" || pn == "" || strings.TrimSpace(dumpRoot) == "" {
		return ""
	}
	base := filepath.Clean(dumpRoot)
	candidates := []string{
		filepath.Join(base, ns, pn, "logs.txt"),
		filepath.Join(base, ns, pn, "summary.txt"),
		filepath.Join(base, ns, pn, "log"),
	}
	var parts []string
	for _, cand := range candidates {
		raw, err := readTailBytes(cand, 512*1024)
		if err != nil || len(raw) == 0 {
			continue
		}
		parts = append(parts, flattenPodLogJSONLines(string(raw)))
	}
	return strings.Join(parts, "\n")
}

func readTailBytes(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size <= int64(max) {
		return os.ReadFile(path)
	}
	if _, err := f.Seek(size-int64(max), 0); err != nil {
		return nil, err
	}
	r := bufio.NewReader(f)
	// Drop partial first line after seek.
	_, _ = r.ReadString('\n')
	var b strings.Builder
	for {
		line, err := r.ReadString('\n')
		b.WriteString(line)
		if err != nil {
			break
		}
	}
	return []byte(b.String()), nil
}

func pgPostgresVersionFromPod(p *podItem) string {
	if p == nil {
		return "—"
	}
	for _, c := range p.Spec.Containers {
		img := strings.TrimSpace(c.Image)
		if img == "" {
			continue
		}
		if v := imageTagVersion(img); v != "" {
			return v
		}
	}
	return "—"
}

func imageTagVersion(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if i := strings.LastIndex(image, ":"); i >= 0 && i < len(image)-1 {
		tag := image[i+1:]
		if at := strings.Index(tag, "@"); at >= 0 {
			tag = tag[:at]
		}
		return tag
	}
	return ""
}
