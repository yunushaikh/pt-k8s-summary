package dumpfiles

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// InventoryStatus is how an expected dump artifact appears in the report.
type InventoryStatus string

const (
	InvFound      InventoryStatus = "found"
	InvMissing    InventoryStatus = "missing"
	InvUnreadable InventoryStatus = "unreadable"
)

// InventoryRow is one expected dump artifact for the dump inventory table.
type InventoryRow struct {
	Key     string
	Label   string
	Status  InventoryStatus
	RelPath string // relative to dump root when found; empty when missing
	Note    string
}

// WarnInventoryGaps writes one stderr-style warning per missing or unreadable
// expected dump artifact so HTML generation still succeeds but gaps are obvious.
func WarnInventoryGaps(w io.Writer, rows []InventoryRow) {
	if w == nil {
		return
	}
	for _, r := range rows {
		switch r.Status {
		case InvMissing:
			note := r.Note
			if note == "" {
				note = "not part of this cluster dump"
			}
			fmt.Fprintf(w, "warning: dump file missing: %s — %s\n", r.Label, note)
		case InvUnreadable:
			note := r.Note
			if note == "" {
				note = "could not read or parse"
			}
			fmt.Fprintf(w, "warning: dump file unreadable: %s — %s\n", r.Label, note)
		}
	}
}

// BuildInventory scans the dump for expected collector exports.
// nodesNote / nodesRel describe the nodes.yaml outcome when already resolved by the caller
// (pass InvMissing and "" when absent; InvUnreadable with a note when parse failed).
func BuildInventory(dumpRoot string, nodesStatus InventoryStatus, nodesRel, nodesNote string) []InventoryRow {
	dumpRoot = filepath.Clean(dumpRoot)
	rows := make([]InventoryRow, 0, 16)

	nodes := InventoryRow{
		Key:     "nodes",
		Label:   "nodes.yaml (Kubernetes Node list)",
		Status:  nodesStatus,
		RelPath: nodesRel,
		Note:    nodesNote,
	}
	if nodes.Status == InvMissing && nodes.Note == "" {
		nodes.Note = "not part of this cluster dump"
	}
	rows = append(rows, nodes)

	rows = append(rows, checkBasename(dumpRoot, "events", "events.yaml (Kubernetes Event list)", "events.yaml", func(p string) bool {
		return listYAMLHasItemKind(p, "Event")
	}))
	rows = append(rows, checkBasename(dumpRoot, "pods", "pods.yaml (Pod list)", "pods.yaml", func(p string) bool {
		return listYAMLHasItemKind(p, "Pod")
	}))
	rows = append(rows, checkBasename(dumpRoot, "statefulsets", "statefulsets.yaml (StatefulSet list)", "statefulsets.yaml", nil))
	if _, rel, ok := FindErrorsTxt(dumpRoot); ok {
		rows = append(rows, InventoryRow{
			Key:     "errors",
			Label:   "errors.txt (collector)",
			Status:  InvFound,
			RelPath: rel,
		})
	} else {
		rows = append(rows, InventoryRow{
			Key:    "errors",
			Label:  "errors.txt (collector)",
			Status: InvMissing,
			Note:   "not part of this cluster dump",
		})
	}

	for _, spec := range []struct {
		key, label string
		target     ListTarget
	}{
		{"pxc-clusters", "PerconaXtraDBCluster list YAML", PXCClusterList},
		{"pxc-backups", "PerconaXtraDBClusterBackup list YAML", PXCBackupList},
		{"ps-clusters", "PerconaServerMySQL list YAML", PSClusterList},
		{"ps-backups", "PerconaServerMySQLBackup list YAML", PSBackupList},
		{"ps-restores", "PerconaServerMySQLRestore list YAML", PSRestoreList},
		{"pg-clusters", "PerconaPGCluster list YAML", PGClusterList},
		{"pg-backups", "PerconaPGBackup list YAML", PGBackupList},
		{"pg-restores", "PerconaPGRestore list YAML", PGRestoreList},
		{"pg-upgrades", "PerconaPGUpgrade list YAML", PGUpgradeList},
	} {
		rows = append(rows, checkListTarget(dumpRoot, spec.key, spec.label, spec.target))
	}
	return rows
}

func checkBasename(root, key, label, base string, validate func(string) bool) InventoryRow {
	paths, err := findFilesByBasename(root, base, validate)
	if err != nil {
		return InventoryRow{Key: key, Label: label, Status: InvUnreadable, Note: err.Error()}
	}
	if len(paths) == 0 {
		return InventoryRow{Key: key, Label: label, Status: InvMissing, Note: "not part of this cluster dump"}
	}
	rel := relUnder(root, paths[0])
	note := ""
	if len(paths) > 1 {
		note = fmt.Sprintf("%d copies under dump; using %s", len(paths), rel)
	}
	return InventoryRow{Key: key, Label: label, Status: InvFound, RelPath: rel, Note: note}
}

func checkListTarget(root, key, label string, target ListTarget) InventoryRow {
	paths, err := FindListYAMLFiles(root, target)
	if err != nil {
		return InventoryRow{Key: key, Label: label, Status: InvUnreadable, Note: err.Error()}
	}
	if len(paths) == 0 {
		return InventoryRow{Key: key, Label: label, Status: InvMissing, Note: "not part of this cluster dump"}
	}
	rels := make([]string, 0, len(paths))
	for _, p := range paths {
		rels = append(rels, relUnder(root, p))
	}
	note := ""
	if len(rels) > 1 {
		note = fmt.Sprintf("%d file(s)", len(rels))
	}
	return InventoryRow{
		Key:     key,
		Label:   label,
		Status:  InvFound,
		RelPath: strings.Join(rels, ", "),
		Note:    note,
	}
}

// RelPathUnder returns a slash path relative to root, or the cleaned absolute path.
func RelPathUnder(root, abs string) string {
	return relUnder(filepath.Clean(root), abs)
}
