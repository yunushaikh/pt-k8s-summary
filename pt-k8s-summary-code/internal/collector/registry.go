package collector

func allSectionCollectors() []SectionCollector {
	var all []SectionCollector
	all = append(all, ownerSections()...)
	all = append(all, partnerSections()...)
	return all
}
