package main

import _ "embed"

//go:embed report_head.html
var reportHeadHTML string

//go:embed report_podlogs.html
var reportPodLogsHTML string

//go:embed report_nodes.html
var reportNodesHTML string

//go:embed pxc_backup_classic.tmpl
var pxcBackupClassicHTML string

//go:embed pxc_backup_grouped.tmpl
var pxcBackupGroupedHTML string

//go:embed ps_cluster_classic.tmpl
var psClusterClassicHTML string

//go:embed ps_cluster_grouped.tmpl
var psClusterGroupedHTML string

//go:embed pg_cluster_classic.tmpl
var pgClusterClassicHTML string

//go:embed pg_cluster_grouped.tmpl
var pgClusterGroupedHTML string

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

//go:embed report_grouped_pg.tmpl
var reportGroupedPGTmpl string

//go:embed report_grouped_tabs.js.html
var reportGroupedTabsJSHTML string

//go:embed jpreport_modals.tmpl
var jpreportModalsHTML string

//go:embed report_tail.tmpl
var reportTailTmpl string

// htmlTemplate is the classic (linear) report layout.
var htmlTemplate = reportHeadHTML + reportPodLogsHTML + reportNodesHTML + pxcBackupClassicHTML + psClusterClassicHTML + pgClusterClassicHTML + reportExtraHTML + jpreportModalsHTML + reportTailTmpl

// htmlTemplateGrouped is the tabbed layout: Kubernetes | PXC | Percona Server | PostgreSQL.
// Operator grouped defines are embedded separately so classic top-level sections are not rendered twice.
var htmlTemplateGrouped = reportHeadHTML + reportGroupedSectionsTmpl + pxcBackupGroupedHTML + psClusterGroupedHTML + pgClusterGroupedHTML + reportGroupedTabsHTML + reportNodesHTML + reportGroupedCommonTmpl + reportGroupedPXCTmpl + reportGroupedPSTmpl + reportGroupedPGTmpl + reportGroupedTabsJSHTML + jpreportModalsHTML + reportTailTmpl

func reportTemplateSource(layout string) string {
	if layout == "grouped" {
		return htmlTemplateGrouped
	}
	return htmlTemplate
}
