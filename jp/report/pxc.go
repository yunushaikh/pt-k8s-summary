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

const pxcFileName = "perconaxtradbclusters.pxc.percona.com.yaml"

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
	CRVersion       string                 `yaml:"crVersion"`
	UpdateStrategy  string                 `yaml:"updateStrategy"`
	PMM             struct{ Enabled bool } `yaml:"pmm"`
	HAProxy         *pxcHAProxySpec        `yaml:"haproxy"`
	ProxySQL        *pxcProxySQLSpec      `yaml:"proxysql"`
	PXC             pxcPodSpec             `yaml:"pxc"`
	UnsafeFlags     map[string]interface{} `yaml:"unsafeFlags"`
}

type pxcHAProxySpec struct {
	Enabled *bool  `yaml:"enabled"`
	Size    int    `yaml:"size"`
	Image   string `yaml:"image"`
}

type pxcProxySQLSpec struct {
	Enabled *bool  `yaml:"enabled"`
	Size    int    `yaml:"size"`
	Image   string `yaml:"image"`
}

type pxcPodSpec struct {
	Size          int    `yaml:"size"`
	Image         string `yaml:"image"`
	Configuration string `yaml:"configuration"`
}

type pxcStatusYAML struct {
	Conditions []pxcCRCondition         `yaml:"conditions"`
	HAProxy    *pxcComponentStatusYAML   `yaml:"haproxy"`
	ProxySQL   *pxcComponentStatusYAML   `yaml:"proxysql"`
	PXC        *pxcPXCStatusYAML         `yaml:"pxc"`
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

type imageCertRowTmpl struct {
	ImageEscaped string
	IsCertified  bool
}

type pxcRowTmpl struct {
	Name                 string
	Namespace            string
	CRVersion            string
	Created              string
	ReadyStatus      string
	ReadySince       string
	ReadyStatusClass string
	PMMEnabled           string
	UnsafeFlagsOK        bool
	UnsafeFlagsEscaped   string
	UpdateStrategy       string
	HAProxyEnabled       bool
	ProxySQLEnabled      bool
	HAProxySize          string
	HAProxyStatus        string
	HAProxyVersion       string
	ProxySQLSize         string
	ProxySQLStatus       string
	ProxySQLVersion      string
	PXCSize              string
	PXCStatus            string
	PXCVersion           string
	PXCConfigSnippet     string
	PXCConfigFullEscaped string
	PXCConfigTruncated   bool
	PXCConfigModalID     string
	HAProxyPods                []pxcPodRowTmpl
	ProxySQLPods               []pxcPodRowTmpl
	PXCPods                    []pxcPodRowTmpl
	CertifiedDocURL            string
	CertifiedFetchErrEscaped   string
	ImageCertRows              []imageCertRowTmpl
}

const pxcConfigurationMaxLines = 5

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

func loadPXCRowsFromDump(dumpRoot string, now time.Time, pods *podLoader, cert *certifiedImageCache) ([]pxcRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := findPXCYAMLs(dumpAbs)
	if err != nil {
		return nil, 0, err
	}
	var rows []pxcRowTmpl
	for _, p := range paths {
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var list pxcListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		for i := range list.Items {
			cr := &list.Items[i]
			if strings.TrimSpace(cr.Metadata.Name) == "" {
				continue
			}
			rows = append(rows, buildPXCRowTmpl(cr, now, pods, dumpAbs, cert))
		}
	}
	return rows, len(paths), nil
}

func buildPXCRowTmpl(cr *pxcClusterYAML, now time.Time, pods *podLoader, dumpRoot string, cert *certifiedImageCache) pxcRowTmpl {
	rs, since, rsClass := pxcReadyCondition(cr.Status.Conditions, now)
	pmm := "no"
	if cr.Spec.PMM.Enabled {
		pmm = "yes"
	}
	hxOn := cr.Spec.HAProxy != nil && haproxySpecEnabled(cr.Spec.HAProxy)
	psOn := cr.Spec.ProxySQL != nil && proxysqlSpecEnabled(cr.Spec.ProxySQL)
	crVerRaw := strings.TrimSpace(cr.Spec.CRVersion)
	crVer := crVerRaw
	if crVer == "" {
		crVer = "—"
	}
	row := pxcRowTmpl{
		Name:            cr.Metadata.Name,
		Namespace:       cr.Metadata.Namespace,
		CRVersion:       crVer,
		Created:          cr.Metadata.CreationTimestamp,
		ReadyStatus:      rs,
		ReadySince:       since,
		ReadyStatusClass: rsClass,
		PMMEnabled:       pmm,
		HAProxyEnabled:  hxOn,
		ProxySQLEnabled: psOn,
	}
	row.UnsafeFlagsOK, row.UnsafeFlagsEscaped = unsafeFlagsCell(cr.Spec.UnsafeFlags)
	us := strings.TrimSpace(cr.Spec.UpdateStrategy)
	if us == "" {
		us = "—"
	}
	row.UpdateStrategy = us
	if hxOn && cr.Spec.HAProxy != nil {
		row.HAProxySize, row.HAProxyStatus, row.HAProxyVersion = sidecarCols(cr.Spec.HAProxy.Size, cr.Status.HAProxy, cr.Spec.HAProxy.Image)
	}
	if psOn && cr.Spec.ProxySQL != nil {
		row.ProxySQLSize, row.ProxySQLStatus, row.ProxySQLVersion = sidecarCols(cr.Spec.ProxySQL.Size, cr.Status.ProxySQL, cr.Spec.ProxySQL.Image)
	}
	row.PXCSize, row.PXCStatus, row.PXCVersion = pxcCols(&cr.Spec.PXC, cr.Status.PXC)
	row.PXCConfigSnippet, row.PXCConfigFullEscaped, row.PXCConfigTruncated, row.PXCConfigModalID =
		formatPXCConfigurationForReport(cr.Metadata.Namespace, cr.Metadata.Name, cr.Spec.PXC.Configuration)
	ns := cr.Metadata.Namespace
	name := cr.Metadata.Name
	if pods != nil {
		if hxOn {
			row.HAProxyPods = pods.podsForPerconaComponent(ns, name, "haproxy", now, dumpRoot)
		}
		if psOn {
			row.ProxySQLPods = pods.podsForPerconaComponent(ns, name, "proxysql", now, dumpRoot)
		}
		row.PXCPods = pods.podsForPerconaComponent(ns, name, "pxc", now, dumpRoot)
	}
	if cert != nil {
		certRefs, docURL, certErr := cert.Lookup(crVerRaw)
		row.CertifiedDocURL = docURL
		row.CertifiedFetchErrEscaped = htmltemplate.HTMLEscapeString(certErr)
		listOK := certErr == "" && certRefs != nil
		var podImgs []podImageRef
		if pods != nil {
			podImgs = pods.distinctImagesForPXCInstance(ns, name)
		}
		for _, pir := range podImgs {
			_, hit := certRefs[pir.Norm]
			row.ImageCertRows = append(row.ImageCertRows, imageCertRowTmpl{
				ImageEscaped: htmltemplate.HTMLEscapeString(pir.Display),
				IsCertified:  listOK && hit,
			})
		}
	}
	return row
}

func unsafeFlagsCell(m map[string]interface{}) (ok bool, escapedList string) {
	if len(m) == 0 {
		return true, ""
	}
	var active []string
	for k, v := range m {
		if !unsafeFlagValueTrue(v) {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		active = append(active, k+": "+unsafeFlagValueString(v))
	}
	sort.Strings(active)
	if len(active) == 0 {
		return true, ""
	}
	return false, htmltemplate.HTMLEscapeString(strings.Join(active, "; "))
}

func unsafeFlagValueTrue(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "yes" || s == "1"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}

func unsafeFlagValueString(v interface{}) string {
	switch t := v.(type) {
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

// pxcReadyCondition returns the last status.condition with type "ready" (case-insensitive).
func pxcReadyCondition(conds []pxcCRCondition, now time.Time) (status, since, statusClass string) {
	status, since, statusClass = "—", "—", "status-muted"
	var ready *pxcCRCondition
	for i := range conds {
		if strings.EqualFold(strings.TrimSpace(conds[i].Type), "ready") {
			ready = &conds[i]
		}
	}
	if ready == nil {
		return
	}
	st := strings.TrimSpace(ready.Status)
	if st == "" {
		return
	}
	if strings.EqualFold(st, "true") {
		status = "True"
		statusClass = "status-true"
	} else if strings.EqualFold(st, "false") {
		status = "False"
		statusClass = "status-false"
	} else {
		status = st
		statusClass = "status-muted"
	}
	if ready.LastTransitionTime != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(ready.LastTransitionTime)); err == nil {
			since = humanizeDurationInState(t, now)
		} else {
			since = strings.TrimSpace(ready.LastTransitionTime)
		}
	}
	return
}

func humanizeDurationInState(from, to time.Time) string {
	if !to.After(from) {
		return "0s"
	}
	d := to.Sub(from)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Round(time.Second)/time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours() / 24)
	h := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, h)
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
	return true
}

