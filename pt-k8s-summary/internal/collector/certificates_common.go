package collector

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type internalCertEntry struct {
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

var (
	reFileHeader  = regexp.MustCompile(`^(?i)[a-z0-9._-]+\.(?:crt|pem|cer|key)$`)
	opensslIssuer = regexp.MustCompile(`(?m)Issuer:\s*(.+)$`)
	opensslStart  = regexp.MustCompile(`(?m)Not Before:\s*(.+)$`)
	opensslEnd    = regexp.MustCompile(`(?m)Not After\s*:\s*(.+)$`)
)

var sslCertDumpSuffixes = []string{"-ssl-internal", "-ca-cert", "-ssl"}

func isSSLCertDumpFile(name string) bool {
	_, ok := clusterNameFromSSLDumpFile(name)
	return ok
}

func clusterNameFromSSLDumpFile(name string) (string, bool) {
	for _, suf := range sslCertDumpSuffixes {
		if strings.HasSuffix(name, suf) {
			return strings.TrimSuffix(name, suf), true
		}
	}
	return "", false
}

func findSSLCertDumpFiles(dumpRoot string) ([]string, error) {
	var paths []string
	err := filepath.Walk(dumpRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if isSSLCertDumpFile(info.Name()) {
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

func isOpenSSLFileHeader(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "/") || strings.Contains(s, string(filepath.Separator)) {
		return false
	}
	return reFileHeader.MatchString(s)
}

func parseOpenSSLTextCerts(dump []byte) []struct {
	Component, Issuer, NotBefore, NotAfter, SkipNote string
} {
	text := string(dump)
	lines := strings.Split(text, "\n")
	var blocks [][2]string
	var curName string
	var blockBuf strings.Builder
	flush := func() {
		if curName == "" {
			return
		}
		blocks = append(blocks, [2]string{curName, blockBuf.String()})
		blockBuf.Reset()
		curName = ""
	}
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if isOpenSSLFileHeader(t) {
			flush()
			curName = t
			blockBuf.Reset()
			continue
		}
		if curName != "" {
			blockBuf.WriteString(line)
			blockBuf.WriteString("\n")
		}
	}
	flush()

	var out []struct {
		Component, Issuer, NotBefore, NotAfter, SkipNote string
	}
	for _, pair := range blocks {
		name, body := pair[0], pair[1]
		low := strings.ToLower(name)
		if strings.HasSuffix(low, ".key") {
			out = append(out, struct {
				Component, Issuer, NotBefore, NotAfter, SkipNote string
			}{name, "", "", "", "private key (not a certificate)"})
			continue
		}
		if !strings.Contains(body, "Certificate:") {
			out = append(out, struct {
				Component, Issuer, NotBefore, NotAfter, SkipNote string
			}{name, "", "", "", "no Certificate block in dump"})
			continue
		}
		im := opensslIssuer.FindStringSubmatch(body)
		sb := opensslStart.FindStringSubmatch(body)
		ea := opensslEnd.FindStringSubmatch(body)
		iss, nb, na := "—", "—", "—"
		if len(im) > 1 {
			iss = strings.TrimSpace(im[1])
		}
		if len(sb) > 1 {
			nb = strings.TrimSpace(sb[1])
		}
		if len(ea) > 1 {
			na = strings.TrimSpace(ea[1])
		}
		if iss == "—" && nb == "—" && na == "—" {
			out = append(out, struct {
				Component, Issuer, NotBefore, NotAfter, SkipNote string
			}{name, "", "", "", "could not parse issuer / validity (unexpected format)"})
			continue
		}
		out = append(out, struct {
			Component, Issuer, NotBefore, NotAfter, SkipNote string
		}{name, iss, nb, na, ""})
	}
	return out
}

// gatherCertEntriesForClusterKeys returns TLS dump rows for clusters in clusterKeys
// (keys are namespace + "\x00" + cluster CR name).
func gatherCertEntriesForClusterKeys(dumpRoot string, clusterKeys map[string]struct{}) ([]internalCertEntry, error) {
	if len(clusterKeys) == 0 {
		return nil, nil
	}
	files, err := findSSLCertDumpFiles(dumpRoot)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	var all []internalCertEntry
	for _, fpath := range files {
		data, err := os.ReadFile(fpath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fpath, err)
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
		clusterName, _ := clusterNameFromSSLDumpFile(base)
		if _, ok := clusterKeys[ns+"\x00"+clusterName]; !ok {
			continue
		}
		entries := parseOpenSSLTextCerts(data)
		for _, e := range entries {
			row := internalCertEntry{
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
		return nil, nil
	}
	return all, nil
}

func renderCertsTable(sectionID, noteHTML string, rows []internalCertEntry) string {
	var b strings.Builder
	esc := html.EscapeString
	b.WriteString(`<style>`)
	fmt.Fprintf(&b, `#%s .pxc-cert-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }`, sectionID)
	fmt.Fprintf(&b, `#%s .pxc-cert-table { width: 100%%; border-collapse: collapse; font-size: 0.75rem; table-layout: fixed; }`, sectionID)
	fmt.Fprintf(&b, `#%s .pxc-cert-table th { text-align: left; padding: 0.4rem 0.5rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; }`, sectionID)
	fmt.Fprintf(&b, `#%s .pxc-cert-table td { padding: 0.4rem 0.5rem; border: 1px solid #e2e8f0; vertical-align: top; word-break: break-word; }`, sectionID)
	fmt.Fprintf(&b, `#%s .pxc-cert-table td.pxc-cert-mono { font-family: ui-monospace, Menlo, monospace; font-size: 0.7rem; }`, sectionID)
	fmt.Fprintf(&b, `#%s .pxc-cert-skip, #%s span.pxc-cert-skip { font-size: 0.7rem; color: #94a3b8; font-style: italic; }`, sectionID, sectionID)
	b.WriteString(`</style>`)
	b.WriteString(`<p class="pxc-cert-note">`)
	b.WriteString(noteHTML)
	b.WriteString(`</p>`)
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
