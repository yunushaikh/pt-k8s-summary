package collector

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// psClusterRecord is a parsed PerconaServerMySQL item from cluster list YAML dumps.
type psClusterRecord struct {
	Name, Namespace string
	Spec            psClusterSpecYAML
	Status          psClusterStatusYAML
}

type psClusterSpecYAML struct {
	CRVersion              string `yaml:"crVersion"`
	EnableVolumeExpansion  *bool  `yaml:"enableVolumeExpansion"`
	UpgradeOptions         struct {
		Apply                  string `yaml:"apply"`
		VersionServiceEndpoint string `yaml:"versionServiceEndpoint"`
	} `yaml:"upgradeOptions"`
	Toolkit *struct {
		Image string `yaml:"image"`
	} `yaml:"toolkit"`
	MySQL struct {
		ClusterType   string `yaml:"clusterType"`
		Size          int    `yaml:"size"`
		AutoRecovery  *bool  `yaml:"autoRecovery"`
		ExposePrimary *psExposeBlockYAML `yaml:"exposePrimary"`
		Expose        *psExposeBlockYAML `yaml:"expose"`
		VolumeSpec    *psVolumeSpecYAML  `yaml:"volumeSpec"`
		Sidecars      []struct {
			Name  string `yaml:"name"`
			Image string `yaml:"image"`
		} `yaml:"sidecars"`
		SidecarPVCs []psSidecarPVCYAML `yaml:"sidecarPVCs"`
	} `yaml:"mysql"`
	Proxy struct {
		HAProxy *psProxyBlockYAML `yaml:"haproxy"`
		Router  *psProxyBlockYAML `yaml:"router"`
	} `yaml:"proxy"`
	Orchestrator *psOrchBlockYAML `yaml:"orchestrator"`
	Backup       *psBackupBlockYAML `yaml:"backup"`
}

type psExposeBlockYAML struct {
	Enabled                  *bool             `yaml:"enabled"`
	Type                     string            `yaml:"type"`
	LoadBalancerSourceRanges []string          `yaml:"loadBalancerSourceRanges"`
	ExternalTrafficPolicy    string            `yaml:"externalTrafficPolicy"`
	InternalTrafficPolicy    string            `yaml:"internalTrafficPolicy"`
	Annotations              map[string]string `yaml:"annotations"`
	Labels                   map[string]string `yaml:"labels"`
}

type psVolumeSpecYAML struct {
	PersistentVolumeClaim *struct {
		StorageClassName *string `yaml:"storageClassName"`
		Resources        struct {
			Requests struct {
				Storage string `yaml:"storage"`
			} `yaml:"requests"`
		} `yaml:"resources"`
	} `yaml:"persistentVolumeClaim"`
}

type psSidecarPVCYAML struct {
	Name string `yaml:"name"`
	Spec struct {
		StorageClassName *string `yaml:"storageClassName"`
		Resources        struct {
			Requests struct {
				Storage string `yaml:"storage"`
			} `yaml:"requests"`
		} `yaml:"resources"`
	} `yaml:"spec"`
}

type psProxyBlockYAML struct {
	Enabled *bool              `yaml:"enabled"`
	Size    int                `yaml:"size"`
	Expose  *psExposeBlockYAML `yaml:"expose"`
}

type psOrchBlockYAML struct {
	Enabled *bool              `yaml:"enabled"`
	Size    int                `yaml:"size"`
	Expose  *psExposeBlockYAML `yaml:"expose"`
}

type psBackupBlockYAML struct {
	Enabled    bool   `yaml:"enabled"`
	Image      string `yaml:"image"`
	SourcePod  string `yaml:"sourcePod"`
	BackoffLimit *int `yaml:"backoffLimit"`
	Schedule   []psBackupScheduleYAML `yaml:"schedule"`
	PITR       struct {
		Enabled      bool `yaml:"enabled"`
		BinlogServer *struct {
			Size  int    `yaml:"size"`
			Image string `yaml:"image"`
			Storage struct {
				S3 *struct {
					Bucket            string `yaml:"bucket"`
					Prefix            string `yaml:"prefix"`
					Region            string `yaml:"region"`
					EndpointURL       string `yaml:"endpointUrl"`
					CredentialsSecret string `yaml:"credentialsSecret"`
				} `yaml:"s3"`
			} `yaml:"storage"`
			ConnectTimeout     int32  `yaml:"connectTimeout"`
			ReadTimeout        int32  `yaml:"readTimeout"`
			WriteTimeout       int32  `yaml:"writeTimeout"`
			IdleTime           int32  `yaml:"idleTime"`
			ServerID           int32  `yaml:"serverId"`
			SSLMode            string `yaml:"sslMode"`
			CheckpointInterval string `yaml:"checkpointInterval"`
			CheckpointSize     string `yaml:"checkpointSize"`
			RewriteFileSize    string `yaml:"rewriteFileSize"`
			LogLevel           string `yaml:"logLevel"`
		} `yaml:"binlogServer"`
	} `yaml:"pitr"`
	Storages map[string]*psBackupStorageYAML `yaml:"storages"`
}

