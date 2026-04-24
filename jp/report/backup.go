package main

import (
	"fmt"
	htmltemplate "html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

const pxcBackupFileName = "perconaxtradbclusterbackups.pxc.percona.com.yaml"

type pxcBackupClusterYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec struct {
		PXCCluster  string `yaml:"pxcCluster"`
		StorageName string `yaml:"storageName"`
	} `yaml:"spec"`
	Status pxcBackupStatusYAML `yaml:"status"`
}

type pxcBackupStatusYAML struct {
	State         string `yaml:"state"`
	Destination   string `yaml:"destination"`
	StorageName   string `yaml:"storageName"`
	StorageType   string `yaml:"storage_type"`
	Completed     string `yaml:"completed"`
	LastScheduled string `yaml:"lastscheduled"`
}

type backupRowTmpl struct {
	Name                  string
	Namespace             string
	Cluster               string
	Storage               string
	Destination           string
	Status                string
	Age                   string
	LogPodName            string
	HasPodLog             bool
	PodLogEscaped         string
	PodLogModalID         string
	BackupManifestEscaped string
	BackupManifestModalID string
}

func findPXCBackupYAMLs(root string) ([]string, error) {
	root = filepath.Clean(root)
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == pxcBackupFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

type backupWithTime struct {
	t            time.Time
	raw          *pxcBackupClusterYAML
	manifestYAML string
}

// asStringKeyedMap normalizes YAML-decoded mapping nodes to map[string]interface{}.
func asStringKeyedMap(v interface{}) (map[string]interface{}, bool) {
	switch m := v.(type) {
	case map[string]interface{}:
		return m, true
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(m))
		for k, val := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, false
			}
			out[ks] = val
		}
		return out, true
	default:
		return nil, false
	}
}

func safeBackupManifestStoreID(ns, backupName string) string {
	var b strings.Builder
	b.WriteString("backupyaml-")
	for _, r := range ns + "-" + backupName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

func loadBackupRowsFromDump(dumpRoot string, now time.Time, pods *podLoader) ([]backupRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := findPXCBackupYAMLs(dumpAbs)
	if err != nil {
		return nil, 0, err
	}
	var pending []backupWithTime
	for _, p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var root map[string]interface{}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		rawItems, ok := root["items"].([]interface{})
		if !ok || rawItems == nil {
			continue
		}
		for _, raw := range rawItems {
			itemMap, ok := asStringKeyedMap(raw)
			if !ok {
				continue
			}
			itemBytes, err := yaml.Marshal(itemMap)
			if err != nil {
				continue
			}
			var b pxcBackupClusterYAML
			if err := yaml.Unmarshal(itemBytes, &b); err != nil {
				continue
			}
			if strings.TrimSpace(b.Metadata.Name) == "" {
				continue
			}
			ts := backupCreationTime(&b)
			manifest := strings.TrimSuffix(string(itemBytes), "\n") + "\n"
			bCopy := b
			pending = append(pending, backupWithTime{t: ts, raw: &bCopy, manifestYAML: manifest})
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].t.Equal(pending[j].t) {
			return pending[i].raw.Metadata.Name > pending[j].raw.Metadata.Name
		}
		return pending[i].t.After(pending[j].t)
	})
	rows := make([]backupRowTmpl, 0, len(pending))
	for _, x := range pending {
		rows = append(rows, buildBackupRowTmpl(x.raw, now, pods, dumpAbs, x.manifestYAML))
	}
	return rows, len(paths), nil
}

func backupCreationTime(b *pxcBackupClusterYAML) time.Time {
	if b == nil {
		return time.Time{}
	}
	s := strings.TrimSpace(b.Metadata.CreationTimestamp)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func buildBackupRowTmpl(b *pxcBackupClusterYAML, now time.Time, pods *podLoader, dumpRoot, manifestYAML string) backupRowTmpl {
	ns := strings.TrimSpace(b.Metadata.Namespace)
	name := strings.TrimSpace(b.Metadata.Name)
	cluster := strings.TrimSpace(b.Spec.PXCCluster)
	if cluster == "" {
		cluster = "—"
	}
	dest := strings.TrimSpace(b.Status.Destination)
	if dest == "" {
		dest = "—"
	}
	st := strings.TrimSpace(b.Status.State)
	if st == "" {
		st = "—"
	}
	storage := formatBackupStorage(b)

	age := "—"
	if b.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(b.Metadata.CreationTimestamp)); err == nil {
			age = humanizeDurationInState(t, now)
		}
	}

	row := backupRowTmpl{
		Name:                  name,
		Namespace:             ns,
		Cluster:               cluster,
		Storage:               storage,
		Destination:           dest,
		Status:                st,
		Age:                   age,
		BackupManifestEscaped: htmltemplate.HTMLEscapeString(manifestYAML),
		BackupManifestModalID: safeBackupManifestStoreID(ns, name),
	}
	podName := ""
	if pods != nil {
		podName = pods.podNameForBackupCR(ns, name)
	}
	row.LogPodName = podName
	if podName != "" {
		esc, has := readPodLogFromDump(dumpRoot, ns, podName)
		row.HasPodLog = has
		row.PodLogEscaped = esc
		row.PodLogModalID = safePodLogStoreID(ns, podName)
	}
	return row
}

func formatBackupStorage(b *pxcBackupClusterYAML) string {
	if b == nil {
		return "—"
	}
	sn := strings.TrimSpace(b.Status.StorageName)
	if sn == "" {
		sn = strings.TrimSpace(b.Spec.StorageName)
	}
	st := strings.TrimSpace(b.Status.StorageType)
	switch {
	case sn != "" && st != "":
		return sn + " (" + st + ")"
	case sn != "":
		return sn
	case st != "":
		return st
	default:
		return "—"
	}
}
