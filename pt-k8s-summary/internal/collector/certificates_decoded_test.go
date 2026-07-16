package collector

import (
	"strings"
	"testing"
)

func TestParseOpenSSLTextCertsDecodedHeader(t *testing.T) {
	dump := []byte(`
--- Decoded root.crt ---
Certificate:
    Data:
        Issuer: CN = postgres-operator-ca
        Validity
            Not Before: Jul 10 08:44:34 2026 GMT
            Not After : Jul  7 09:44:34 2036 GMT
`)
	got := parseOpenSSLTextCerts(dump)
	if len(got) != 1 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	if got[0].Component != "root.crt" {
		t.Fatalf("component=%q", got[0].Component)
	}
	if !strings.Contains(got[0].Issuer, "postgres-operator-ca") {
		t.Fatalf("issuer=%q", got[0].Issuer)
	}
	if got[0].NotBefore == "—" || got[0].NotAfter == "—" {
		t.Fatalf("validity not parsed: %+v", got[0])
	}
}

func TestParseOpenSSLTextCertsBareHeaderStillWorks(t *testing.T) {
	dump := []byte(`tls.crt
Certificate:
    Issuer: O = Root CA
    Validity
        Not Before: Jul 10 09:39:20 2026 GMT
        Not After : Oct  8 09:39:20 2026 GMT
`)
	got := parseOpenSSLTextCerts(dump)
	if len(got) != 1 || got[0].Component != "tls.crt" {
		t.Fatalf("%+v", got)
	}
}
