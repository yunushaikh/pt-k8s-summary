package collector

import (
	"time"

	"pt-k8s-summary/internal/jpreport"
)

// GatherPSPodLogsForReportHTML lists Percona Server for MySQL operator pod logs (no Galera block).
func GatherPSPodLogsForReportHTML(dumpRoot, reportOutPath string, pods *jpreport.PodLoader, now time.Time) (string, error) {
	var k8s map[string]jpreport.PodK8sMeta
	if pods != nil {
		k8s = pods.K8sMetaByPod(dumpRoot, now)
	}
	return gatherPodLogsSectionHTML(dumpRoot, "", reportOutPath, k8s, pods, "ps", false)
}
