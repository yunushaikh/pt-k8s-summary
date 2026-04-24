// Package collector defines pluggable report sections for pt-k8s-summary.
//
// Merge workflow (two contributors):
//   - Implement SectionCollector in a new file in this package (e.g. my_pods.go).
//   - Register your collector only in contrib_owner.go or contrib_partner.go so
//     each person touches a different file; registry.go concatenates both lists.
//
// Example implementation:
//
//	type mySection struct{}
//	func (mySection) ID() string    { return "my-pods" }
//	func (mySection) Title() string { return "Pods snapshot" }
//	func (mySection) Collect(ctx dumpctx.Context) (Section, error) {
//		b, err := ctx.ReadRel("some-namespace/pods.yaml")
//		if err != nil { return Section{}, err }
//		// parse b, build HTML string `h` (escape user content with html.EscapeString)
//		return Section{HTML: template.HTML(h)}, nil
//	}
package collector

import (
	"fmt"
	"html/template"
	"os"

	"pt-k8s-summary/internal/dumpctx"
)

// SectionCollector reads dump files and returns an HTML fragment for the report.
type SectionCollector interface {
	// ID is a stable HTML fragment id (letters, digits, hyphen); used as <section id="…">.
	ID() string
	Title() string
	Collect(ctx dumpctx.Context) (Section, error)
}

// Section is one optional block appended after the built-in report tables.
type Section struct {
	ID    string
	Title string
	HTML  template.HTML
}

// GatherSections runs all registered collectors. Errors are logged; successful sections are returned.
func GatherSections(ctx dumpctx.Context) []Section {
	var out []Section
	for _, c := range allSectionCollectors() {
		sec, err := c.Collect(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "section %q (%s): %v\n", c.ID(), c.Title(), err)
			continue
		}
		if sec.ID == "" {
			sec.ID = c.ID()
		}
		if sec.Title == "" {
			sec.Title = c.Title()
		}
		if string(sec.HTML) == "" {
			continue
		}
		out = append(out, sec)
	}
	return out
}
