package collector

// Owner / primary developer: register your SectionCollector implementations here.
// Keep this file for your work; your teammate should use contrib_partner.go only.

func ownerSections() []SectionCollector {
	return []SectionCollector{
		eventsDumpSection{},
		pvcInventorySection{},
		pxcHelmPMMPairSection{},
		pitrPXCSection{},
		pxcExtraPVCsSection{},
		pxcJemallocSection{},
		pxcExposeSection{},
		pxcReplicationChannelsSection{},
		pxcUpdateStrategySection{},
		certificatesSection{},
		pxcPodLogsSection{},
		pxcUnsafePauseRowSection{},
	}
}
