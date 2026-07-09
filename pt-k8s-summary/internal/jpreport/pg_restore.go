package jpreport

import (
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

type pgRestoreYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec struct {
		PGCluster string `yaml:"pgCluster"`
		RepoName  string `yaml:"repoName"`
	} `yaml:"spec"`
	Status struct {
		State string `yaml:"state"`
	} `yaml:"status"`
}

// PGRestoreRowTmpl is one PerconaPGRestore row.
type PGRestoreRowTmpl struct {
	Name              string
	Namespace         string
	Cluster           string
	Repo              string
	Status            string
	Age               string
	RestoreYAMLModalID string
	RestoreYAMLEscaped string
}

func LoadPGRestoreRowsFromDump(dumpRoot string, now time.Time) ([]PGRestoreRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PGRestoreList)
	if err != nil {
		return nil, 0, err
	}
	var rows []PGRestoreRowTmpl
	for fileIdx, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var root map[string]interface{}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		rawItems, ok := root["items"].([]interface{})
		if !ok {
			continue
		}
		for itemIdx, raw := range rawItems {
			itemMap, ok := asStringKeyedMap(raw)
			if !ok {
				continue
			}
			itemBytes, err := yaml.Marshal(itemMap)
			if err != nil {
				continue
			}
			var r pgRestoreYAML
			if err := yaml.Unmarshal(itemBytes, &r); err != nil {
				continue
			}
			name := strings.TrimSpace(r.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(r.Metadata.Namespace)
			if ns == "" {
				ns = filepath.Base(filepath.Dir(p))
			}
			created := time.Time{}
			if r.Metadata.CreationTimestamp != "" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(r.Metadata.CreationTimestamp)); err == nil {
					created = t
				}
			}
			row := PGRestoreRowTmpl{
				Name:      name,
				Namespace: ns,
				Cluster:   orDash(strings.TrimSpace(r.Spec.PGCluster)),
				Repo:      orDash(strings.TrimSpace(r.Spec.RepoName)),
				Status:    orDash(strings.TrimSpace(r.Status.State)),
				Age:       HumanizeDurationInState(created, now),
			}
			if created.IsZero() {
				row.Age = "—"
			}
			if esc, id, ok := pgRestoreYAMLEscapedForModal(itemBytes, ns, name, fileIdx, itemIdx); ok {
				row.RestoreYAMLModalID = id
				row.RestoreYAMLEscaped = esc
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, len(paths), nil
}

func pgRestoreYAMLEscapedForModal(raw []byte, ns, name string, fileIdx, itemIdx int) (escaped, modalID string, ok bool) {
	if len(raw) == 0 || strings.TrimSpace(name) == "" {
		return "", "", false
	}
	modalID = "pgrestoreyaml-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(name) +
		"-f" + strconv.Itoa(fileIdx) + "-i" + strconv.Itoa(itemIdx)
	trunc := false
	if len(raw) > pgCRYAMLModalMaxBytes {
		raw = raw[:pgCRYAMLModalMaxBytes]
		trunc = true
	}
	s := string(raw)
	if trunc {
		s += "\n\n# … truncated for report embed"
	}
	return htmltemplate.HTMLEscapeString(s), modalID, true
}
