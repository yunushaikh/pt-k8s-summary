package jpreport

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// errCertifiedListUnpublished means the release notes page loaded fine but carries no
// certified-images section at all. Percona dropped those tables from older releases
// (PXC before 1.17.0, PS before 0.11.0), so there is nothing to compare against.
var errCertifiedListUnpublished = errors.New("Percona does not publish a certified images list for this operator release, so images cannot be compared")

// CertifiedImageCache avoids repeated HTTP requests when several clusters share the same spec.crVersion.
type CertifiedImageCache struct {
	enabled bool
	store   map[string]cachedCertified
}

type cachedCertified struct {
	refs        map[string]struct{}
	url         string
	errMsg      string
	unpublished bool
}

func NewCertifiedImageCache(enabled bool) *CertifiedImageCache {
	return &CertifiedImageCache{
		enabled: enabled,
		store:   make(map[string]cachedCertified),
	}
}

// Lookup returns normalized certified image refs (lowercase repo:tag), the documentation URL
// (with #percona-certified-images), an error message if the list could not be loaded, and
// whether Percona simply does not publish a list for that release (not a tool failure).
func (c *CertifiedImageCache) Lookup(crVersionRaw string) (refs map[string]struct{}, docURL string, errMsg string, unpublished bool) {
	return c.lookupOperator("pxc", crVersionRaw, certifiedPXCLinkURL, fetchCertifiedPXCImageRefs)
}

// LookupPS is like Lookup for Percona Server for MySQL operator CRs.
func (c *CertifiedImageCache) LookupPS(crVersionRaw string) (refs map[string]struct{}, docURL string, errMsg string, unpublished bool) {
	return c.lookupOperator("ps", crVersionRaw, certifiedPSLinkURL, fetchCertifiedPSImageRefs)
}

func (c *CertifiedImageCache) lookupOperator(cacheKey, crVersionRaw string, linkFn func(string) string, fetchFn func(string) (map[string]struct{}, string, error)) (refs map[string]struct{}, docURL string, errMsg string, unpublished bool) {
	v := strings.TrimSpace(crVersionRaw)
	docURL = linkFn(v)
	if v == "" {
		return nil, docURL, "no spec.crVersion on the Custom Resource", false
	}
	if !c.enabled {
		return nil, docURL, "certified image fetch disabled (-certified-images=false)", false
	}
	key := cacheKey + ":" + v
	if ent, ok := c.store[key]; ok {
		return ent.refs, ent.url, ent.errMsg, ent.unpublished
	}
	refs, u, err := fetchFn(v)
	ent := cachedCertified{refs: refs, url: u}
	if err != nil {
		ent.errMsg = err.Error()
		ent.refs = nil
		ent.unpublished = errors.Is(err, errCertifiedListUnpublished)
	}
	c.store[key] = ent
	return ent.refs, ent.url, ent.errMsg, ent.unpublished
}

func certifiedPXCLinkURL(version string) string {
	v := sanitizeCRVersionForURL(version)
	if v == "" {
		return "https://docs.percona.com/percona-operator-for-mysql/pxc/ReleaseNotes/"
	}
	return fmt.Sprintf(
		"https://docs.percona.com/percona-operator-for-mysql/pxc/ReleaseNotes/Kubernetes-Operator-for-PXC-RN%s.html#percona-certified-images",
		v,
	)
}

func sanitizeCRVersionForURL(v string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(v) {
		if (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var htmlCertifiedImageRE = regexp.MustCompile(`(?i)\b(percona/[a-z0-9./-]+:[a-z0-9._-]+)\b`)

func certifiedPSLinkURL(version string) string {
	v := sanitizeCRVersionForURL(version)
	if v == "" {
		return "https://docs.percona.com/percona-operator-for-mysql/ps/ReleaseNotes/"
	}
	return fmt.Sprintf(
		"https://docs.percona.com/percona-operator-for-mysql/ps/ReleaseNotes/Kubernetes-Operator-for-PS-RN%s.html#percona-certified-images",
		v,
	)
}

func fetchCertifiedPSImageRefs(version string) (map[string]struct{}, string, error) {
	v := sanitizeCRVersionForURL(version)
	if v == "" {
		return nil, certifiedPSLinkURL(version), fmt.Errorf("invalid or empty crVersion for release notes URL")
	}
	base := fmt.Sprintf(
		"https://docs.percona.com/percona-operator-for-mysql/ps/ReleaseNotes/Kubernetes-Operator-for-PS-RN%s.html",
		v,
	)
	docURL := certifiedPSLinkURL(version)
	return fetchCertifiedFromReleaseNotesURL(base, docURL)
}

func fetchCertifiedPXCImageRefs(version string) (map[string]struct{}, string, error) {
	v := sanitizeCRVersionForURL(version)
	if v == "" {
		return nil, certifiedPXCLinkURL(version), fmt.Errorf("invalid or empty crVersion for release notes URL")
	}
	base := fmt.Sprintf(
		"https://docs.percona.com/percona-operator-for-mysql/pxc/ReleaseNotes/Kubernetes-Operator-for-PXC-RN%s.html",
		v,
	)
	docURL := certifiedPXCLinkURL(version)
	return fetchCertifiedFromReleaseNotesURL(base, docURL)
}

func fetchCertifiedFromReleaseNotesURL(base, docURL string) (map[string]struct{}, string, error) {
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Get(base)
	if err != nil {
		return nil, docURL, fmt.Errorf("GET %s: %w", base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, docURL, fmt.Errorf("GET %s: %s", base, resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, docURL, err
	}
	html := string(body)
	section, hasSection := certifiedSectionSuffix(html)
	if !hasSection {
		return nil, docURL, errCertifiedListUnpublished
	}
	set := make(map[string]struct{})
	for _, m := range htmlCertifiedImageRE.FindAllStringSubmatch(section, -1) {
		img := normalizeOCIImageRef(m[1])
		if img != "" {
			set[img] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil, docURL, fmt.Errorf("certified images section found but no percona/… image references could be parsed (docs layout may have changed)")
	}
	return set, docURL, nil
}

var certifiedHeadingRE = regexp.MustCompile(`(?is)<h[1-6][^>]*>\s*percona certified images`)

// certifiedSectionSuffix returns HTML from the certified-images section onward. The bool
// reports whether a real section was found: a sidebar link with the same wording (present on
// every docs page) does not count, otherwise pages without the section look like parse bugs.
func certifiedSectionSuffix(html string) (string, bool) {
	low := strings.ToLower(html)
	keys := []string{
		`id="percona-certified-images"`,
		`id='percona-certified-images'`,
		`name="percona-certified-images"`,
	}
	for _, key := range keys {
		if i := strings.Index(low, key); i >= 0 {
			return html[i:], true
		}
	}
	if loc := certifiedHeadingRE.FindStringIndex(html); loc != nil {
		return html[loc[0]:], true
	}
	return "", false
}

func normalizeOCIImageRef(img string) string {
	img = strings.TrimSpace(strings.ToLower(img))
	if img == "" {
		return ""
	}
	img = strings.TrimPrefix(img, "docker.io/")
	if i := strings.Index(img, "@"); i >= 0 {
		img = img[:i]
	}
	return strings.TrimSpace(img)
}
