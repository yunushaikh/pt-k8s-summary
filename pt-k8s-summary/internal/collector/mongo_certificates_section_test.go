package collector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMongoCertificatesFromOpenSSLDump(t *testing.T) {
	dir := t.TempDir()
	ns := filepath.Join(dir, "psmdb")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	cluster := `
apiVersion: v1
kind: List
items:
- apiVersion: psmdb.percona.com/v1
  kind: PerconaServerMongoDB
  metadata:
    name: my-cluster-name
    namespace: psmdb
`
	if err := os.WriteFile(filepath.Join(ns, "perconaservermongodbs.yaml"), []byte(cluster), 0o644); err != nil {
		t.Fatal(err)
	}
	ssl := `
--- Decoded tls.crt ---
Certificate:
    Issuer: CN=my-cluster-name-ca
    Validity
        Not Before: Nov  3 18:25:40 2026 GMT
        Not After : Feb  1 18:25:40 2027 GMT
--- Decoded tls.key ---
-----BEGIN PRIVATE KEY-----
AAAA
-----END PRIVATE KEY-----
`
	if err := os.WriteFile(filepath.Join(ns, "my-cluster-name-ssl"), []byte(ssl), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ns, "other-cluster-ssl"), []byte(ssl), 0o644); err != nil {
		t.Fatal(err)
	}
	certs := `
apiVersion: v1
kind: List
items:
- apiVersion: cert-manager.io/v1
  kind: Certificate
  metadata:
    name: my-cluster-name-ssl
    namespace: psmdb
  status:
    notBefore: "1999-01-01T00:00:00Z"
    notAfter: "1999-02-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(ns, "certificates.yaml"), []byte(certs), 0o644); err != nil {
		t.Fatal(err)
	}

	html, err := gatherMongoCertificateSectionHTML(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"mongo-ssl-certificates", "tls.crt", "CN=my-cluster-name-ca", "Feb  1 18:25:40 2027 GMT", "private key (not a certificate)"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(html, "other-cluster") || strings.Contains(html, "1999-02-01") {
		t.Fatalf("included a cert that is not this cluster's OpenSSL dump:\n%s", html)
	}
}

func TestMongoCertificatesFallbackToCertManager(t *testing.T) {
	dir := t.TempDir()
	ns := filepath.Join(dir, "psmdb")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	cluster := `
apiVersion: v1
kind: List
items:
- metadata:
    name: my-cluster-name
    namespace: psmdb
`
	if err := os.WriteFile(filepath.Join(ns, "perconaservermongodbs.yaml"), []byte(cluster), 0o644); err != nil {
		t.Fatal(err)
	}
	certs := `
apiVersion: v1
kind: List
items:
- metadata:
    name: my-cluster-name-ssl
    namespace: psmdb
  spec:
    issuerRef:
      name: my-cluster-name-psmdb-issuer
  status:
    notBefore: "2026-11-03T18:25:40Z"
    notAfter: "2027-02-01T18:25:40Z"
    conditions:
    - type: Ready
      status: "False"
      message: "certificate is expired"
- metadata:
    name: unrelated
    namespace: psmdb
  status:
    notAfter: "2099-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(ns, "certificates.yaml"), []byte(certs), 0o644); err != nil {
		t.Fatal(err)
	}
	html, err := gatherMongoCertificateSectionHTML(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"certificates.yaml", "my-cluster-name-psmdb-issuer", "2027-02-01T18:25:40Z", "Ready False: certificate is expired"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q\n%s", want, html)
		}
	}
	if strings.Contains(html, "2099-01-01") {
		t.Fatal("included an unrelated Certificate")
	}
}
