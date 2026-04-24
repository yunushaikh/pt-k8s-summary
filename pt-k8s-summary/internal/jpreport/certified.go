package jpreport

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// CertifiedImageCache avoids repeated HTTP requests when several clusters share the same spec.crVersion.
type CertifiedImageCache struct {
	enabled bool
	store   map[string]cachedCertified
}

type cachedCertified struct {
	refs   map[string]struct{}
	url    string
	errMsg string
}

func NewCertifiedImageCache(enabled bool) *CertifiedImageCache {
	return &CertifiedImageCache{
		enabled: enabled,
		store:   make(map[string]cachedCertified),
	}
}

// Lookup returns normalized certified image refs (lowercase repo:tag), the documentation URL
// (with #percona-certified-images), and an error message if the list could not be loaded.
func (c *CertifiedImageCache) Lookup(crVersionRaw string) (refs map[string]struct{}, docURL string, errMsg string) {
	v := strings.TrimSpace(crVersionRaw)
	docURL = certifiedPXCLinkURL(v)
	if v == "" {
		return nil, docURL, "no spec.crVersion on the Custom Resource"
	}
	if !c.enabled {
		return nil, docURL, "certified image fetch disabled (-certified-images=false)"
	}
	if ent, ok := c.store[v]; ok {
		return ent.refs, ent.url, ent.errMsg
	}
	refs, u, err := fetchCertifiedPerconaImageRefs(v)
	ent := cachedCertified{refs: refs, url: u}
	if err != nil {
		ent.errMsg = err.Error()
		ent.refs = nil
	}
	c.store[v] = ent
	return ent.refs, ent.url, ent.errMsg
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

func fetchCertifiedPerconaImageRefs(version string) (map[string]struct{}, string, error) {
	v := sanitizeCRVersionForURL(version)
	if v == "" {
		return nil, certifiedPXCLinkURL(version), fmt.Errorf("invalid or empty crVersion for release notes URL")
	}
	base := fmt.Sprintf(
		"https://docs.percona.com/percona-operator-for-mysql/pxc/ReleaseNotes/Kubernetes-Operator-for-PXC-RN%s.html",
		v,
	)
	docURL := certifiedPXCLinkURL(version)

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
	section := certifiedSectionSuffix(html)
	set := make(map[string]struct{})
	for _, m := range htmlCertifiedImageRE.FindAllStringSubmatch(section, -1) {
		img := normalizeOCIImageRef(m[1])
		if img != "" {
			set[img] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil, docURL, fmt.Errorf("no percona/… image references found under certified images (docs layout may have changed)")
	}
	return set, docURL, nil
}

// certifiedSectionSuffix returns HTML from the certified-images anchor onward (or the full page).
func certifiedSectionSuffix(html string) string {
	low := strings.ToLower(html)
	keys := []string{
		`id="percona-certified-images"`,
		`id='percona-certified-images'`,
		`name="percona-certified-images"`,
	}
	for _, key := range keys {
		if i := strings.Index(low, key); i >= 0 {
			return html[i:]
		}
	}
	if i := strings.Index(low, "percona certified images"); i >= 0 {
		return html[i:]
	}
	return html
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
