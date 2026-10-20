package dumpfiles

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// Caps keep the HTML report from embedding an entire large dump.
	BrowserMaxFileBytes  = 256 * 1024
	BrowserMaxFiles      = 80
	BrowserMaxTotalBytes = 6 * 1024 * 1024
)

// BrowserFile is one dump YAML (or text) artifact embedded for the in-report browser.
type BrowserFile struct {
	ID              string
	RelPath         string
	ContentEscaped  string
	Truncated       bool
	Omitted         bool
	SizeBytes       int64
	Note            string
}

// CollectBrowserFiles embeds list/export YAML found under the dump for the report browser.
func CollectBrowserFiles(dumpRoot string) []BrowserFile {
	dumpRoot = filepath.Clean(dumpRoot)
	seen := make(map[string]struct{})
	var absPaths []string
	add := func(paths []string) {
		for _, p := range paths {
			p = filepath.Clean(p)
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			absPaths = append(absPaths, p)
		}
	}

	if abs, _, err := FindNodesYAML(dumpRoot); err == nil {
		add([]string{abs})
	}
	if paths, err := FindEventsYAMLFiles(dumpRoot); err == nil {
		add(paths)
	}
	if paths, err := findFilesByBasename(dumpRoot, "pods.yaml", func(p string) bool {
		return listYAMLHasItemKind(p, "Pod")
	}); err == nil {
		add(paths)
	}
	if paths, err := findFilesByBasename(dumpRoot, "statefulsets.yaml", nil); err == nil {
		add(paths)
	}
	for _, target := range []ListTarget{
		PXCClusterList, PXCBackupList,
		PSClusterList, PSBackupList, PSRestoreList,
		PGClusterList, PGBackupList, PGRestoreList, PGUpgradeList,
	} {
		if paths, err := FindListYAMLFiles(dumpRoot, target); err == nil {
			add(paths)
		}
	}

	sort.Slice(absPaths, func(i, j int) bool {
		return relUnder(dumpRoot, absPaths[i]) < relUnder(dumpRoot, absPaths[j])
	})

	out := make([]BrowserFile, 0, len(absPaths))
	var total int64
	for i, abs := range absPaths {
		if len(out) >= BrowserMaxFiles {
			break
		}
		rel := relUnder(dumpRoot, abs)
		st, err := os.Stat(abs)
		if err != nil {
			out = append(out, BrowserFile{
				ID:      fmt.Sprintf("dump-yaml-%d", i),
				RelPath: rel,
				Omitted: true,
				Note:    html.EscapeString(err.Error()),
			})
			continue
		}
		size := st.Size()
		f := BrowserFile{
			ID:        fmt.Sprintf("dump-yaml-%d", i),
			RelPath:   rel,
			SizeBytes: size,
		}
		if size > BrowserMaxFileBytes || total+size > BrowserMaxTotalBytes {
			f.Omitted = true
			f.Note = html.EscapeString(fmt.Sprintf("too large to embed (%s); open from the dump on disk", formatByteSize(size)))
			out = append(out, f)
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			f.Omitted = true
			f.Note = html.EscapeString(err.Error())
			out = append(out, f)
			continue
		}
		body := data
		if len(body) > BrowserMaxFileBytes {
			body = append(append([]byte{}, body[:BrowserMaxFileBytes]...), []byte("\n… (truncated for report)\n")...)
			f.Truncated = true
		}
		f.ContentEscaped = html.EscapeString(string(body))
		total += int64(len(body))
		out = append(out, f)
	}
	return out
}

func formatByteSize(n int64) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.0f KiB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// BrowserTreeGroups groups RelPaths by first path segment for the browser sidebar.
func BrowserTreeGroups(files []BrowserFile) []BrowserTreeGroup {
	order := make([]string, 0)
	by := make(map[string][]BrowserFile)
	for _, f := range files {
		seg := f.RelPath
		if i := strings.Index(seg, "/"); i >= 0 {
			seg = seg[:i]
		}
		if _, ok := by[seg]; !ok {
			order = append(order, seg)
		}
		by[seg] = append(by[seg], f)
	}
	out := make([]BrowserTreeGroup, 0, len(order))
	for _, k := range order {
		out = append(out, BrowserTreeGroup{Dir: k, Files: by[k]})
	}
	return out
}

// BrowserTreeGroup is one top-level dump folder in the YAML browser tree.
type BrowserTreeGroup struct {
	Dir   string
	Files []BrowserFile
}
