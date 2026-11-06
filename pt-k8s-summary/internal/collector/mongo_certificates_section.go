package collector

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"pt-k8s-summary/internal/dumpctx"
	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

// mongoCertificatesSection parses OpenSSL text dumps for PerconaServerMongoDB
// clusters only (<namespace>/<cluster>-{ca-cert,ssl,ssl-internal}).
type mongoCertificatesSection struct{}

func (mongoCertificatesSection) ID() string          { return "mongo-ssl-certificates" }
func (mongoCertificatesSection) Title() string       { return "Certificates" }
func (mongoCertificatesSection) Group() SectionGroup { return GroupMongo }

func (mongoCertificatesSection) Collect(ctx dumpctx.Context) (Section, error) {
	html, err := gatherMongoCertificateSectionHTML(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if html == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(html)}, nil
}

func gatherMongoCertificateSectionHTML(dumpRoot string) (string, error) {
	keys, err := mongoClusterKeysFromDump(dumpRoot)
	if err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return "", nil
	}
	rows, err := gatherCertEntriesForClusterKeys(dumpRoot, keys)
	if err != nil {
		return "", err
	}
	note := `Percona Server for MongoDB TLS material from collector files <code>&lt;namespace&gt;/&lt;cluster-name&gt;-ca-cert</code>, <code>…-ssl</code>, and <code>…-ssl-internal</code> (whichever are present in the dump). Each file contains OpenSSL <code>x509 -text</code> output. Each row is one embedded certificate (<code>ca.crt</code>, <code>tls.crt</code>, …) with <strong>issuer</strong>, start date (<strong>Not Before</strong>), and expiry (<strong>Not After</strong>). Private keys are listed only as a note.`
	if len(rows) == 0 {
		rows, err = mongoCertManagerFallback(dumpRoot, keys)
		if err != nil {
			return "", err
		}
		note = `No OpenSSL text dumps (<code>&lt;cluster&gt;-ca-cert</code>, <code>…-ssl</code>, <code>…-ssl-internal</code>) matched this cluster. Dates below come from cert-manager <code>certificates.yaml</code> (<code>status.notBefore</code>, <code>status.notAfter</code>, Ready condition).`
	}
	if len(rows) == 0 {
		return "", nil
	}
	return renderCertsTable("mongo-ssl-certificates", note, rows), nil
}

func mongoClusterKeysFromDump(dumpRoot string) (map[string]struct{}, error) {
	paths, err := dumpfiles.FindListYAMLFiles(dumpRoot, dumpfiles.PSMDBClusterList)
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

func mongoCertManagerFallback(dumpRoot string, clusterKeys map[string]struct{}) ([]internalCertEntry, error) {
	var paths []string
	err := filepath.Walk(dumpRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if info.Name() == "certificates.yaml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var rows []internalCertEntry
	for _, p := range paths {
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
				Spec struct {
					IssuerRef struct {
						Name string `yaml:"name"`
					} `yaml:"issuerRef"`
				} `yaml:"spec"`
				Status struct {
					NotBefore  string `yaml:"notBefore"`
					NotAfter   string `yaml:"notAfter"`
					Conditions []struct {
						Type    string `yaml:"type"`
						Status  string `yaml:"status"`
						Message string `yaml:"message"`
					} `yaml:"conditions"`
				} `yaml:"status"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(data, &list); err != nil {
			continue
		}
		nsHint := filepath.Base(filepath.Dir(p))
		rel, relErr := filepath.Rel(dumpRoot, p)
		if relErr != nil {
			rel = p
		}
		for _, item := range list.Items {
			name := strings.TrimSpace(item.Metadata.Name)
			cluster, ok := clusterNameFromSSLDumpFile(name)
			if !ok {
				continue
			}
			ns := strings.TrimSpace(item.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			if _, match := clusterKeys[ns+"\x00"+cluster]; !match {
				continue
			}
			ready, msg := "", ""
			for _, c := range item.Status.Conditions {
				if strings.EqualFold(c.Type, "Ready") {
					ready = c.Status
					msg = strings.TrimSpace(c.Message)
				}
			}
			note := "from certificates.yaml"
			skip := false
			if ready != "" && !strings.EqualFold(ready, "True") {
				skip = true
				if msg != "" {
					note = "Ready " + ready + ": " + msg
				} else {
					note = "Ready " + ready
				}
			} else if msg != "" {
				note = msg
			}
			rows = append(rows, internalCertEntry{
				Namespace:   ns,
				ClusterName: cluster,
				DumpFile:    filepath.ToSlash(rel),
				Component:   name,
				Issuer:      orCertDash(item.Spec.IssuerRef.Name),
				NotBefore:   orCertDash(item.Status.NotBefore),
				NotAfter:    orCertDash(item.Status.NotAfter),
				Skip:        skip,
				SkipReason:  note,
			})
		}
	}
	return rows, nil
}

func orCertDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}
