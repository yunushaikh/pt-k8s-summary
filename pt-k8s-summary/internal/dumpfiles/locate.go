package dumpfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FindNodesYAML locates the Kubernetes Node list export anywhere under the dump root.
// Collector layouts vary (e.g. nodes.yaml at dump root vs cluster-scope/nodes.yaml);
// candidates must be named nodes.yaml and contain List items with kind Node.
func FindNodesYAML(root string) (abs, rel string, err error) {
	root = filepath.Clean(root)
	paths, err := findFilesByBasename(root, "nodes.yaml", func(path string) bool {
		return listYAMLHasItemKind(path, "Node")
	})
	if err != nil {
		return "", "", err
	}
	if len(paths) == 0 {
		return "", "", fmt.Errorf("nodes.yaml (Kubernetes Node list) not found under %q", root)
	}
	sort.Slice(paths, func(i, j int) bool {
		return rankNodesYAML(relUnder(root, paths[i])) < rankNodesYAML(relUnder(root, paths[j]))
	})
	abs = paths[0]
	rel = relUnder(root, abs)
	return abs, rel, nil
}

// FindErrorsTxt locates collector errors.txt under the dump root (shallowest match wins).
func FindErrorsTxt(root string) (abs, rel string, ok bool) {
	root = filepath.Clean(root)
	paths, err := findFilesByBasename(root, "errors.txt", nil)
	if err != nil || len(paths) == 0 {
		return "", "", false
	}
	sort.Slice(paths, func(i, j int) bool {
		return pathDepth(relUnder(root, paths[i])) < pathDepth(relUnder(root, paths[j]))
	})
	abs = paths[0]
	return abs, relUnder(root, abs), true
}

// FindEventsYAMLFiles returns sorted paths to namespace events.yaml exports (kind Event).
func FindEventsYAMLFiles(root string) ([]string, error) {
	root = filepath.Clean(root)
	paths, err := findFilesByBasename(root, "events.yaml", func(path string) bool {
		return listYAMLHasItemKind(path, "Event")
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func rankNodesYAML(rel string) int {
	rel = filepath.ToSlash(rel)
	switch rel {
	case "cluster-scope/nodes.yaml":
		return 0
	case "nodes.yaml":
		return 1
	default:
		return 100 + strings.Count(rel, "/")
	}
}

func pathDepth(rel string) int {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

func relUnder(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func findFilesByBasename(root, basename string, validate func(path string) bool) ([]string, error) {
	want := strings.ToLower(strings.TrimSpace(basename))
	var out []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Base(path)) != want {
			return nil
		}
		if validate != nil && !validate(path) {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
