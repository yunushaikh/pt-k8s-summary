package jpreport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadMongoRowsFlagsShardAndRestoreIssues(t *testing.T) {
	dir := t.TempDir()
	ns := filepath.Join(dir, "psmdb")
	if err := os.MkdirAll(ns, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ns, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("perconaservermongodbs.yaml", `
apiVersion: v1
kind: List
items:
- apiVersion: psmdb.percona.com/v1
  kind: PerconaServerMongoDB
  metadata:
    name: my-cluster-name
    namespace: psmdb
  spec:
    crVersion: 1.20.1
    updateStrategy: SmartUpdate
    unsafeFlags:
      replsetSize: true
    replsets:
    - name: rs0
      size: 1
    - name: rs1
      size: 1
    sharding:
      enabled: true
      configsvrReplSet:
        size: 1
      mongos:
        size: 1
    backup:
      enabled: true
      pitr:
        enabled: false
  status:
    state: error
    ready: 3
    size: 4
    host: my-cluster-name-mongos.psmdb.svc.cluster.local:27017
    mongoVersion: 7.0.18-11
    message: "failed to get mongos connection: connection refused"
    mongos:
      ready: 1
      size: 1
      status: ready
    replsets:
      cfg:
        status: ready
        ready: 1
        size: 1
        members:
          my-cluster-name-cfg-0:
            name: my-cluster-name-cfg-0
            stateStr: PRIMARY
      rs0:
        status: ready
        ready: 1
        size: 1
        added_as_shard: true
      rs1:
        status: initializing
        ready: 0
        size: 1
        added_as_shard: true
    conditions:
    - type: error
      status: "True"
      reason: ErrorReconcile
      message: "handle ReplicaSetNoPrimary: no mongod containers in running state"
`)
	write("perconaservermongodbbackups.yaml", `
apiVersion: v1
kind: List
items:
- metadata:
    name: backup-ok
    namespace: psmdb
  spec:
    clusterName: my-cluster-name
    storageName: minio
    type: logical
  status:
    state: ready
    storageName: minio
    type: logical
- metadata:
    name: backup-storage-unreachable
    namespace: psmdb
  spec:
    clusterName: my-cluster-name
    storageName: minio
  status:
    state: error
    storageName: minio
    type: logical
    error: "connection refused http://minio.psmdb.svc.cluster.local:9000"
`)
	write("perconaservermongodbrestores.yaml", `
apiVersion: v1
kind: List
items:
- metadata:
    name: restore-from-backup
    namespace: psmdb
  spec:
    clusterName: my-cluster-name
    backupName: backup-ok
  status:
    state: ready
`)
	write("pods.yaml", `
apiVersion: v1
kind: List
items:
- metadata:
    name: my-cluster-name-cfg-0
    namespace: psmdb
    labels:
      app.kubernetes.io/instance: my-cluster-name
      app.kubernetes.io/component: cfg
      app.kubernetes.io/replset: cfg
      app.kubernetes.io/part-of: percona-server-mongodb
  spec:
    containers:
    - name: mongod
  status:
    phase: Running
    containerStatuses:
    - name: mongod
      ready: true
- metadata:
    name: my-cluster-name-rs0-0
    namespace: psmdb
    labels:
      app.kubernetes.io/instance: my-cluster-name
      app.kubernetes.io/component: mongod
      app.kubernetes.io/replset: rs0
      app.kubernetes.io/part-of: percona-server-mongodb
  spec:
    containers:
    - name: mongod
  status:
    phase: Running
    containerStatuses:
    - name: mongod
      ready: true
- metadata:
    name: my-cluster-name-mongos-0
    namespace: psmdb
    labels:
      app.kubernetes.io/instance: my-cluster-name
      app.kubernetes.io/component: mongos
      app.kubernetes.io/part-of: percona-server-mongodb
  spec:
    containers:
    - name: mongos
  status:
    phase: Running
    containerStatuses:
    - name: mongos
      ready: false
`)
	write("statefulsets.yaml", `
apiVersion: v1
kind: List
items:
- metadata:
    name: my-cluster-name-cfg
    namespace: psmdb
  spec:
    replicas: 1
  status:
    readyReplicas: 1
- metadata:
    name: my-cluster-name-rs0
    namespace: psmdb
  spec:
    replicas: 1
  status:
    readyReplicas: 1
- metadata:
    name: my-cluster-name-rs1
    namespace: psmdb
  spec:
    replicas: 1
- metadata:
    name: my-cluster-name-mongos
    namespace: psmdb
  spec:
    replicas: 1
  status:
    readyReplicas: 0
`)

	pods, err := LoadPodLoader(dir)
	if err != nil {
		t.Fatal(err)
	}
	rows, n, err := LoadMongoRowsFromDump(dir, time.Now(), pods)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(rows) != 1 {
		t.Fatalf("clusters: files=%d rows=%d", n, len(rows))
	}
	row := rows[0]
	if !row.StateProblem {
		t.Fatal("expected cluster state error")
	}
	joined := strings.Join(row.Issues, "\n")
	for _, want := range []string{
		"cluster ready 3 of 4",
		"failed to get mongos connection",
		"handle ReplicaSetNoPrimary",
		"rs1:",
		"no matching pods",
		"primary/secondary unknown",
		"rs0:",
		"no member roles",
		"cfg-0 PRIMARY",
		"mongos:",
		"containers 0/1",
	} {
		if !strings.Contains(joined, want) && !componentHas(row, want) {
			t.Errorf("missing %q in issues:\n%s", want, joined)
		}
	}
	cfg := componentByName(t, row, "cfg")
	if !strings.Contains(cfg.Members, "PRIMARY") || cfg.IsProblem {
		t.Fatalf("cfg should be a healthy PRIMARY, got %+v", cfg)
	}

	backups, _, err := LoadMongoBackupRowsFromDump(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 2 || backups[0].Name != "backup-storage-unreachable" || !backups[0].StateProblem {
		t.Fatalf("backups: %+v", backups)
	}
	if !strings.Contains(backups[0].Error, "connection refused") {
		t.Fatalf("backup error: %q", backups[0].Error)
	}
	restores, _, err := LoadMongoRestoreRowsFromDump(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(restores) != 1 || restores[0].StateProblem || restores[0].Backup != "backup-ok" {
		t.Fatalf("restores: %+v", restores)
	}
}

func TestFormatMongoMembersRoles(t *testing.T) {
	line, issues := formatMongoMembers(mongoReplsetStatus{
		Size: 2,
		Members: map[string]mongoMemberStatus{
			"rs0-0": {StateStr: "SECONDARY"},
			"rs0-1": {StateStr: "SECONDARY"},
		},
	})
	if !strings.Contains(line, "rs0-0 SECONDARY") || !strings.Contains(strings.Join(issues, " "), "no PRIMARY") {
		t.Fatalf("line=%q issues=%v", line, issues)
	}
}

func componentByName(t *testing.T, row MongoRowTmpl, name string) MongoComponentTmpl {
	t.Helper()
	for _, c := range row.Components {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("missing component %s", name)
	return MongoComponentTmpl{}
}

func componentHas(row MongoRowTmpl, frag string) bool {
	for _, c := range row.Components {
		if strings.Contains(c.Members, frag) || strings.Contains(c.Issue, frag) || strings.Contains(c.Name, frag) {
			return true
		}
	}
	return false
}
