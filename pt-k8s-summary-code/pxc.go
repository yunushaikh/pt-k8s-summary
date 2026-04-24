package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const pxcFileName = "perconaxtradbclusters.pxc.percona.com.yaml"

// --- YAML shapes (subset of PerconaXtraDBCluster List items) ---

type pxcListDoc struct {
	Items []pxcClusterYAML `yaml:"items"`
}

type pxcClusterYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec   pxcSpecYAML   `yaml:"spec"`
	Status pxcStatusYAML `yaml:"status"`
}

type pxcSpecYAML struct {
	CRVersion string `yaml:"crVersion"`
	PMM       struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"pmm"`
	HAProxy  *pxcHAProxySpec  `yaml:"haproxy"`
	ProxySQL *pxcProxySQLSpec `yaml:"proxysql"`
	PXC      pxcPodSpec       `yaml:"pxc"`
}

type pxcHAProxySpec struct {
	Enabled *bool `yaml:"enabled"`
	Size    int   `yaml:"size"`
}

type pxcProxySQLSpec struct {
	Enabled *bool `yaml:"enabled"`
	Size    int   `yaml:"size"`
}

type pxcPodSpec struct {
	Size  int    `yaml:"size"`
	Image string `yaml:"image"`
}

type pxcStatusYAML struct {
	Conditions []pxcCRCondition        `yaml:"conditions"`
	HAProxy    *pxcComponentStatusYAML `yaml:"haproxy"`
	ProxySQL   *pxcComponentStatusYAML `yaml:"proxysql"`
	PXC        *pxcPXCStatusYAML       `yaml:"pxc"`
}

type pxcCRCondition struct {
	Type               string `yaml:"type"`
	Status             string `yaml:"status"`
	LastTransitionTime string `yaml:"lastTransitionTime"`
}

type pxcComponentStatusYAML struct {
	Size   *int   `yaml:"size"`
	Ready  *int   `yaml:"ready"`
	Status string `yaml:"status"`
}

type pxcPXCStatusYAML struct {
	Size    *int   `yaml:"size"`
	Ready   *int   `yaml:"ready"`
	Status  string `yaml:"status"`
	Image   string `yaml:"image"`
	Version string `yaml:"version"`
}

type pxcRowTmpl struct {
	Name            string
	Namespace       string
	CRVersion       string
	Created         string
	CondType        string
	CondStatus      string
	CondSince       string
	PMMEnabled      string
	HAProxyEnabled  bool
	ProxySQLEnabled bool
	HAProxyCell     string
	ProxySQLCell    string
	PXCCell         string
}

func findPXCYAMLs(root string) ([]string, error) {
	root = filepath.Clean(root)
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == pxcFileName {
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

func loadPXCRowsFromDump(dumpRoot string, now time.Time) ([]pxcRowTmpl, []string, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, nil, err
	}
	paths, err := findPXCYAMLs(dumpAbs)
	if err != nil {
		return nil, nil, err
	}
	var rows []pxcRowTmpl
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p, err)
		}
		var list pxcListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, nil, fmt.Errorf("%s: yaml: %w", p, err)
		}
		for i := range list.Items {
			cr := &list.Items[i]
			if strings.TrimSpace(cr.Metadata.Name) == "" {
				continue
			}
			rows = append(rows, buildPXCRowTmpl(cr, now))
		}
	}
	return rows, paths, nil
}

func buildPXCRowTmpl(cr *pxcClusterYAML, now time.Time) pxcRowTmpl {
	ct, cs, since := latestPXCCondition(cr.Status.Conditions, now)
	pmm := "no"
	if cr.Spec.PMM.Enabled {
		pmm = "yes"
	}
	hxOn := cr.Spec.HAProxy != nil && haproxySpecEnabled(cr.Spec.HAProxy)
	psOn := cr.Spec.ProxySQL != nil && proxysqlSpecEnabled(cr.Spec.ProxySQL)
	crVer := strings.TrimSpace(cr.Spec.CRVersion)
	if crVer == "" {
		crVer = "—"
	}
	return pxcRowTmpl{
		Name:            cr.Metadata.Name,
		Namespace:       cr.Metadata.Namespace,
		CRVersion:       crVer,
		Created:         cr.Metadata.CreationTimestamp,
		CondType:        ct,
		CondStatus:      cs,
		CondSince:       since,
		PMMEnabled:      pmm,
		HAProxyEnabled:  hxOn,
		ProxySQLEnabled: psOn,
		HAProxyCell:     formatHAProxy(&cr.Spec, cr.Status.HAProxy),
		ProxySQLCell:    formatProxySQL(&cr.Spec, cr.Status.ProxySQL),
		PXCCell:         formatPXC(&cr.Spec, cr.Status.PXC),
	}
}

func haproxySpecEnabled(s *pxcHAProxySpec) bool {
	if s == nil {
		return false
	}
	if s.Enabled != nil {
		return *s.Enabled
	}
	return true
}

func proxysqlSpecEnabled(s *pxcProxySQLSpec) bool {
	if s == nil {
		return false
	}
	if s.Enabled != nil {
		return *s.Enabled
	}
	return false
}

func latestPXCCondition(conds []pxcCRCondition, now time.Time) (typ, status, since string) {
	bestIdx := -1
	var bestTime time.Time
	for i := range conds {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(conds[i].LastTransitionTime))
		if err != nil {
			continue
		}
		if bestIdx < 0 || !t.Before(bestTime) {
			bestTime = t
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return "—", "—", "—"
	}
	c := conds[bestIdx]
	return c.Type, c.Status, humanizeDurationInState(bestTime, now)
}

func formatHAProxy(spec *pxcSpecYAML, st *pxcComponentStatusYAML) string {
	if spec == nil || spec.HAProxy == nil {
		return "not configured"
	}
	if !haproxySpecEnabled(spec.HAProxy) {
		return "disabled"
	}
	return briefSidecarStatus(spec.HAProxy.Size, st)
}

func formatProxySQL(spec *pxcSpecYAML, st *pxcComponentStatusYAML) string {
	if spec == nil || spec.ProxySQL == nil {
		return "not configured"
	}
	if !proxysqlSpecEnabled(spec.ProxySQL) {
		return "disabled"
	}
	return briefSidecarStatus(spec.ProxySQL.Size, st)
}

// briefSidecarStatus is a single-line summary: status word + ready counts (HAProxy / ProxySQL only).
func briefSidecarStatus(specSize int, st *pxcComponentStatusYAML) string {
	if st == nil {
		return fmt.Sprintf("size %d · pending", specSize)
	}
	sts := strings.TrimSpace(st.Status)
	if sts == "" {
		sts = "—"
	}
	if st.Ready != nil && st.Size != nil {
		return fmt.Sprintf("%s · %d/%d", sts, *st.Ready, *st.Size)
	}
	if st.Ready != nil {
		return fmt.Sprintf("%s · %d", sts, *st.Ready)
	}
	return fmt.Sprintf("%s · size %d", sts, specSize)
}

func formatPXC(spec *pxcSpecYAML, st *pxcPXCStatusYAML) string {
	if spec == nil {
		return "—"
	}
	size := spec.PXC.Size
	ver := ""
	if st != nil && strings.TrimSpace(st.Version) != "" {
		ver = strings.TrimSpace(st.Version)
	} else if tag := imageTag(spec.PXC.Image); tag != "" {
		ver = tag
	}
	verPart := ""
	if ver != "" {
		verPart = " · version: " + ver
	}
	return fmt.Sprintf("size %d (spec) · %s%s", size, formatRuntimeStatus(pxcToComponent(st)), verPart)
}

func pxcToComponent(st *pxcPXCStatusYAML) *pxcComponentStatusYAML {
	if st == nil {
		return nil
	}
	return &pxcComponentStatusYAML{Size: st.Size, Ready: st.Ready, Status: st.Status}
}

func formatRuntimeStatus(st *pxcComponentStatusYAML) string {
	if st == nil {
		return "runtime: not reported"
	}
	sts := strings.TrimSpace(st.Status)
	hasReady := st.Ready != nil
	hasSize := st.Size != nil
	if sts == "" && !hasReady && !hasSize {
		return "runtime: not reported"
	}
	if sts == "" {
		sts = "—"
	}
	readyLine := ""
	if hasReady {
		if st.Size != nil {
			readyLine = fmt.Sprintf("ready %d/%d", *st.Ready, *st.Size)
		} else {
			readyLine = fmt.Sprintf("ready %d", *st.Ready)
		}
	} else if hasSize {
		readyLine = fmt.Sprintf("status size %d", *st.Size)
	}
	if readyLine == "" {
		return fmt.Sprintf("status: %s", sts)
	}
	return fmt.Sprintf("status: %s · %s", sts, readyLine)
}

func imageTag(img string) string {
	img = strings.TrimSpace(img)
	if img == "" {
		return ""
	}
	if i := strings.LastIndex(img, "@"); i >= 0 {
		img = img[:i]
	}
	if i := strings.LastIndex(img, ":"); i >= 0 {
		return img[i+1:]
	}
	return img
}
