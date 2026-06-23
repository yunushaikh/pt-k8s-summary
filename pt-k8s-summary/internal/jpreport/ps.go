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

type psListDoc struct {
	Items []psClusterYAML `yaml:"items"`
}

type psClusterYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec   psSpecYAML   `yaml:"spec"`
	Status psStatusYAML `yaml:"status"`
}

type psSpecYAML struct {
	CRVersion      string                 `yaml:"crVersion"`
	UpdateStrategy string                 `yaml:"updateStrategy"`
	Pause          *bool                  `yaml:"pause"`
	PMM            struct{ Enabled bool } `yaml:"pmm"`
	MySQL          psMySQLSpec            `yaml:"mysql"`
	Proxy          psProxySpec            `yaml:"proxy"`
	Orchestrator   *psOrchestratorSpec    `yaml:"orchestrator"`
	UnsafeFlags    map[string]interface{} `yaml:"unsafeFlags"`
}

type psMySQLSpec struct {
	ClusterType   string `yaml:"clusterType"`
	Size          int    `yaml:"size"`
	Image         string `yaml:"image"`
	Configuration string `yaml:"configuration"`
}

type psProxySpec struct {
	HAProxy *psProxyComponentSpec `yaml:"haproxy"`
	Router  *psProxyComponentSpec `yaml:"router"`
}

type psProxyComponentSpec struct {
	Enabled *bool  `yaml:"enabled"`
	Size    int    `yaml:"size"`
	Image   string `yaml:"image"`
}

type psOrchestratorSpec struct {
	Enabled *bool  `yaml:"enabled"`
	Size    int    `yaml:"size"`
	Image   string `yaml:"image"`
}

type psStatusYAML struct {
	Conditions []pxcCRCondition        `yaml:"conditions"`
	MySQL      *psComponentStatusYAML  `yaml:"mysql"`
	HAProxy    *psComponentStatusYAML  `yaml:"haproxy"`
	Router     *psComponentStatusYAML  `yaml:"router"`
	Orchestrator *psComponentStatusYAML `yaml:"orchestrator"`
}

type psComponentStatusYAML struct {
	Size    *int   `yaml:"size"`
	Ready   *int   `yaml:"ready"`
	Status  string `yaml:"status"`
	State   string `yaml:"state"`
	Version string `yaml:"version"`
}

// PSRowTmpl is one PerconaServerMySQL cluster row for the HTML report.
type PSRowTmpl struct {
	Name                 string
	Namespace            string
	CRYAMLModalID        string
	CRYAMLEscaped        string
	CRVersion            string
	Created              string
	ReadyStatus          string
	ReadySince           string
	ReadyStatusClass     string
	PMMEnabled           string
	UnsafeFlagsOK        bool
	UnsafeFlagsEscaped   string
	UpdateStrategy       string
	HAProxyEnabled       bool
	RouterEnabled        bool
	OrchestratorEnabled  bool
	HAProxySize          string
	HAProxyStatus        string
	HAProxyVersion       string
	RouterSize           string
	RouterStatus         string
	RouterVersion        string
	OrchestratorSize     string
	OrchestratorStatus   string
	OrchestratorVersion  string
	MySQLSize            string
	MySQLStatus          string
	MySQLVersion         string
	MySQLClusterType     string
	MySQLConfigSnippet   string
	MySQLConfigFullEscaped string
	MySQLConfigTruncated bool
	MySQLConfigModalID   string
	CertifiedDocURL            string
	CertifiedFetchErrEscaped   string
	ImageCertRows              []ImageCertRowTmpl
}

const psCRYAMLModalMaxBytes = 512 * 1024

func safePSCRYAMLModalID(ns, crName string, fileIdx, itemIdx int) string {
	base := "pscryaml-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(crName)
	return base + "-f" + strconv.Itoa(fileIdx) + "-i" + strconv.Itoa(itemIdx)
}

