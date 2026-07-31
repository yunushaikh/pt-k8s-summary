package jpreport

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Docs pages always carry a sidebar link worded "Percona certified images"; only a real
// section heading means the list is published for that release.
const navOnlyPage = `<html><body>
<nav><li><a href="../images.html"><span>Percona certified images</span></a></li></nav>
<h1 id="release-1130">Operator 1.13.0</h1>
<p>Release notes without any certified images table.</p>
</body></html>`

const sectionPage = `<html><body>
<nav><li><a href="../images.html"><span>Percona certified images</span></a></li></nav>
<h2 id="percona-certified-images">Percona certified images</h2>
<table><tr><td>percona/percona-xtradb-cluster-operator:1.20.0</td></tr>
<tr><td>percona/pmm-client:3.8.0</td></tr></table>
</body></html>`

const sectionNoImagesPage = `<html><body>
<h2 id="percona-certified-images">Percona certified images</h2>
<p>See the table below.</p>
</body></html>`

func serve(t *testing.T, page string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
}

func TestCertifiedNavLinkOnlyMeansUnpublished(t *testing.T) {
	srv := serve(t, navOnlyPage)
	defer srv.Close()

	refs, _, err := fetchCertifiedFromReleaseNotesURL(srv.URL, srv.URL)
	if err == nil {
		t.Fatal("expected an error when the page has no certified-images section")
	}
	if !errors.Is(err, errCertifiedListUnpublished) {
		t.Fatalf("want unpublished sentinel, got %v", err)
	}
	if refs != nil {
		t.Fatalf("expected no refs, got %v", refs)
	}
}

func TestCertifiedSectionParsesImages(t *testing.T) {
	srv := serve(t, sectionPage)
	defer srv.Close()

	refs, _, err := fetchCertifiedFromReleaseNotesURL(srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"percona/percona-xtradb-cluster-operator:1.20.0", "percona/pmm-client:3.8.0"} {
		if _, ok := refs[want]; !ok {
			t.Fatalf("missing %q in %v", want, refs)
		}
	}
}

func TestCertifiedSectionWithoutImagesIsALayoutProblem(t *testing.T) {
	srv := serve(t, sectionNoImagesPage)
	defer srv.Close()

	_, _, err := fetchCertifiedFromReleaseNotesURL(srv.URL, srv.URL)
	if err == nil {
		t.Fatal("expected an error when the section parses to zero images")
	}
	if errors.Is(err, errCertifiedListUnpublished) {
		t.Fatalf("a present-but-unparsable section must not be reported as unpublished: %v", err)
	}
}
