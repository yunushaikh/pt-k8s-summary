package collector

import (
	"html/template"

	"pt-k8s-summary/internal/dumpctx"
)

// psCertificatesSection parses OpenSSL text dumps from TLS collector files for
// PerconaServerMySQL CRs only (&lt;namespace&gt;/&lt;cluster-name&gt;-{ca-cert,ssl,ssl-internal}).
type psCertificatesSection struct{}

func (psCertificatesSection) ID() string          { return "ps-ssl-certificates" }
func (psCertificatesSection) Title() string       { return "Certificates" }
func (psCertificatesSection) Group() SectionGroup { return GroupPS }

func (psCertificatesSection) Collect(ctx dumpctx.Context) (Section, error) {
	html, err := gatherPSCertificateSectionHTML(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if html == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(html)}, nil
}

func gatherPSCertificateSectionHTML(dumpRoot string) (string, error) {
	clusters, err := loadPSClusters(dumpRoot)
	if err != nil {
		return "", err
	}
	if len(clusters) == 0 {
		return "", nil
	}
	keys := make(map[string]struct{}, len(clusters))
	for _, c := range clusters {
		keys[c.Namespace+"\x00"+c.Name] = struct{}{}
	}
	rows, err := gatherCertEntriesForClusterKeys(dumpRoot, keys)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	note := `Percona Server for MySQL TLS material from collector files <code>&lt;namespace&gt;/&lt;cluster-name&gt;-ca-cert</code>, <code>…-ssl</code>, and <code>…-ssl-internal</code> (whichever are present in the dump). Each file contains OpenSSL <code>x509 -text</code> output. Each row is one embedded certificate (<code>ca.crt</code>, <code>tls.crt</code>, …) with <strong>issuer</strong>, start date (<strong>Not Before</strong>), and expiry (<strong>Not After</strong>).`
	return renderCertsTable("ps-ssl-certificates", note, rows), nil
}
