package jpreport

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type stsListDoc struct {
	Items []stsItemYAML `yaml:"items"`
}

type stsItemYAML struct {
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas *int `yaml:"replicas"`
	} `yaml:"spec"`
	Status struct {
		Replicas      int `yaml:"replicas"`
		ReadyReplicas int `yaml:"readyReplicas"`
	} `yaml:"status"`
}

// stsIndex maps "namespace\x00name" → StatefulSet replica info from dump statefulsets.yaml.
type stsIndex map[string]stsReplicaInfo

type stsReplicaInfo struct {
	SpecReplicas  int
	ReadyReplicas int
	Found         bool
}

func loadSTSIndex(dumpRoot string) (stsIndex, error) {
	paths, err := findYAMLFilesByBasename(dumpRoot, "statefulsets.yaml")
	if err != nil {
		return nil, err
	}
	idx := make(stsIndex)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var list stsListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			continue
		}
		nsHint := filepath.Base(filepath.Dir(p))
		for i := range list.Items {
			it := &list.Items[i]
			name := strings.TrimSpace(it.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(it.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			spec := 0
			if it.Spec.Replicas != nil {
				spec = *it.Spec.Replicas
			} else {
				spec = it.Status.Replicas
			}
			idx[ns+"\x00"+name] = stsReplicaInfo{
				SpecReplicas:  spec,
				ReadyReplicas: it.Status.ReadyReplicas,
				Found:         true,
			}
		}
	}
	return idx, nil
}

func (idx stsIndex) lookup(ns, name string) stsReplicaInfo {
	if idx == nil {
		return stsReplicaInfo{}
	}
	return idx[strings.TrimSpace(ns)+"\x00"+strings.TrimSpace(name)]
}

// componentSTSCheck compares a CR desired size to the matching StatefulSet replicas.
type componentSTSCheck struct {
	CRSize        string // CR spec size
	STSReplicas   string // STS spec.replicas or —
	STSReady      string // STS status.readyReplicas or —
	STSName       string
	Mismatch      bool
	STSMissing    bool
	CompareNote   string // short status for UI
}

func (idx stsIndex) checkComponent(ns, cluster, suffix string, crSize int) componentSTSCheck {
	stsName := strings.TrimSpace(cluster) + "-" + suffix
	info := idx.lookup(ns, stsName)
	out := componentSTSCheck{
		CRSize:  strconv.Itoa(crSize),
		STSName: stsName,
	}
	if !info.Found {
		out.STSReplicas = "—"
		out.STSReady = "—"
		out.STSMissing = true
		out.CompareNote = "STS not in dump"
		return out
	}
	out.STSReplicas = strconv.Itoa(info.SpecReplicas)
	out.STSReady = strconv.Itoa(info.ReadyReplicas)
	if info.SpecReplicas != crSize {
		out.Mismatch = true
		out.CompareNote = fmt.Sprintf("mismatch: CR size %d ≠ STS replicas %d", crSize, info.SpecReplicas)
	} else {
		out.CompareNote = "match"
	}
	return out
}