func psCRYAMLEscapedForModal(fileBytes []byte, cr *psClusterYAML, fileIdx, itemIdx int) (escaped string, modalID string, ok bool) {
	if cr == nil {
		return "", "", false
	}
	ns := strings.TrimSpace(cr.Metadata.Namespace)
	name := strings.TrimSpace(cr.Metadata.Name)
	if name == "" {
		return "", "", false
	}
	modalID = safePSCRYAMLModalID(ns, name, fileIdx, itemIdx)
	var raw []byte
	var err error
	if b, hit := extractListItemYAMLRaw(fileBytes, ns, name); hit {
		raw = b
	} else {
		raw, err = yaml.Marshal(cr)
		if err != nil || len(raw) == 0 {
			return "", "", false
		}
	}
	trunc := false
	if len(raw) > psCRYAMLModalMaxBytes {
		raw = raw[:psCRYAMLModalMaxBytes]
		trunc = true
	}
	s := string(raw)
	if trunc {
		s += "\n\n# … truncated for report embed (see raw cluster dump for full document)"
	}
	return htmltemplate.HTMLEscapeString(s), modalID, true
}

// extractListItemYAMLRaw returns raw YAML for one List item by metadata name/namespace.
func extractListItemYAMLRaw(data []byte, wantNS, wantName string) ([]byte, bool) {
	return extractPXCCRItemYAMLRaw(data, wantNS, wantName)
}

// ListPSYAMLFiles returns paths to PerconaServerMySQL list YAML under dumpRoot.
func ListPSYAMLFiles(root string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return dumpfiles.FindListYAMLFiles(root, dumpfiles.PSClusterList)
}

