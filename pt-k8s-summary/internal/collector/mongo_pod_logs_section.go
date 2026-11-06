package collector

import (
	"time"

	"pt-k8s-summary/internal/jpreport"
)

// GatherMongoWorkloadPodLogsForReportHTML lists mongod, config server, and mongos pod logs.
func GatherMongoWorkloadPodLogsForReportHTML(dumpRoot, reportOutPath string, pods *jpreport.PodLoader, now time.Time) (string, error) {
	var k8s map[string]jpreport.PodK8sMeta
	if pods != nil {
		k8s = pods.K8sMetaByPod(dumpRoot, now)
	}
	return gatherPodLogsSectionHTML(dumpRoot, "", reportOutPath, k8s, pods, "mongo-workload", false)
}

// GatherMongoOperatorPodLogsForReportHTML lists Percona Server for MongoDB operator pod logs.
func GatherMongoOperatorPodLogsForReportHTML(dumpRoot, reportOutPath string, pods *jpreport.PodLoader, now time.Time) (string, error) {
	var k8s map[string]jpreport.PodK8sMeta
	if pods != nil {
		k8s = pods.K8sMetaByPod(dumpRoot, now)
	}
	return gatherPodLogsSectionHTML(dumpRoot, "", reportOutPath, k8s, pods, "mongo-operator", false)
}
