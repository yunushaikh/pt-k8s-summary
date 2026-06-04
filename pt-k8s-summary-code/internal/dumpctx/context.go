// Package dumpctx provides read-only access to an extracted pt-k8s-debug-collector dump tree.
// Analyzers should use Context instead of raw paths so contributors share the same helpers.
package dumpctx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Context is the root of an unpacked cluster dump (absolute path on disk).
type Context struct {
	root string
	now  time.Time
	// galeraSince, if set, is passed to pt-galera-log-explainer as --since= (RFC3339).
	galeraSince string
}

// New returns a context for reading files under rootAbs (must be absolute or cleaned).
func New(rootAbs string, now time.Time) Context {
	r, err := filepath.Abs(rootAbs)
	if err != nil {
		r = filepath.Clean(rootAbs)
	}
	return Context{root: r, now: now}
}

// WithGaleraSince returns a copy of c with Galera timeline filtering for pt-galera-log-explainer.
// Pass an empty string to clear. Value must be RFC3339 (validated by the binary before Collect).
func (c Context) WithGaleraSince(s string) Context {
	c.galeraSince = strings.TrimSpace(s)
	return c
}

// GaleraSince is the lower time bound for pt-galera-log-explainer, or "" if unused.
func (c Context) GaleraSince() string { return c.galeraSince }

// Root returns the absolute dump root directory.
func (c Context) Root() string { return c.root }

// Now is the reference time used for durations in the report (e.g. condition ages).
func (c Context) Now() time.Time { return c.now }

// Join returns an absolute path under the dump root.
func (c Context) Join(elem ...string) string {
	return filepath.Join(append([]string{c.root}, elem...)...)
}

// ReadRel reads a file relative to the dump root. rel must not escape the tree.
func (c Context) ReadRel(rel string) ([]byte, error) {
	if err := validateRel(rel); err != nil {
		return nil, err
	}
	p := filepath.Join(c.root, filepath.FromSlash(rel))
	if !underRoot(c.root, p) {
		return nil, fmt.Errorf("path escapes dump root: %s", rel)
	}
	return os.ReadFile(p)
}

func validateRel(rel string) error {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == "" {
		return fmt.Errorf("empty relative path")
	}
	for _, p := range strings.Split(rel, "/") {
		if p == ".." {
			return fmt.Errorf("invalid path segment in %q", rel)
		}
	}
	return nil
}

func underRoot(root, p string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pAbs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
