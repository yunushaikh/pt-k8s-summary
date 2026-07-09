package collector

import (
	"time"

	"pt-k8s-summary/internal/jpreport"
)

// GatherPGWorkloadPodLogsForReportHTML lists PostgreSQL cluster + pgBouncer pod logs.
func GatherPGWorkloadPodLogsForReportHTML(dumpRoot, reportOutPath string, pods *jpreport.PodLoader, now time.Time) (string, error) {
	var k8s map[string]jpreport.PodK8sMeta
	if pods != nil {
		k8s = pods.K8sMetaByPod(dumpRoot, now)
	}
	return gatherPodLogsSectionHTML(dumpRoot, "", reportOutPath, k8s, pods, "pg-workload", false)
}

// GatherPGOperatorPodLogsForReportHTML lists Percona PostgreSQL operator pod logs.
func GatherPGOperatorPodLogsForReportHTML(dumpRoot, reportOutPath string, pods *jpreport.PodLoader, now time.Time) (string, error) {
	var k8s map[string]jpreport.PodK8sMeta
	if pods != nil {
		k8s = pods.K8sMetaByPod(dumpRoot, now)
	}
	return gatherPodLogsSectionHTML(dumpRoot, "", reportOutPath, k8s, pods, "pg-operator", false)
}
