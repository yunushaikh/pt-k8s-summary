// Package dumpfiles locates operator list YAML exports in pt-k8s-debug-collector dumps.
// Preferred basenames match current collector output; kind sniffing is a fallback when names change.
package dumpfiles

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ListTarget describes a Kubernetes List document (kind: List, items: [...]).
type ListTarget struct {
	Kind               string
	PreferredBasenames []string
	// NameContains: fallback — basename must contain one fragment (case-insensitive) and items must match Kind.
	NameContains []string
}

var (
	PXCClusterList = ListTarget{
		Kind: "PerconaXtraDBCluster",
		PreferredBasenames: []string{
			"perconaxtradbclusters.pxc.percona.com.yaml",
			"perconaxtradbclusters.yaml",
		},
		NameContains: []string{"perconaxtradbcluster"},
	}
	PXCBackupList = ListTarget{
		Kind: "PerconaXtraDBClusterBackup",
		PreferredBasenames: []string{
			"perconaxtradbclusterbackups.pxc.percona.com.yaml",
			"perconaxtradbclusterbackups.yaml",
		},
		NameContains: []string{"perconaxtradbclusterbackup"},
	}
	PSClusterList = ListTarget{
		Kind: "PerconaServerMySQL",
		PreferredBasenames: []string{
			"perconaservermysqls.ps.percona.com.yaml",
			"perconaservermysqls.yaml",
		},
		NameContains: []string{"perconaservermysqls"},
	}
	PSBackupList = ListTarget{
		Kind: "PerconaServerMySQLBackup",
		PreferredBasenames: []string{
			"perconaservermysqlbackups.ps.percona.com.yaml",
			"perconaservermysqlbackups.yaml",
		},
		NameContains: []string{"perconaservermysqlbackup"},
	}
	PSRestoreList = ListTarget{
		Kind: "PerconaServerMySQLRestore",
		PreferredBasenames: []string{
			"perconaservermysqlrestores.ps.percona.com.yaml",
			"perconaservermysqlrestores.yaml",
		},
		NameContains: []string{"perconaservermysqlrestore"},
	}
)

// FindListYAMLFiles returns sorted absolute paths. Preferred basenames win; otherwise kind-matched fallbacks.
func FindListYAMLFiles(root string, target ListTarget) ([]string, error) {
	root = filepath.Clean(root)
	preferred := make(map[string]struct{}, len(target.PreferredBasenames))
	for _, b := range target.PreferredBasenames {
		preferred[strings.ToLower(strings.TrimSpace(b))] = struct{}{}
	}
	var primary, fallback []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		if _, ok := preferred[base]; ok {
			primary = append(primary, path)
			return nil
		}
		if !strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".yml") {
			return nil
		}
		if target.Kind == "" || len(target.NameContains) == 0 {
			return nil
		}
		for _, frag := range target.NameContains {
			if strings.Contains(base, strings.ToLower(frag)) {
				if listYAMLHasItemKind(path, target.Kind) {
					fallback = append(fallback, path)
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(primary) > 0 {
		return dedupeSorted(primary), nil
	}
	return dedupeSorted(fallback), nil
}

func dedupeSorted(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var out []string
	for _, p := range paths {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func listYAMLHasItemKind(path, wantKind string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return false
	}
	top := &root
	if top.Kind == yaml.DocumentNode && len(top.Content) > 0 {
		top = top.Content[0]
	}
	if top.Kind == yaml.AliasNode && len(top.Content) > 0 {
		top = top.Content[0]
	}
	if top.Kind != yaml.MappingNode {
		return false
	}
	items := yamlMapValue(top, "items")
	if items == nil || items.Kind != yaml.SequenceNode {
		return false
	}
	wantKind = strings.TrimSpace(wantKind)
	for _, el := range items.Content {
		kind := yamlDocKind(el)
		if kind == wantKind {
			return true
		}
	}
	return false
}

func yamlDocKind(n *yaml.Node) string {
	n = yamlNodeAlias(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return ""
	}
	k := yamlMapValue(n, "kind")
	if k == nil {
		return ""
	}
	return strings.TrimSpace(k.Value)
}

func yamlNodeAlias(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	if n.Kind == yaml.AliasNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

func yamlMapValue(n *yaml.Node, key string) *yaml.Node {
	n = yamlNodeAlias(n)
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := yamlNodeAlias(n.Content[i])
		if k != nil && k.Value == key {
			return yamlNodeAlias(n.Content[i+1])
		}
	}
	return nil
}