type psBackupScheduleYAML struct {
	Name        string `yaml:"name"`
	Schedule    string `yaml:"schedule"`
	Keep        int    `yaml:"keep"`
	StorageName string `yaml:"storageName"`
	Type        string `yaml:"type"`
}

type psBackupStorageYAML struct {
	Type      string `yaml:"type"`
	VerifyTLS *bool  `yaml:"verifyTLS"`
	S3        *struct {
		Bucket            string `yaml:"bucket"`
		Prefix            string `yaml:"prefix"`
		Region            string `yaml:"region"`
		EndpointURL       string `yaml:"endpointUrl"`
		CredentialsSecret string `yaml:"credentialsSecret"`
	} `yaml:"s3"`
	GCS *struct {
		Bucket            string `yaml:"bucket"`
		Prefix            string `yaml:"prefix"`
		EndpointURL       string `yaml:"endpointUrl"`
		CredentialsSecret string `yaml:"credentialsSecret"`
		StorageClass      string `yaml:"storageClass"`
	} `yaml:"gcs"`
	Azure *struct {
		ContainerName     string `yaml:"container"`
		Prefix            string `yaml:"prefix"`
		EndpointURL       string `yaml:"endpointUrl"`
		CredentialsSecret string `yaml:"credentialsSecret"`
		StorageClass      string `yaml:"storageClass"`
	} `yaml:"azure"`
}

type psCRCondition struct {
	Type    string `yaml:"type"`
	Status  string `yaml:"status"`
	Reason  string `yaml:"reason"`
	Message string `yaml:"message"`
}

type psClusterStatusYAML struct {
	State          string `yaml:"state"`
	Host           string `yaml:"host"`
	BackupVersion  string `yaml:"backupVersion"`
	PMMVersion     string `yaml:"pmmVersion"`
	ToolkitVersion string `yaml:"toolkitVersion"`
	MySQL          psComponentStatusYAML `yaml:"mysql"`
	HAProxy        psComponentStatusYAML `yaml:"haproxy"`
	Router         psComponentStatusYAML `yaml:"router"`
	Orchestrator   psComponentStatusYAML `yaml:"orchestrator"`
	BinlogServer   psComponentStatusYAML `yaml:"binlogServer"`
	Conditions     []psCRCondition `yaml:"conditions"`
}

type psComponentStatusYAML struct {
	Size    int32  `yaml:"size"`
	Ready   int32  `yaml:"ready"`
	State   string `yaml:"state"`
	Version string `yaml:"version"`
}

func loadPSClusters(dumpRoot string) ([]psClusterRecord, error) {
	paths, err := findPSCRListYAMLs(dumpRoot)
	if err != nil {
		return nil, err
	}
	var out []psClusterRecord
	seen := make(map[string]struct{})
	for _, p := range paths {
		nsHint := filepath.Base(filepath.Dir(p))
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var list struct {
			Items []struct {
				Metadata struct {
					Name      string `yaml:"name"`
					Namespace string `yaml:"namespace"`
				} `yaml:"metadata"`
				Spec   psClusterSpecYAML   `yaml:"spec"`
				Status psClusterStatusYAML `yaml:"status"`
			} `yaml:"items"`
		}
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("yaml %s: %w", p, err)
		}
		for _, item := range list.Items {
			name := strings.TrimSpace(item.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(item.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
			}
			key := ns + "\x00" + name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, psClusterRecord{
				Name: name, Namespace: ns,
				Spec: item.Spec, Status: item.Status,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func psBoolish(b *bool) string {
	if b == nil {
		return "—"
	}
	if *b {
		return "yes"
	}
	return "no"
}

func psProxyEnabled(p *psProxyBlockYAML) bool {
	return p != nil && p.Enabled != nil && *p.Enabled
}

func psOrchEnabled(o *psOrchBlockYAML) bool {
	return o != nil && o.Enabled != nil && *o.Enabled
}
