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

//go:embed report_extra.html
var reportExtraHTML string

//go:embed jpreport_modals.tmpl
var jpreportModalsHTML string

//go:embed report_tail.tmpl
var reportTailTmpl string

// htmlTemplate is the full text/template source (define "condCell" in report_tail.tmpl).
var htmlTemplate = reportHeadHTML + reportPodLogsHTML + reportNodesHTML + pxcBackupHTML + reportExtraHTML + jpreportModalsHTML + reportTailTmpl
