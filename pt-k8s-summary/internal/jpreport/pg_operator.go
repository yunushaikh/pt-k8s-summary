package jpreport

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PGOperatorRowTmpl summarizes a Percona PostgreSQL operator Deployment from dumps.
type PGOperatorRowTmpl struct {
	Name        string
	Namespace   string
	Created     string
	Version     string
	Concurrency string
	PMMEnabled  string
}

type deploymentListDoc struct {
	Items []deploymentItemYAML `yaml:"items"`
}

type deploymentItemYAML struct {
	Metadata struct {
		Name              string            `yaml:"name"`
		Namespace         string            `yaml:"namespace"`
		CreationTimestamp string            `yaml:"creationTimestamp"`
		Labels            map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name  string `yaml:"name"`
					Image string `yaml:"image"`
					Env   []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// LoadPGOperatorRowsFromDump parses operator Deployments and PMM summary from PG clusters.
func LoadPGOperatorRowsFromDump(dumpRoot string, now time.Time) ([]PGOperatorRowTmpl, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, err
	}
	pmmSummary := pgPMMSummaryFromClusters(dumpAbs)
	paths, err := findYAMLFilesByBasename(dumpAbs, "deployments.yaml")
	if err != nil {
		return nil, err
	}
	var rows []PGOperatorRowTmpl
	seen := make(map[string]struct{})
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		var list deploymentListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			continue
		}
		nsHint := filepath.Base(filepath.Dir(p))
		for i := range list.Items {
			d := &list.Items[i]
			if !isPGOperatorDeployment(d) {
				continue
			}
			name := strings.TrimSpace(d.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(d.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			key := ns + "\x00" + name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			created := "—"
			if d.Metadata.CreationTimestamp != "" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(d.Metadata.CreationTimestamp)); err == nil {
					created = HumanizeDurationInState(t, now)
				}
			}
			rows = append(rows, PGOperatorRowTmpl{
				Name:        name,
				Namespace:   ns,
				Created:     created,
				Version:     pgOperatorVersionFromDeployment(d),
				Concurrency: pgOperatorConcurrencyFromDeployment(d),
				PMMEnabled:  pmmSummary,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

func isPGOperatorDeployment(d *deploymentItemYAML) bool {
	if d == nil {
		return false
	}
	l := d.Metadata.Labels
	if l != nil {
		if strings.TrimSpace(l["pgv2.percona.com/control-plane"]) == "postgres-operator" {
			return true
		}
		if strings.TrimSpace(l["app.kubernetes.io/part-of"]) == "pg-operator" &&
			strings.TrimSpace(l["app.kubernetes.io/name"]) == "pg-operator" {
			return true
		}
		for k := range l {
			if strings.HasPrefix(k, "operators.coreos.com/percona-postgresql-operator") {
				return true
			}
		}
	}
	name := strings.ToLower(strings.TrimSpace(d.Metadata.Name))
	if name == "percona-postgresql-operator" || strings.Contains(name, "percona-postgresql-operator") {
		return true
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		img := strings.ToLower(strings.TrimSpace(c.Image))
		if strings.Contains(img, "percona-postgresql-operator") {
			return true
		}
	}
	return false
}

func pgOperatorVersionFromDeployment(d *deploymentItemYAML) string {
	for _, c := range d.Spec.Template.Spec.Containers {
		if v := imageTagVersion(c.Image); v != "" {
			return v
		}
	}
	return "—"
}

func pgOperatorConcurrencyFromDeployment(d *deploymentItemYAML) string {
	for _, c := range d.Spec.Template.Spec.Containers {
		for _, e := range c.Env {
			if strings.TrimSpace(e.Name) == "PGO_WORKERS" {
				v := strings.TrimSpace(e.Value)
				if v != "" {
					return v
				}
			}
		}
	}
	return "—"
}

func pgPMMSummaryFromClusters(dumpRoot string) string {
	rows, _, err := LoadPGRowsFromDump(dumpRoot, time.Now())
	if err != nil || len(rows) == 0 {
		return "—"
	}
	yes, no := 0, 0
	for _, r := range rows {
		switch r.PMMEnabled {
		case "yes":
			yes++
		case "no":
			no++
		}
	}
	if yes > 0 && no > 0 {
		return "mixed"
	}
	if yes > 0 {
		return "yes"
	}
	if no > 0 {
		return "no"
	}
	return "—"
}

func findYAMLFilesByBasename(root, basename string) ([]string, error) {
	root = filepath.Clean(root)
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Base(path), basename) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
