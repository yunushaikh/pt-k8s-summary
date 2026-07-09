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

type pgListDoc struct {
	Items []pgClusterYAML `yaml:"items"`
}

type pgClusterYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec   pgSpecYAML   `yaml:"spec"`
	Status pgStatusYAML `yaml:"status"`
}

type pgSpecYAML struct {
	CRVersion       string `yaml:"crVersion"`
	PostgresVersion int    `yaml:"postgresVersion"`
	Image           string `yaml:"image"`
	PMM             struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"pmm"`
}

type pgStatusYAML struct {
	State    string `yaml:"state"`
	Host     string `yaml:"host"`
	Postgres struct {
		Ready   int    `yaml:"ready"`
		Size    int    `yaml:"size"`
		Version int    `yaml:"version"`
	} `yaml:"postgres"`
	PGBouncer struct {
		Ready int `yaml:"ready"`
		Size  int `yaml:"size"`
	} `yaml:"pgbouncer"`
	Conditions []pxcCRCondition `yaml:"conditions"`
}

// PGRowTmpl is one PerconaPGCluster row (kubectl get pg style).
type PGRowTmpl struct {
	Name             string
	Namespace        string
	CRYAMLModalID    string
	CRYAMLEscaped    string
	CRVersion        string
	PostgresVersion  string
	Endpoint         string
	Status           string
	PostgresCount    string
	PGBouncerCount   string
	Age              string
	PMMEnabled       string
}

const pgCRYAMLModalMaxBytes = 512 * 1024

func safePGCRYAMLModalID(ns, crName string, fileIdx, itemIdx int) string {
	return "pgcryaml-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(crName) +
		"-f" + strconv.Itoa(fileIdx) + "-i" + strconv.Itoa(itemIdx)
}

func pgCRYAMLEscapedForModal(fileBytes []byte, cr *pgClusterYAML, fileIdx, itemIdx int) (escaped string, modalID string, ok bool) {
	if cr == nil {
		return "", "", false
	}
	ns := strings.TrimSpace(cr.Metadata.Namespace)
	name := strings.TrimSpace(cr.Metadata.Name)
	if name == "" {
		return "", "", false
	}
	modalID = safePGCRYAMLModalID(ns, name, fileIdx, itemIdx)
	var raw []byte
	if b, hit := extractListItemYAMLRaw(fileBytes, ns, name); hit {
		raw = b
	} else {
		var err error
		raw, err = yaml.Marshal(cr)
		if err != nil || len(raw) == 0 {
			return "", "", false
		}
	}
	trunc := false
	if len(raw) > pgCRYAMLModalMaxBytes {
		raw = raw[:pgCRYAMLModalMaxBytes]
		trunc = true
	}
	s := string(raw)
	if trunc {
		s += "\n\n# … truncated for report embed (see raw cluster dump for full document)"
	}
	return htmltemplate.HTMLEscapeString(s), modalID, true
}

// ListPGYAMLFiles returns paths to PerconaPGCluster list YAML under dumpRoot.
func ListPGYAMLFiles(root string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return dumpfiles.FindListYAMLFiles(root, dumpfiles.PGClusterList)
}

func LoadPGRowsFromDump(dumpRoot string, now time.Time) ([]PGRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PGClusterList)
	if err != nil {
		return nil, 0, err
	}
	seen := make(map[string]struct{})
	var rows []PGRowTmpl
	for fileIdx, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var list pgListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		nsHint := filepath.Base(filepath.Dir(p))
		for itemIdx := range list.Items {
			cr := &list.Items[itemIdx]
			name := strings.TrimSpace(cr.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(cr.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
				cr.Metadata.Namespace = ns
			}
			key := ns + "\x00" + name
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			row := buildPGRowTmpl(cr, now)
			if esc, id, ok := pgCRYAMLEscapedForModal(data, cr, fileIdx, itemIdx); ok {
				row.CRYAMLModalID = id
				row.CRYAMLEscaped = esc
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

func buildPGRowTmpl(cr *pgClusterYAML, now time.Time) PGRowTmpl {
	age := "—"
	if cr.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(cr.Metadata.CreationTimestamp)); err == nil {
			age = HumanizeDurationInState(t, now)
		}
	}
	crVer := strings.TrimSpace(cr.Spec.CRVersion)
	pgVer := strconv.Itoa(cr.Spec.PostgresVersion)
	if cr.Status.Postgres.Version > 0 {
		pgVer = strconv.Itoa(cr.Status.Postgres.Version)
	}
	if pgVer == "0" {
		pgVer = "—"
	}
	status := strings.TrimSpace(cr.Status.State)
	if status == "" {
		status = "—"
	}
	endpoint := strings.TrimSpace(cr.Status.Host)
	if endpoint == "" {
		endpoint = "—"
	}
	pmm := "no"
	if cr.Spec.PMM.Enabled {
		pmm = "yes"
	}
	return PGRowTmpl{
		Name:            strings.TrimSpace(cr.Metadata.Name),
		Namespace:       strings.TrimSpace(cr.Metadata.Namespace),
		CRVersion:       orDash(crVer),
		PostgresVersion: pgVer,
		Endpoint:        endpoint,
		Status:          status,
		PostgresCount:   pgComponentCount(cr.Status.Postgres.Ready, cr.Status.Postgres.Size),
		PGBouncerCount:  pgComponentCount(cr.Status.PGBouncer.Ready, cr.Status.PGBouncer.Size),
		Age:             age,
		PMMEnabled:      pmm,
	}
}

func pgComponentCount(ready, size int) string {
	if size == 0 && ready == 0 {
		return "—"
	}
	if size > 0 {
		return strconv.Itoa(ready)
	}
	return strconv.Itoa(ready)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return strings.TrimSpace(s)
}
