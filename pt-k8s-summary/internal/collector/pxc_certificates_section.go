package collector

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"pt-k8s-summary/internal/dumpctx"

	"gopkg.in/yaml.v3"
)

// pxcCertificatesSection parses OpenSSL text dumps from PXC TLS collector files under
// &lt;namespace&gt;/&lt;cluster-name&gt;-{ca-cert,ssl,ssl-internal} for PerconaXtraDBCluster CRs only.
type pxcCertificatesSection struct{}

func (pxcCertificatesSection) ID() string           { return "pxc-ssl-certificates" }
func (pxcCertificatesSection) Title() string        { return "Certificates" }
func (pxcCertificatesSection) Group() SectionGroup  { return GroupPXC }

func (pxcCertificatesSection) Collect(ctx dumpctx.Context) (Section, error) {
	html, err := gatherPXCCertificateSectionHTML(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if html == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(html)}, nil
}

func pxcClusterKeysFromDump(dumpRoot string) (map[string]struct{}, error) {
	paths, err := findYAMLFiles(dumpRoot, pxcCRListFile)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{})
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list struct {
			Items []struct {
				Metadata struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("yaml %s: %w", p, err)
		}
		for _, item := range list.Items {
			name := strings.TrimSpace(item.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(item.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			out[ns+"\x00"+name] = struct{}{}
		}
	}
	return out, nil
}

func gatherPXCCertificateSectionHTML(dumpRoot string) (string, error) {
	keys, err := pxcClusterKeysFromDump(dumpRoot)
	if err != nil {
		return "", err
	}
	rows, err := gatherCertEntriesForClusterKeys(dumpRoot, keys)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	note := `PXC / Galera TLS material from collector files <code>&lt;namespace&gt;/&lt;cluster-name&gt;-ca-cert</code>, <code>…-ssl</code>, and <code>…-ssl-internal</code> (whichever are present in the dump). Each file contains OpenSSL <code>x509 -text</code> output. Each row is one embedded certificate (<code>ca.crt</code>, <code>tls.crt</code>, …) with <strong>issuer</strong>, start date (<strong>Not Before</strong>), and expiry (<strong>Not After</strong>).`
	return renderCertsTable("pxc-ssl-certificates", note, rows), nil
}
