package collector

import "testing"

func TestClusterNameFromSSLDumpFile(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		cluster string
		ok      bool
	}{
		{"ssl-internal", "stg1-ssl-internal", "stg1", true},
		{"ca-cert", "stg1-ca-cert", "stg1", true},
		{"ssl", "stg1-ssl", "stg1", true},
		{"ssl-internal precedence", "mycluster-ssl-internal", "mycluster", true},
		{"unrelated", "nodes.yaml", "", false},
		{"partial match", "foo-ssl-extra", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := clusterNameFromSSLDumpFile(tc.file)
			if got != tc.cluster || ok != tc.ok {
				t.Fatalf("clusterNameFromSSLDumpFile(%q) = %q, %v; want %q, %v", tc.file, got, ok, tc.cluster, tc.ok)
			}
		})
	}
}

func TestIsSSLCertDumpFile(t *testing.T) {
	if !isSSLCertDumpFile("pxc-db-ssl") {
		t.Fatal("expected pxc-db-ssl to match")
	}
	if isSSLCertDumpFile("pxc-db-ssl-internal-backup") {
		t.Fatal("unexpected match for non-dump filename")
	}
}
