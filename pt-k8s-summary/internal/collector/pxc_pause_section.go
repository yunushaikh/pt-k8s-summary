package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type pauseRow struct {
	Name, Namespace string
	Present         bool
	Paused          bool // meaningful when Present
}

type pausePXCListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			Pause *bool `yaml:"pause"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPauseRows(dumpRoot string) ([]pauseRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var rows []pauseRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pausePXCListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("yaml %s: %w", p, err)
		}
		for i := range list.Items {
			md := list.Items[i].Metadata
			name := strings.TrimSpace(md.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(md.Namespace)
			if ns == "" {
				ns = nsHint
			}
			pp := list.Items[i].Spec.Pause
			pr := pauseRow{Name: name, Namespace: ns}
			if pp != nil {
				pr.Present = true
				pr.Paused = *pp
			}
			rows = append(rows, pr)
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

// renderPauseBadgeHTML returns only the pause status pill (styles live in pxc_unsafe_pause_row_section).
func renderPauseBadgeHTML(r pauseRow) string {
	var b strings.Builder
	b.WriteString(`<span class="pause-val `)
	if !r.Present {
		b.WriteString(`unset">pause: not set`)
	} else if r.Paused {
		b.WriteString(`true">pause: true`)
	} else {
		b.WriteString(`false">pause: false`)
	}
	b.WriteString(`</span>`)
	return b.String()
}