func formatPXCConfigurationForReport(ns, crName, cfg string) (snippet, fullEscaped string, truncated bool, modalID string) {
	cfg = strings.TrimRight(cfg, "\n")
	lines := strings.Split(cfg, "\n")
	snippet = cfg
	truncated = false
	if len(lines) > pxcConfigurationMaxLines {
		snippet = strings.Join(lines[:pxcConfigurationMaxLines], "\n")
		truncated = true
		fullEscaped = htmltemplate.HTMLEscapeString(cfg)
	}
	modalID = "pxc-cfg-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(crName)
	return snippet, fullEscaped, truncated, modalID
}

func sanitizeModalFragment(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		return "x"
	}
	return out
}

func sidecarCols(specSize int, st *pxcComponentStatusYAML, image string) (size, status, ver string) {
	ver = imageTag(image)
	if ver == "" {
		ver = "—"
	}
	status = "—"
	if st != nil {
		if s := strings.TrimSpace(st.Status); s != "" {
			status = s
		}
	}
	size = fmt.Sprintf("%d", specSize)
	if st != nil && st.Ready != nil && st.Size != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, *st.Size)
	} else if st != nil && st.Ready != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, specSize)
	}
	return size, status, ver
}

func pxcCols(spec *pxcPodSpec, st *pxcPXCStatusYAML) (size, status, ver string) {
	if spec == nil {
		return "—", "—", "—"
	}
	ver = "—"
	if st != nil && strings.TrimSpace(st.Version) != "" {
		ver = strings.TrimSpace(st.Version)
	} else if t := imageTag(spec.Image); t != "" {
		ver = t
	}
	status = "—"
	if st != nil && strings.TrimSpace(st.Status) != "" {
		status = strings.TrimSpace(st.Status)
	}
	size = fmt.Sprintf("%d", spec.Size)
	if st != nil && st.Ready != nil && st.Size != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, *st.Size)
	} else if st != nil && st.Ready != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, spec.Size)
	}
	return size, status, ver
}

func imageTag(img string) string {
	img = strings.TrimSpace(img)
	if img == "" {
		return ""
	}
	if i := strings.LastIndex(img, "/"); i >= 0 {
		img = img[i+1:]
	}
	if i := strings.LastIndex(img, ":"); i >= 0 {
		return strings.TrimSpace(img[i+1:])
	}
	return img
}
