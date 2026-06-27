// Package collector defines pluggable report sections for pt-k8s-summary.
//
// Merge workflow (two contributors):
//   - Implement SectionCollector in a new file in this package (e.g. my_pods.go).
//   - Register your collector only in contrib_owner.go or contrib_partner.go so
//     each person touches a different file; registry.go concatenates both lists.
//
// Technology isolation (PXC, PS, future PG/Mongo/MariaDB/MySQL operator):
//   - One file (or small file group) per operator section, e.g. pxc_certificates_section.go
//     and ps_certificates_section.go — not a shared section that branches on operator.
//   - Shared parsing/helpers live in neutral *_common.go files (certificates_common.go).
//   - Each collector implements Group() so grouped layout routes sections to the right tab.
//   - CR loaders stay in operator-specific files (ps_cr_load.go, jpreport/pxc.go, …).
//
// Example implementation:
//
//	type mySection struct{}
//	func (mySection) ID() string    { return "my-pods" }
//	func (mySection) Title() string { return "Pods snapshot" }
//	func (mySection) Group() SectionGroup { return GroupCommon }
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

// SectionGroup tags a collector for grouped report layout (Kubernetes vs operator tabs).
type SectionGroup string

const (
	GroupCommon SectionGroup = "common"
	GroupPXC    SectionGroup = "pxc"
	GroupPS     SectionGroup = "ps"
)

// SectionCollector reads dump files and returns an HTML fragment for the report.
type SectionCollector interface {
	// ID is a stable HTML fragment id (letters, digits, hyphen); used as <section id="…">.
	ID() string
	Title() string
	Group() SectionGroup
	Collect(ctx dumpctx.Context) (Section, error)
}

// Section is one optional block appended after the built-in report tables.
type Section struct {
	ID    string
	Title string
	HTML  template.HTML
}

func collectOne(c SectionCollector, ctx dumpctx.Context) (Section, bool) {
	sec, err := c.Collect(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "section %q (%s): %v\n", c.ID(), c.Title(), err)
		return Section{}, false
	}
	if sec.ID == "" {
		sec.ID = c.ID()
	}
	if sec.Title == "" {
		sec.Title = c.Title()
	}
	if string(sec.HTML) == "" {
		return Section{}, false
	}
	return sec, true
}

// GatherSections runs all registered collectors in registration order (classic layout).
func GatherSections(ctx dumpctx.Context) []Section {
	var out []Section
	for _, c := range allSectionCollectors() {
		if sec, ok := collectOne(c, ctx); ok {
			out = append(out, sec)
		}
	}
	return out
}

// GatherSectionsByGroup runs collectors and buckets HTML by technology group.
func GatherSectionsByGroup(ctx dumpctx.Context) map[SectionGroup][]Section {
	out := map[SectionGroup][]Section{
		GroupCommon: nil,
		GroupPXC:    nil,
		GroupPS:     nil,
	}
	for _, c := range allSectionCollectors() {
		sec, ok := collectOne(c, ctx)
		if !ok {
			continue
		}
		g := c.Group()
		out[g] = append(out[g], sec)
	}
	return out
}
