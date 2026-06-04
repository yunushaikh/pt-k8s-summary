package collector

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Known Percona XtraDBCluster unsafeFlags keys (order preserved for display).
var pxcUnsafeFlagKeys = []string{"tls", "pxcSize", "proxySize", "backupIfUnhealthy"}

type unsafeFlagCluster struct {
	Name, Namespace string
	Flags           map[string]unsafeFlagTri
}

// unsafeFlagTri: Present=false means key omitted from CR.
type unsafeFlagTri struct {
	Present bool
	Value   bool // meaningful when Present
}

type unsafePXCListDoc struct {
	Items []struct {
		Metadata struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			UnsafeFlags map[string]any `yaml:"unsafeFlags"`
		} `yaml:"spec"`
	} `yaml:"items"`
}

func gatherUnsafeFlagRows(dumpRoot string) ([]unsafeFlagCluster, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	var out []unsafeFlagCluster
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list unsafePXCListDoc
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
			raw := list.Items[i].Spec.UnsafeFlags
			flags := make(map[string]unsafeFlagTri)
			for _, key := range pxcUnsafeFlagKeys {
				if raw == nil {
					flags[key] = unsafeFlagTri{Present: false}
					continue
				}
				v, ok := raw[key]
				if !ok {
					flags[key] = unsafeFlagTri{Present: false}
					continue
				}
				b, parsed := parseBoolish(v)
				if !parsed {
					flags[key] = unsafeFlagTri{Present: false}
					continue
				}
				flags[key] = unsafeFlagTri{Present: true, Value: b}
			}
			out = append(out, unsafeFlagCluster{Name: name, Namespace: ns, Flags: flags})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func parseBoolish(v any) (bool, bool) {
	if v == nil {
		return false, false
	}
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		if s == "true" || s == "1" || s == "yes" {
			return true, true
		}
		if s == "false" || s == "0" || s == "no" || s == "" {
			return false, true
		}
		return false, false
	case int:
		return x != 0, true
	case int64:
		return x != 0, true
	case float64:
		return x != 0, true
	default:
		return false, false
	}
}

// renderUnsafeTabRowHTML returns the flag tab strip for one cluster (no outer wrapper or styles).
func renderUnsafeTabRowHTML(c unsafeFlagCluster) string {
	var b strings.Builder
	b.WriteString(`<div class="unsafe-tab-row">`)
	for _, key := range pxcUnsafeFlagKeys {
		tri := c.Flags[key]
		var bodyClass, label string
		switch {
		case !tri.Present:
			bodyClass = "unsafe-tab-st unset"
			label = "Not set"
		case tri.Value:
			bodyClass = "unsafe-tab-st active"
			label = "Active"
		default:
			bodyClass = "unsafe-tab-st inactive"
			label = "Inactive"
		}
		b.WriteString(`<div class="unsafe-tab"><div class="unsafe-tab-name">`)
		b.WriteString(html.EscapeString(key))
		b.WriteString(`</div><div class="` + bodyClass + `">`)
		b.WriteString(html.EscapeString(label))
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
