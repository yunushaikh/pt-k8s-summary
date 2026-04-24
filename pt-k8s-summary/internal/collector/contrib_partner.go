package collector

// Partner developer: register your SectionCollector implementations here.
// Prefer adding new types in separate files (e.g. partner_events.go) and only
// listing them in this file to minimize merge conflicts with contrib_owner.go.

func partnerSections() []SectionCollector {
	return []SectionCollector{
		// &partnerEventsSection{},
	}
}
