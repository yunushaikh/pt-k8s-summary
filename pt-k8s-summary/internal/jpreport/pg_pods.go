package jpreport

import (
	"sort"
	"strings"
	"time"
)

// PGPodRowTmpl is one PostgreSQL workload pod (instance or pgBouncer) for kubectl-style tables.
type PGPodRowTmpl struct {
	Name            string
	Namespace       string
	Role            string
	Ready           string
	Status          string
	Restarts        string
	Age             string
	PodIP           string
	Node            string
	PostgresVersion string
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
		role := pgWorkloadRole(p)
		rows = append(rows, PGPodRowTmpl{
			Name:            row.Name,
			Namespace:       ns,
			Role:            role,
			Ready:           row.Ready,
			Status:          row.Status,
			Restarts:        row.Restarts,
			Age:             row.Age,
			PodIP:           row.PodIP,
			Node:            row.Node,
			PostgresVersion: pgPostgresVersionFromPod(p),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		if rows[i].Role != rows[j].Role {
			return rows[i].Role < rows[j].Role
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func pgWorkloadRole(p *podItem) string {
	if p == nil {
		return "—"
	}
	l := p.Metadata.Labels
	if l != nil {
		switch strings.TrimSpace(l["postgres-operator.crunchydata.com/role"]) {
		case "pgbouncer":
			return "pgBouncer"
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
