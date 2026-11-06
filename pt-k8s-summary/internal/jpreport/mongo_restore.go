package jpreport

import (
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

// MongoRestoreRowTmpl is one PerconaServerMongoDBRestore.
type MongoRestoreRowTmpl struct {
	Name               string
	Namespace          string
	Cluster            string
	Backup             string
	State              string
	StateProblem       bool
	Error              string
	Completed          string
	RestoreYAMLModalID string
	RestoreYAMLEscaped string
}

func LoadMongoRestoreRowsFromDump(dumpRoot string) ([]MongoRestoreRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PSMDBRestoreList)
	if err != nil {
		return nil, 0, err
	}
	var rows []MongoRestoreRowTmpl
	for fileIdx, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var root map[string]interface{}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		rawItems, _ := root["items"].([]interface{})
		nsHint := filepath.Base(filepath.Dir(p))
		for itemIdx, raw := range rawItems {
			item, ok := asStringKeyedMap(raw)
			if !ok {
				continue
			}
			md, _ := asStringKeyedMap(item["metadata"])
			spec, _ := asStringKeyedMap(item["spec"])
			st, _ := asStringKeyedMap(item["status"])
			name := yamlString(md["name"])
			if name == "" {
				continue
			}
			ns := yamlString(md["namespace"])
			if ns == "" {
				ns = nsHint
			}
			state := yamlString(st["state"])
			row := MongoRestoreRowTmpl{
				Name:         name,
				Namespace:    ns,
				Cluster:      orDash(yamlString(spec["clusterName"])),
				Backup:       orDash(yamlString(spec["backupName"])),
				State:        orDash(state),
				StateProblem: state != "" && !strings.EqualFold(state, "ready"),
				Error:        oneLine(yamlString(st["error"]), 240),
				Completed:    orDash(yamlString(st["completed"])),
			}
			if b, err := yaml.Marshal(item); err == nil && len(b) > 0 {
				row.RestoreYAMLModalID = "mongorestore-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(name) + "-" + strconv.Itoa(fileIdx) + "-" + strconv.Itoa(itemIdx)
				row.RestoreYAMLEscaped = htmltemplate.HTMLEscapeString(string(b))
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StateProblem != rows[j].StateProblem {
			return rows[i].StateProblem
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, len(paths), nil
}
