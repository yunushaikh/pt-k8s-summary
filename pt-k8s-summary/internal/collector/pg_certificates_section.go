package collector

import (
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"pt-k8s-summary/internal/dumpctx"
)

type pgCertificatesSection struct{}

func (pgCertificatesSection) ID() string           { return "pg-ssl-certificates" }
func (pgCertificatesSection) Title() string        { return "Certificates" }
func (pgCertificatesSection) Group() SectionGroup  { return GroupPG }

func (pgCertificatesSection) Collect(ctx dumpctx.Context) (Section, error) {
	htmlStr, err := gatherPGCertificateSectionHTML(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if htmlStr == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(htmlStr)}, nil
}

type pgCertEntry struct {
	Namespace   string
	ClusterName string
	DumpFile    string
	Component   string
	Issuer      string
	NotBefore   string
	NotAfter    string
	Skip        bool
	SkipReason  string
}

var pgCertDumpSuffixes = []string{"-cluster-cert", "-replication-cert"}
var pgCertExactNames = []string{"pgo-root-cacert"}

func isPGCertDumpFile(name string) bool {
	for _, suf := range pgCertDumpSuffixes {
		if strings.HasSuffix(name, suf) {
			return true
		}
	}
	for _, exact := range pgCertExactNames {
		if name == exact {
			return true
		}
	}
	return false
}

func clusterNameFromPGCertFile(name string) string {
	for _, suf := range pgCertDumpSuffixes {
		if strings.HasSuffix(name, suf) {
			return strings.TrimSuffix(name, suf)
		}
	}
	if name == "pgo-root-cacert" {
		return "pgo-root-ca"
	}
	return name
}

func findPGCertDumpFiles(dumpRoot string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dumpRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if isPGCertDumpFile(info.Name()) {
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

func gatherPGCertificateSectionHTML(dumpRoot string) (string, error) {
	files, err := findPGCertDumpFiles(dumpRoot)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	var all []pgCertEntry
	for _, fpath := range files {
		data, err := os.ReadFile(fpath)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", fpath, err)
		}
		rel, err := filepath.Rel(dumpRoot, fpath)
		if err != nil {
			rel = fpath
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		ns := "—"
		if len(parts) >= 2 {
			ns = parts[0]
		}
		base := filepath.Base(fpath)
		clusterName := clusterNameFromPGCertFile(base)
		entries := parseOpenSSLTextCerts(data)
		for _, e := range entries {
			row := pgCertEntry{
				Namespace:   ns,
				ClusterName: clusterName,
				DumpFile:    rel,
				Component:   e.Component,
				Issuer:      e.Issuer,
				NotBefore:   e.NotBefore,
				NotAfter:    e.NotAfter,
			}
			if e.SkipNote != "" {
				row.Skip = true
				row.SkipReason = e.SkipNote
			}
			all = append(all, row)
		}
	}
	if len(all) == 0 {
		return "", nil
	}
	return renderPGCertsTable(all), nil
}

func renderPGCertsTable(rows []pgCertEntry) string {
	var b strings.Builder
	esc := html.EscapeString
	b.WriteString(`<style>`)
	b.WriteString(`#pg-ssl-certificates .pxc-cert-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }`)
	b.WriteString(`#pg-ssl-certificates .pxc-cert-table { width: 100%; border-collapse: collapse; font-size: 0.75rem; table-layout: fixed; }`)
	b.WriteString(`#pg-ssl-certificates .pxc-cert-table th { text-align: left; padding: 0.4rem 0.5rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; }`)
	b.WriteString(`#pg-ssl-certificates .pxc-cert-table td { padding: 0.4rem 0.5rem; border: 1px solid #e2e8f0; vertical-align: top; word-break: break-word; }`)
	b.WriteString(`#pg-ssl-certificates .pxc-cert-table td.pxc-cert-mono { font-family: ui-monospace, Menlo, monospace; font-size: 0.7rem; }`)
	b.WriteString(`#pg-ssl-certificates .pxc-cert-skip, #pg-ssl-certificates span.pxc-cert-skip { font-size: 0.7rem; color: #94a3b8; font-style: italic; }`)
	b.WriteString(`</style>`)
	b.WriteString(`<p class="pxc-cert-note">Percona PostgreSQL TLS material from collector files such as <code>&lt;namespace&gt;/&lt;cluster&gt;-cluster-cert</code> and <code>pgo-root-cacert</code>. Each row is one certificate with issuer, <strong>Not Before</strong>, and <strong>Not After</strong> parsed from OpenSSL <code>x509 -text</code> output in the dump.</p>`)
	b.WriteString(`<table class="pxc-cert-table"><thead><tr>`)
	b.WriteString(`<th scope="col">Namespace</th><th scope="col">Cluster</th><th scope="col">Dump file</th><th scope="col">cert</th>`)
	b.WriteString(`<th scope="col">Issuer</th><th scope="col">Start (Not Before)</th><th scope="col">Expiry (Not After)</th><th scope="col">Note</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		b.WriteString(`<tr><td>`)
		b.WriteString(esc(r.Namespace))
		b.WriteString(`</td><td class="pxc-cert-mono">`)
		b.WriteString(esc(r.ClusterName))
		b.WriteString(`</td><td class="pxc-cert-mono">`)
		b.WriteString(esc(r.DumpFile))
		b.WriteString(`</td><td class="pxc-cert-mono">`)
		b.WriteString(esc(r.Component))
		b.WriteString(`</td><td>`)
		if r.Skip && r.Issuer == "" {
			b.WriteString(`<span class="pxc-cert-skip">—</span>`)
		} else {
			b.WriteString(esc(r.Issuer))
		}
		b.WriteString(`</td><td class="pxc-cert-mono">`)
		if r.Skip && r.NotBefore == "" {
			b.WriteString(`<span class="pxc-cert-skip">—</span>`)
		} else {
			b.WriteString(esc(r.NotBefore))
		}
		b.WriteString(`</td><td class="pxc-cert-mono">`)
		if r.Skip && r.NotAfter == "" {
			b.WriteString(`<span class="pxc-cert-skip">—</span>`)
		} else {
			b.WriteString(esc(r.NotAfter))
		}
		b.WriteString(`</td><td class="pxc-cert-skip">`)
		if r.Skip {
			b.WriteString(esc(r.SkipReason))
		} else {
			b.WriteString(`—`)
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return b.String()
}
