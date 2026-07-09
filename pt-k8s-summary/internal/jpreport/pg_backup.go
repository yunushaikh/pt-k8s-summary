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

type pgBackupYAML struct {
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
		State      string `yaml:"state"`
		BackupType string `yaml:"backupType"`
		Completed  string `yaml:"completed"`
		BackupName string `yaml:"backupName"`
	} `yaml:"status"`
}

// PGBackupRowTmpl is one PerconaPGBackup row (kubectl get perconapgbackups style).
type PGBackupRowTmpl struct {
	Name                  string
	Namespace             string
	Cluster               string
	Repo                  string
	Destination           string
	Status                string
	BackupType            string
	Completed             string
	Age                   string
	BackupYAMLModalID     string
	BackupYAMLEscaped     string
}

func LoadPGBackupRowsFromDump(dumpRoot string, now time.Time) ([]PGBackupRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PGBackupList)
	if err != nil {
		return nil, 0, err
	}
	var pending []struct {
		t   time.Time
		row PGBackupRowTmpl
	}
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
			var b pgBackupYAML
			if err := yaml.Unmarshal(itemBytes, &b); err != nil {
				continue
			}
			name := strings.TrimSpace(b.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(b.Metadata.Namespace)
			if ns == "" {
				ns = filepath.Base(filepath.Dir(p))
			}
			created := time.Time{}
			if b.Metadata.CreationTimestamp != "" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(b.Metadata.CreationTimestamp)); err == nil {
					created = t
				}
			}
			completedAge := "—"
			if b.Status.Completed != "" {
				if t, err := time.Parse(time.RFC3339, strings.TrimSpace(b.Status.Completed)); err == nil {
					completedAge = HumanizeDurationInState(t, now)
				}
			}
			row := PGBackupRowTmpl{
				Name:        name,
				Namespace:   ns,
				Cluster:     orDash(strings.TrimSpace(b.Spec.PGCluster)),
				Repo:        orDash(strings.TrimSpace(b.Spec.RepoName)),
				Destination: orDash(strings.TrimSpace(b.Status.BackupName)),
				Status:      orDash(strings.TrimSpace(b.Status.State)),
				BackupType:  orDash(strings.TrimSpace(b.Status.BackupType)),
				Completed:   completedAge,
				Age:         HumanizeDurationInState(created, now),
			}
			if created.IsZero() {
				row.Age = "—"
			}
			if esc, id, ok := pgBackupYAMLEscapedForModal(itemBytes, ns, name, fileIdx, itemIdx); ok {
				row.BackupYAMLModalID = id
				row.BackupYAMLEscaped = esc
			}
			pending = append(pending, struct {
				t   time.Time
				row PGBackupRowTmpl
			}{created, row})
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].t.After(pending[j].t)
	})
	rows := make([]PGBackupRowTmpl, len(pending))
	for i := range pending {
		rows[i] = pending[i].row
	}
	return rows, len(paths), nil
}

func pgBackupYAMLEscapedForModal(raw []byte, ns, name string, fileIdx, itemIdx int) (escaped, modalID string, ok bool) {
	if len(raw) == 0 || strings.TrimSpace(name) == "" {
		return "", "", false
	}
	modalID = "pgbackupyaml-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(name) +
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
