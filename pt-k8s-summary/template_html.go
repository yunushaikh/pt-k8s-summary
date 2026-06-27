package main

import _ "embed"

//go:embed report_head.html
var reportHeadHTML string

//go:embed report_podlogs.html
var reportPodLogsHTML string

//go:embed report_nodes.html
var reportNodesHTML string

//go:embed pxc_backup_section.tmpl
var pxcBackupHTML string

//go:embed ps_cluster_section.tmpl
var psClusterHTML string

//go:embed report_extra.html
var reportExtraHTML string

//go:embed report_grouped_sections.tmpl
var reportGroupedSectionsTmpl string

//go:embed report_grouped_tabs.html
var reportGroupedTabsHTML string

//go:embed report_grouped_common.tmpl
var reportGroupedCommonTmpl string

//go:embed report_grouped_pxc.tmpl
var reportGroupedPXCTmpl string

//go:embed report_grouped_ps.tmpl
var reportGroupedPSTmpl string

//go:embed report_grouped_tabs.js.html
var reportGroupedTabsJSHTML string

//go:embed jpreport_modals.tmpl
var jpreportModalsHTML string

//go:embed report_tail.tmpl
var reportTailTmpl string

// htmlTemplate is the classic (linear) report layout.
var htmlTemplate = reportHeadHTML + reportPodLogsHTML + reportNodesHTML + pxcBackupHTML + psClusterHTML + reportExtraHTML + jpreportModalsHTML + reportTailTmpl

// htmlTemplateGrouped is the beta tabbed layout: Kubernetes | PXC | Percona Server.
var htmlTemplateGrouped = reportHeadHTML + reportGroupedSectionsTmpl + reportGroupedTabsHTML + reportNodesHTML + reportGroupedCommonTmpl + reportGroupedPXCTmpl + reportGroupedPSTmpl + pxcBackupHTML + psClusterHTML + reportGroupedTabsJSHTML + jpreportModalsHTML + reportTailTmpl

func reportTemplateSource(layout string) string {
	if layout == "grouped" {
		return htmlTemplateGrouped
	}
	return htmlTemplate
}
