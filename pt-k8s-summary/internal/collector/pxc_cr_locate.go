package collector

import "pt-k8s-summary/internal/dumpfiles"

// findPXCClusterListYAML locates PerconaXtraDBCluster list exports (legacy long
// basename and short perconaxtradbclusters.yaml from newer collectors).
func findPXCClusterListYAML(dumpRoot string) ([]string, error) {
	return dumpfiles.FindListYAMLFiles(dumpRoot, dumpfiles.PXCClusterList)
}
