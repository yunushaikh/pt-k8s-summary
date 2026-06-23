package jpreport

import (
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

type psRestoreYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec struct {
		ClusterName string `yaml:"clusterName"`
		BackupName  string `yaml:"backupName"`
		PITR        *struct {
			Type  string `yaml:"type"`
			Date  string `yaml:"date"`
			GTID  string `yaml:"gtid"`
			Force bool   `yaml:"force"`
		} `yaml:"pitr"`
	} `yaml:"spec"`
	Status struct {
		State     string `yaml:"state"`
		StateDesc string `yaml:"stateDescription"`
	} `yaml:"status"`
}

// PSRestoreRowTmpl is one PerconaServerMySQLRestore row for the HTML report.
type PSRestoreRowTmpl struct {
	Name                  string
	Namespace             string
	Cluster               string
	BackupName            string
	PITRTarget            string
	Status                string
	Age                   string
	RestoreManifestEscaped string
	RestoreManifestModalID string
}

func LoadPSRestoreRowsFromDump(dumpRoot string, now time.Time) ([]PSRestoreRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PSRestoreList)
	if err != nil {
		return nil, 0, err
	}
	var pending []struct {
		t   time.Time
		raw *psRestoreYAML
		manifest string
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var root map[string]interface{}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		rawItems, ok := root["items"].([]interface{})
		if !ok || rawItems == nil {
			continue
		}
		for _, raw := range rawItems {
			itemMap, ok := asStringKeyedMap(raw)
			if !ok {
				continue
			}
			itemBytes, err := yaml.Marshal(itemMap)
			if err != nil {
				continue
			}
			var r psRestoreYAML
			if err := yaml.Unmarshal(itemBytes, &r); err != nil {
				continue
			}
			if strings.TrimSpace(r.Metadata.Name) == "" {
				continue
			}
			ts := time.Time{}
			if s := strings.TrimSpace(r.Metadata.CreationTimestamp); s != "" {
				if t, err := time.Parse(time.RFC3339, s); err == nil {
					ts = t
				}
			}
			manifest := strings.TrimSuffix(string(itemBytes), "\n") + "\n"
			rc := r
			pending = append(pending, struct {
				t        time.Time
				raw      *psRestoreYAML
				manifest string
			}{t: ts, raw: &rc, manifest: manifest})
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].t.Equal(pending[j].t) {
			return pending[i].raw.Metadata.Name > pending[j].raw.Metadata.Name
		}
		return pending[i].t.After(pending[j].t)
	})
	rows := make([]PSRestoreRowTmpl, 0, len(pending))
	for _, x := range pending {
		rows = append(rows, buildPSRestoreRowTmpl(x.raw, now, x.manifest))
	}
	return rows, len(paths), nil
}

func buildPSRestoreRowTmpl(r *psRestoreYAML, now time.Time, manifestYAML string) PSRestoreRowTmpl {
	ns := strings.TrimSpace(r.Metadata.Namespace)
	name := strings.TrimSpace(r.Metadata.Name)
	cluster := strings.TrimSpace(r.Spec.ClusterName)
	if cluster == "" {
		cluster = "—"
	}
	backup := strings.TrimSpace(r.Spec.BackupName)
	if backup == "" {
		backup = "—"
	}
	st := strings.TrimSpace(r.Status.State)
	if st == "" {
		st = "—"
	}
	pitr := formatPSRestorePITR(r.Spec.PITR)
	age := "—"
	if r.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(r.Metadata.CreationTimestamp)); err == nil {
			age = HumanizeDurationInState(t, now)
		}
	}
	return PSRestoreRowTmpl{
		Name: name, Namespace: ns, Cluster: cluster, BackupName: backup,
		PITRTarget: pitr, Status: st, Age: age,
		RestoreManifestEscaped: htmltemplate.HTMLEscapeString(manifestYAML),
		RestoreManifestModalID: safePSRestoreManifestStoreID(ns, name),
	}
}

func formatPSRestorePITR(p *struct {
	Type  string `yaml:"type"`
	Date  string `yaml:"date"`
	GTID  string `yaml:"gtid"`
	Force bool   `yaml:"force"`
}) string {
	if p == nil {
		return "—"
	}
	typ := strings.TrimSpace(p.Type)
	if typ == "" {
		return "—"
	}
	switch typ {
	case "date":
		if d := strings.TrimSpace(p.Date); d != "" {
			return "date " + d
		}
	case "gtid":
		if g := strings.TrimSpace(p.GTID); g != "" {
			return "gtid " + g
		}
	}
	if p.Force {
		return typ + " (force)"
	}
	return typ
}

func safePSRestoreManifestStoreID(ns, restoreName string) string {
	var b strings.Builder
	b.WriteString("psrestoreyaml-")
	for _, r := range ns + "-" + restoreName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}
