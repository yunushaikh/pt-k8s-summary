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

// MongoBackupRowTmpl is one PerconaServerMongoDBBackup.
type MongoBackupRowTmpl struct {
	Name              string
	Namespace         string
	Cluster           string
	Storage           string
	Type              string
	State             string
	StateProblem      bool
	Error             string
	Started           string
	Completed         string
	BackupYAMLModalID string
	BackupYAMLEscaped string
}

func LoadMongoBackupRowsFromDump(dumpRoot string, now time.Time) ([]MongoBackupRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PSMDBBackupList)
	if err != nil {
		return nil, 0, err
	}
	var rows []MongoBackupRowTmpl
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
			errText := oneLine(yamlString(st["error"]), 240)
			row := MongoBackupRowTmpl{
				Name:         name,
				Namespace:    ns,
				Cluster:      orDash(yamlString(spec["clusterName"])),
				Storage:      orDash(yamlString(st["storageName"])),
				Type:         orDash(firstNonEmpty(yamlString(st["type"]), yamlString(spec["type"]))),
				State:        orDash(state),
				StateProblem: state != "" && !strings.EqualFold(state, "ready"),
				Error:        errText,
				Started:      orDash(yamlString(st["start"])),
				Completed:    orDash(yamlString(st["completed"])),
			}
			if yamlString(row.Storage) == "—" {
				row.Storage = orDash(yamlString(spec["storageName"]))
			}
			if b, err := yaml.Marshal(item); err == nil && len(b) > 0 {
				if len(b) > 256*1024 {
					b = b[:256*1024]
				}
				row.BackupYAMLModalID = "mongobackup-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(name) + "-" + strconv.Itoa(fileIdx) + "-" + strconv.Itoa(itemIdx)
				row.BackupYAMLEscaped = htmltemplate.HTMLEscapeString(string(b))
			}
			_ = now
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

func yamlString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
