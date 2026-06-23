package jpreport

import (
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

type psBackupClusterYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec struct {
		ClusterName string `yaml:"clusterName"`
		StorageName string `yaml:"storageName"`
		Type        string `yaml:"type"`
	} `yaml:"spec"`
	Status psBackupStatusYAML `yaml:"status"`
}

type psBackupStatusYAML struct {
	State       string `yaml:"state"`
	Destination string `yaml:"destination"`
	Type        string `yaml:"type"`
	Storage     *struct {
		Type string `yaml:"type"`
		S3   *struct {
			Bucket string `yaml:"bucket"`
		} `yaml:"s3"`
	} `yaml:"storage"`
}

type psBackupWithTime struct {
	t            time.Time
	raw          *psBackupClusterYAML
	manifestYAML string
}

func LoadPSBackupRowsFromDump(dumpRoot string, now time.Time, pods *PodLoader) ([]BackupRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PSBackupList)
	if err != nil {
		return nil, 0, err
	}
	var pending []psBackupWithTime
	for _, p := range paths {
		data, err := os.ReadFile(p)
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
			var b psBackupClusterYAML
			if err := yaml.Unmarshal(itemBytes, &b); err != nil {
				continue
			}
			if strings.TrimSpace(b.Metadata.Name) == "" {
				continue
			}
			ts := psBackupCreationTime(&b)
			manifest := strings.TrimSuffix(string(itemBytes), "\n") + "\n"
			bCopy := b
			pending = append(pending, psBackupWithTime{t: ts, raw: &bCopy, manifestYAML: manifest})
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].t.Equal(pending[j].t) {
			return pending[i].raw.Metadata.Name > pending[j].raw.Metadata.Name
		}
		return pending[i].t.After(pending[j].t)
	})
	rows := make([]BackupRowTmpl, 0, len(pending))
	for _, x := range pending {
		rows = append(rows, buildPSBackupRowTmpl(x.raw, now, pods, dumpAbs, x.manifestYAML))
	}
	return rows, len(paths), nil
}

func psBackupCreationTime(b *psBackupClusterYAML) time.Time {
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

func buildPSBackupRowTmpl(b *psBackupClusterYAML, now time.Time, pods *PodLoader, dumpRoot, manifestYAML string) BackupRowTmpl {
	ns := strings.TrimSpace(b.Metadata.Namespace)
	name := strings.TrimSpace(b.Metadata.Name)
	cluster := strings.TrimSpace(b.Spec.ClusterName)
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
	storage := formatPSBackupStorage(b)

	age := "—"
	if b.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(b.Metadata.CreationTimestamp)); err == nil {
			age = HumanizeDurationInState(t, now)
		}
	}

	row := BackupRowTmpl{
		Name:                  name,
		Namespace:             ns,
		Cluster:               cluster,
		Storage:               storage,
		Destination:           dest,
		Status:                st,
		Age:                   age,
		BackupManifestEscaped: htmltemplate.HTMLEscapeString(manifestYAML),
		BackupManifestModalID: safePSBackupManifestStoreID(ns, name),
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

func safePSBackupManifestStoreID(ns, backupName string) string {
	var b strings.Builder
	b.WriteString("psbackupyaml-")
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

func formatPSBackupStorage(b *psBackupClusterYAML) string {
	if b == nil {
		return "—"
	}
	sn := strings.TrimSpace(b.Spec.StorageName)
	st := ""
	if b.Status.Storage != nil {
		st = strings.TrimSpace(b.Status.Storage.Type)
	}
	bt := strings.TrimSpace(b.Spec.Type)
	if bt == "" {
		bt = strings.TrimSpace(b.Status.Type)
	}
	parts := []string{}
	if sn != "" {
		parts = append(parts, sn)
	}
	if st != "" {
		parts = append(parts, "("+st+")")
	}
	if bt != "" && bt != "full" {
		parts = append(parts, bt)
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, " ")
}