func LoadPSRowsFromDump(dumpRoot string, now time.Time, pods *PodLoader, cert *CertifiedImageCache) ([]PSRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PSClusterList)
	if err != nil {
		return nil, 0, err
	}
	seen := make(map[string]struct{})
	var rows []PSRowTmpl
	for fileIdx, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var list psListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		nsHint := filepath.Base(filepath.Dir(p))
		for itemIdx := range list.Items {
			cr := &list.Items[itemIdx]
			name := strings.TrimSpace(cr.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(cr.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
				cr.Metadata.Namespace = ns
			}
			key := ns + "\x00" + name
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			row := buildPSRowTmpl(cr, now, pods, dumpAbs, cert)
			if esc, id, ok := psCRYAMLEscapedForModal(data, cr, fileIdx, itemIdx); ok {
				row.CRYAMLModalID = id
				row.CRYAMLEscaped = esc
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, len(paths), nil
}

func buildPSRowTmpl(cr *psClusterYAML, now time.Time, pods *PodLoader, dumpRoot string, cert *CertifiedImageCache) PSRowTmpl {
	rs, since, rsClass := pxcReadyCondition(cr.Status.Conditions, now)
	pmm := "no"
	if cr.Spec.PMM.Enabled {
		pmm = "yes"
	}
	hxOn := cr.Spec.Proxy.HAProxy != nil && psProxyEnabled(cr.Spec.Proxy.HAProxy)
	rtOn := cr.Spec.Proxy.Router != nil && psProxyEnabled(cr.Spec.Proxy.Router)
	orcOn := cr.Spec.Orchestrator != nil && psOrchestratorEnabled(cr.Spec.Orchestrator)
	crVerRaw := strings.TrimSpace(cr.Spec.CRVersion)
	crVer := crVerRaw
	if crVer == "" {
		crVer = "—"
	}
	row := PSRowTmpl{
		Name:                cr.Metadata.Name,
		Namespace:           cr.Metadata.Namespace,
		CRVersion:           crVer,
		Created:             cr.Metadata.CreationTimestamp,
		ReadyStatus:         rs,
		ReadySince:          since,
		ReadyStatusClass:    rsClass,
		PMMEnabled:          pmm,
		HAProxyEnabled:      hxOn,
		RouterEnabled:       rtOn,
		OrchestratorEnabled: orcOn,
	}
	row.UnsafeFlagsOK, row.UnsafeFlagsEscaped = unsafeFlagsCell(cr.Spec.UnsafeFlags)
	us := strings.TrimSpace(cr.Spec.UpdateStrategy)
	if us == "" {
		us = "—"
	}
	row.UpdateStrategy = us
	if hxOn && cr.Spec.Proxy.HAProxy != nil {
		row.HAProxySize, row.HAProxyStatus, row.HAProxyVersion = psComponentCols(cr.Spec.Proxy.HAProxy.Size, cr.Status.HAProxy, cr.Spec.Proxy.HAProxy.Image)
	}
	if rtOn && cr.Spec.Proxy.Router != nil {
		row.RouterSize, row.RouterStatus, row.RouterVersion = psComponentCols(cr.Spec.Proxy.Router.Size, cr.Status.Router, cr.Spec.Proxy.Router.Image)
	}
	if orcOn && cr.Spec.Orchestrator != nil {
		row.OrchestratorSize, row.OrchestratorStatus, row.OrchestratorVersion = psComponentCols(cr.Spec.Orchestrator.Size, cr.Status.Orchestrator, cr.Spec.Orchestrator.Image)
	}
	row.MySQLSize, row.MySQLStatus, row.MySQLVersion = psMySQLCols(&cr.Spec.MySQL, cr.Status.MySQL)
	ct := strings.TrimSpace(cr.Spec.MySQL.ClusterType)
	if ct == "" {
		ct = "—"
	}
	row.MySQLClusterType = ct
	row.MySQLConfigSnippet, row.MySQLConfigFullEscaped, row.MySQLConfigTruncated, row.MySQLConfigModalID =
		formatPSMySQLConfigurationForReport(cr.Metadata.Namespace, cr.Metadata.Name, cr.Spec.MySQL.Configuration)
	ns := cr.Metadata.Namespace
	name := cr.Metadata.Name
	if cert != nil {
		certRefs, docURL, certErr := cert.LookupPS(crVerRaw)
		row.CertifiedDocURL = docURL
		row.CertifiedFetchErrEscaped = htmltemplate.HTMLEscapeString(certErr)
		listOK := certErr == "" && certRefs != nil
		var podImgs []podImageRef
		if pods != nil {
			podImgs = pods.distinctImagesForPSInstance(ns, name)
		}
		for _, pir := range podImgs {
			_, hit := certRefs[pir.Norm]
			row.ImageCertRows = append(row.ImageCertRows, ImageCertRowTmpl{
				ImageEscaped: htmltemplate.HTMLEscapeString(pir.Display),
				IsCertified:  listOK && hit,
			})
		}
	}
	_ = dumpRoot
	return row
}

func psProxyEnabled(s *psProxyComponentSpec) bool {
	if s == nil {
		return false
	}
	if s.Enabled != nil {
		return *s.Enabled
	}
	return false
}

func psOrchestratorEnabled(s *psOrchestratorSpec) bool {
	if s == nil {
		return false
	}
	if s.Enabled != nil {
		return *s.Enabled
	}
	return false
}

func psComponentStatusText(st *psComponentStatusYAML) string {
	if st == nil {
		return ""
	}
	if s := strings.TrimSpace(st.State); s != "" {
		return s
	}
	return strings.TrimSpace(st.Status)
}

func psComponentCols(specSize int, st *psComponentStatusYAML, image string) (size, status, ver string) {
	ver = imageTag(image)
	if ver == "" {
		ver = "—"
	}
	status = "—"
	if s := psComponentStatusText(st); s != "" {
		status = s
	}
	size = fmt.Sprintf("%d", specSize)
	if st != nil && st.Ready != nil && st.Size != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, *st.Size)
	} else if st != nil && st.Ready != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, specSize)
	}
	return size, status, ver
}

func psMySQLCols(spec *psMySQLSpec, st *psComponentStatusYAML) (size, status, ver string) {
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
	if s := psComponentStatusText(st); s != "" {
		status = s
	}
	size = fmt.Sprintf("%d", spec.Size)
	if st != nil && st.Ready != nil && st.Size != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, *st.Size)
	} else if st != nil && st.Ready != nil {
		size = fmt.Sprintf("%d / %d", *st.Ready, spec.Size)
	}
	return size, status, ver
}

func formatPSMySQLConfigurationForReport(ns, crName, cfg string) (snippet, fullEscaped string, truncated bool, modalID string) {
	cfg = strings.TrimRight(cfg, "\n")
	modalID = "ps-mysql-cfg-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(crName)
	if strings.TrimSpace(cfg) == "" {
		return "—", "", false, modalID
	}
	fullEscaped = htmltemplate.HTMLEscapeString(cfg)
	lines := strings.Split(cfg, "\n")
	snippet = cfg
	truncated = false
	if len(lines) > pxcConfigurationMaxLines {
		snippet = strings.Join(lines[:pxcConfigurationMaxLines], "\n")
		truncated = true
	}
	return snippet, fullEscaped, truncated, modalID
}
