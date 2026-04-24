package collector

import (
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"pt-k8s-summary/internal/dumpctx"

	"gopkg.in/yaml.v3"
)

// pitrPXCSection reports PITR and backup image settings from each PXC CR list YAML.
type pitrPXCSection struct{}

func (pitrPXCSection) ID() string    { return "pxc-pitr" }
func (pitrPXCSection) Title() string { return "PITR" }

func (pitrPXCSection) Collect(ctx dumpctx.Context) (Section, error) {
	rows, err := gatherPITRRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(rows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderPITRTable(rows))}, nil
}

type pitrRow struct {
	Name, Namespace    string
	PITREnabled        bool
	PITRStorageName    string
	TimeBetweenUploads string
	BackupImage        string
}

type pitrPXCListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			Backup struct {
				Image string `yaml:"image"`
				PITR  struct {
					Enabled            bool   `yaml:"enabled"`
					StorageName        string `yaml:"storageName"`
					TimeBetweenUploads any    `yaml:"timeBetweenUploads"`
				} `yaml:"pitr"`
			} `yaml:"backup"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherPITRRows(dumpRoot string) ([]pitrRow, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var rows []pitrRow
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list pitrPXCListDoc
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
			bu := list.Items[i].Spec.Backup
			pitr := bu.PITR
			tbu := formatPITRTimeBetweenUploads(pitr.TimeBetweenUploads)
			st := strings.TrimSpace(pitr.StorageName)
			if st == "" {
				st = "—"
			}
			img := strings.TrimSpace(bu.Image)
			if img == "" {
				img = "—"
			}
			rows = append(rows, pitrRow{
				Name:               name,
				Namespace:          ns,
				PITREnabled:        pitr.Enabled,
				PITRStorageName:    st,
				TimeBetweenUploads: tbu,
				BackupImage:        img,
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

func renderPITRTable(rows []pitrRow) string {
	var b strings.Builder
	b.WriteString(`<style>
.pitr-wrap { font-family: ui-sans-serif, system-ui, sans-serif; font-size: 0.82rem; color: #1e293b; overflow-x: auto; }
.pitr-table { width: 100%; border-collapse: collapse; min-width: 36rem; }
.pitr-table th, .pitr-table td { border: 1px solid #e2e8f0; padding: 0.4rem 0.5rem; text-align: left; vertical-align: top; }
.pitr-table th { background: #f1f5f9; font-weight: 600; font-size: 0.78rem; }
.pitr-table td { background: #fff; }
.pitr-table code { font-family: ui-monospace, Menlo, monospace; font-size: 0.76rem; background: #f8fafc; padding: 0.1rem 0.25rem; border-radius: 4px; }
.pitr-pill { display: inline-block; font-size: 0.68rem; font-weight: 700; padding: 0.15rem 0.45rem; border-radius: 999px; }
.pitr-pill.on { background: #0d9488; color: #fff; }
.pitr-pill.off { background: #94a3b8; color: #fff; }
</style>`)
	b.WriteString(`<div class="pitr-wrap"><table class="pitr-table"><thead><tr>`)
	b.WriteString(`<th>Cluster</th><th>Namespace</th><th>PITR</th><th>PITR storage</th>`)
	b.WriteString(`<th><code>timeBetweenUploads</code></th><th>Backup image</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		pill := `<span class="pitr-pill off">off</span>`
		if r.PITREnabled {
			pill = `<span class="pitr-pill on">on</span>`
		}
		b.WriteString(`<tr><td><code>`)
		b.WriteString(html.EscapeString(r.Name))
		b.WriteString(`</code></td><td><code>`)
		b.WriteString(html.EscapeString(r.Namespace))
		b.WriteString(`</code></td><td>`)
		b.WriteString(pill)
		b.WriteString(`</td><td><code>`)
		b.WriteString(html.EscapeString(r.PITRStorageName))
		b.WriteString(`</code></td><td>`)
		if r.TimeBetweenUploads == "—" {
			b.WriteString(`—`)
		} else {
			b.WriteString(`<code>`)
			b.WriteString(html.EscapeString(r.TimeBetweenUploads))
			b.WriteString(`</code>`)
		}
		b.WriteString(`</td><td><code>`)
		b.WriteString(html.EscapeString(r.BackupImage))
		b.WriteString(`</code></td></tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func formatPITRTimeBetweenUploads(v any) string {
	if v == nil {
		return "—"
	}
	switch x := v.(type) {
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return "—"
		}
		return s
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}
