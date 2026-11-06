package jpreport

import (
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"pt-k8s-summary/internal/dumpfiles"

	"gopkg.in/yaml.v3"
)

type mongoListDoc struct {
	Items []mongoClusterYAML `yaml:"items"`
}

type mongoClusterYAML struct {
	Metadata struct {
		Name              string `yaml:"name"`
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
	Spec   mongoSpecYAML   `yaml:"spec"`
	Status mongoStatusYAML `yaml:"status"`
}

type mongoSpecYAML struct {
	CRVersion string `yaml:"crVersion"`
	Image     string `yaml:"image"`
	PMM       struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"pmm"`
	Replsets []struct {
		Name string `yaml:"name"`
		Size int    `yaml:"size"`
	} `yaml:"replsets"`
	Sharding struct {
		Enabled          bool `yaml:"enabled"`
		ConfigsvrReplSet struct {
			Size int `yaml:"size"`
		} `yaml:"configsvrReplSet"`
		Mongos struct {
			Size int `yaml:"size"`
		} `yaml:"mongos"`
	} `yaml:"sharding"`
	Backup struct {
		Enabled bool `yaml:"enabled"`
		PITR    struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"pitr"`
	} `yaml:"backup"`
	UnsafeFlags    map[string]bool `yaml:"unsafeFlags"`
	UpdateStrategy string          `yaml:"updateStrategy"`
}

type mongoStatusYAML struct {
	State        string                        `yaml:"state"`
	Ready        int                           `yaml:"ready"`
	Size         int                           `yaml:"size"`
	Host         string                        `yaml:"host"`
	Message      string                        `yaml:"message"`
	MongoVersion string                        `yaml:"mongoVersion"`
	MongoImage   string                        `yaml:"mongoImage"`
	Replsets     map[string]mongoReplsetStatus `yaml:"replsets"`
	Mongos       mongoComponentStatus          `yaml:"mongos"`
	Conditions   []pxcCRCondition              `yaml:"conditions"`
}

type mongoComponentStatus struct {
	Ready  int    `yaml:"ready"`
	Size   int    `yaml:"size"`
	Status string `yaml:"status"`
}

type mongoReplsetStatus struct {
	AddedAsShard *bool                        `yaml:"added_as_shard"`
	Initialized  bool                         `yaml:"initialized"`
	Ready        int                          `yaml:"ready"`
	Size         int                          `yaml:"size"`
	Status       string                       `yaml:"status"`
	Members      map[string]mongoMemberStatus `yaml:"members"`
}

type mongoMemberStatus struct {
	Name     string `yaml:"name"`
	State    int    `yaml:"state"`
	StateStr string `yaml:"stateStr"`
}

// MongoRowTmpl is one PerconaServerMongoDB cluster, with topology rows that
// call out missing members, size gaps, and recorded errors.
type MongoRowTmpl struct {
	Name           string
	Namespace      string
	CRYAMLModalID  string
	CRYAMLEscaped  string
	CRVersion      string
	MongoVersion   string
	Endpoint       string
	State          string
	StateProblem   bool
	ReadyCount     string
	Sharding       string
	PITR           string
	PMM            string
	Age            string
	Message        string
	Issues         []string
	Conditions     []MongoCondTmpl
	Components     []MongoComponentTmpl
	UnsafeOn       string
	UpdateStrategy string
}

// MongoCondTmpl is one status.conditions entry.
type MongoCondTmpl struct {
	Type    string
	Status  string
	Reason  string
	Message string
	Time    string
	IsError bool
}

// MongoComponentTmpl is one config server, shard, or mongos row.
type MongoComponentTmpl struct {
	Name      string
	Kind      string
	Status    string
	Ready     string
	SpecSize  string
	STSReady  string
	Members   string
	Issue     string
	IsProblem bool
}

const mongoCRYAMLModalMaxBytes = 512 * 1024

func LoadMongoRowsFromDump(dumpRoot string, now time.Time, pods *PodLoader) ([]MongoRowTmpl, int, error) {
	dumpAbs, err := filepath.Abs(dumpRoot)
	if err != nil {
		return nil, 0, err
	}
	stsIdx, _ := loadSTSIndex(dumpAbs)
	paths, err := dumpfiles.FindListYAMLFiles(dumpAbs, dumpfiles.PSMDBClusterList)
	if err != nil {
		return nil, 0, err
	}
	seen := make(map[string]struct{})
	var rows []MongoRowTmpl
	for fileIdx, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, fmt.Errorf("%s: %w", p, err)
		}
		var list mongoListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, 0, fmt.Errorf("%s: yaml: %w", p, err)
		}
		nsHint := filepath.Base(filepath.Dir(p))
		for itemIdx := range list.Items {
			cr := &list.Items[itemIdx]
			name := strings.TrimSpace(cr.Metadata.Name)
			if name == "" {
				continue
			}
			ns := strings.TrimSpace(cr.Metadata.Namespace)
			if ns == "" {
				ns = nsHint
				cr.Metadata.Namespace = ns
			}
			key := ns + "\x00" + name
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			row := buildMongoRow(cr, now, stsIdx, pods)
			if esc, id, ok := mongoCRYAMLEscaped(data, cr, fileIdx, itemIdx); ok {
				row.CRYAMLModalID = id
				row.CRYAMLEscaped = esc
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, len(paths), nil
}

func buildMongoRow(cr *mongoClusterYAML, now time.Time, stsIdx stsIndex, pods *PodLoader) MongoRowTmpl {
	age := "—"
	if cr.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(cr.Metadata.CreationTimestamp)); err == nil {
			age = HumanizeDurationInState(t, now)
		}
	}
	state := strings.TrimSpace(cr.Status.State)
	if state == "" {
		state = "—"
	}
	ver := strings.TrimSpace(cr.Status.MongoVersion)
	if ver == "" {
		ver = "—"
	}
	host := strings.TrimSpace(cr.Status.Host)
	if host == "" {
		host = "—"
	}
	shard := "no"
	if cr.Spec.Sharding.Enabled {
		shard = "yes"
	}
	pitr := "off"
	if cr.Spec.Backup.PITR.Enabled {
		pitr = "on"
	}
	pmm := "no"
	if cr.Spec.PMM.Enabled {
		pmm = "yes"
	}
	row := MongoRowTmpl{
		Name:           strings.TrimSpace(cr.Metadata.Name),
		Namespace:      strings.TrimSpace(cr.Metadata.Namespace),
		CRVersion:      orDash(strings.TrimSpace(cr.Spec.CRVersion)),
		MongoVersion:   ver,
		Endpoint:       host,
		State:          state,
		StateProblem:   state != "" && state != "—" && !strings.EqualFold(state, "ready"),
		ReadyCount:     fmt.Sprintf("%d/%d", cr.Status.Ready, cr.Status.Size),
		Sharding:       shard,
		PITR:           pitr,
		PMM:            pmm,
		Age:            age,
		Message:        strings.TrimSpace(cr.Status.Message),
		UpdateStrategy: orDash(strings.TrimSpace(cr.Spec.UpdateStrategy)),
		UnsafeOn:       mongoUnsafeOn(cr.Spec.UnsafeFlags),
	}
	if cr.Status.Ready < cr.Status.Size {
		row.Issues = append(row.Issues, fmt.Sprintf("cluster ready %d of %d", cr.Status.Ready, cr.Status.Size))
	}
	if row.Message != "" {
		row.Issues = append(row.Issues, "status message: "+oneLine(row.Message, 280))
	}
	for _, c := range cr.Status.Conditions {
		ct := MongoCondTmpl{
			Type:    orDash(c.Type),
			Status:  orDash(c.Status),
			Reason:  orDash(c.Reason),
			Message: strings.TrimSpace(c.Message),
			Time:    orDash(c.LastTransitionTime),
			IsError: strings.EqualFold(c.Type, "error") || strings.EqualFold(c.Status, "false") && strings.EqualFold(c.Type, "ready"),
		}
		if strings.EqualFold(c.Type, "error") && strings.EqualFold(c.Status, "true") {
			ct.IsError = true
			msg := ct.Message
			if msg == "" {
				msg = c.Reason
			}
			row.Issues = append(row.Issues, "error condition: "+oneLine(msg, 220))
		}
		row.Conditions = append(row.Conditions, ct)
	}
	row.Components = mongoComponents(cr, stsIdx, pods)
	for _, comp := range row.Components {
		if comp.IsProblem && comp.Issue != "" {
			row.Issues = append(row.Issues, comp.Name+": "+comp.Issue)
		}
	}
	return row
}

func mongoComponents(cr *mongoClusterYAML, stsIdx stsIndex, pods *PodLoader) []MongoComponentTmpl {
	ns := strings.TrimSpace(cr.Metadata.Namespace)
	cluster := strings.TrimSpace(cr.Metadata.Name)
	var out []MongoComponentTmpl
	if cr.Spec.Sharding.Enabled {
		out = append(out, mongoReplComponent(cr, stsIdx, pods, "cfg", "config server", cr.Spec.Sharding.ConfigsvrReplSet.Size, false))
	}
	for _, rs := range cr.Spec.Replsets {
		name := strings.TrimSpace(rs.Name)
		if name == "" {
			continue
		}
		out = append(out, mongoReplComponent(cr, stsIdx, pods, name, "shard", rs.Size, true))
	}
	if cr.Spec.Sharding.Enabled {
		st := cr.Status.Mongos
		sts := stsIdx.checkComponent(ns, cluster, "mongos", cr.Spec.Sharding.Mongos.Size)
		podNote, podBad := mongoPodReadyNote(pods, ns, cluster, "mongos", "")
		comp := MongoComponentTmpl{
			Name:     "mongos",
			Kind:     "router",
			Status:   orDash(st.Status),
			Ready:    fmt.Sprintf("%d/%d", st.Ready, st.Size),
			SpecSize: strconv.Itoa(cr.Spec.Sharding.Mongos.Size),
			STSReady: sts.STSReady,
			Members:  "—",
		}
		var issues []string
		if st.Status != "" && !strings.EqualFold(st.Status, "ready") || st.Ready < st.Size {
			issues = append(issues, fmt.Sprintf("mongos %s %d/%d", orDash(st.Status), st.Ready, st.Size))
		}
		issues = append(issues, mongoSTSIssues(sts, cr.Spec.Sharding.Mongos.Size)...)
		if podBad {
			issues = append(issues, podNote)
			comp.Members = podNote
		}
		comp.Issue = strings.Join(issues, "; ")
		comp.IsProblem = comp.Issue != ""
		out = append(out, comp)
	}
	return out
}

func mongoReplComponent(cr *mongoClusterYAML, stsIdx stsIndex, pods *PodLoader, name, kind string, specSize int, shardFlag bool) MongoComponentTmpl {
	ns := strings.TrimSpace(cr.Metadata.Namespace)
	cluster := strings.TrimSpace(cr.Metadata.Name)
	st := cr.Status.Replsets[name]
	sts := stsIdx.checkComponent(ns, cluster, name, specSize)
	members, memberIssues := formatMongoMembers(st)
	comp := MongoComponentTmpl{
		Name:     name,
		Kind:     kind,
		Status:   orDash(st.Status),
		Ready:    fmt.Sprintf("%d/%d", st.Ready, st.Size),
		SpecSize: strconv.Itoa(specSize),
		STSReady: sts.STSReady,
		Members:  members,
	}
	var issues []string
	if st.Size == 0 && specSize > 0 {
		issues = append(issues, "no status for this replset")
	}
	if st.Status != "" && !strings.EqualFold(st.Status, "ready") || (st.Size > 0 && st.Ready < st.Size) || (specSize > 0 && st.Ready < specSize) {
		issues = append(issues, fmt.Sprintf("%s %d/%d (spec %d)", orDash(st.Status), st.Ready, st.Size, specSize))
	}
	issues = append(issues, memberIssues...)
	if shardFlag && st.AddedAsShard != nil && !*st.AddedAsShard {
		issues = append(issues, "not added as shard")
	}
	issues = append(issues, mongoSTSIssues(sts, specSize)...)
	if note, bad := mongoPodReadyNote(pods, ns, cluster, mongoPodComponent(name), name); bad {
		issues = append(issues, note)
	}
	comp.Issue = strings.Join(issues, "; ")
	comp.IsProblem = comp.Issue != ""
	return comp
}

func mongoPodComponent(replset string) string {
	if replset == "cfg" {
		return "cfg"
	}
	return "mongod"
}

func formatMongoMembers(st mongoReplsetStatus) (string, []string) {
	if len(st.Members) == 0 {
		if st.Size > 0 || st.Ready > 0 {
			return "no members in status", []string{"no member roles in status (primary/secondary unknown)"}
		}
		return "no members in status", []string{"no members in status"}
	}
	names := make([]string, 0, len(st.Members))
	for k := range st.Members {
		names = append(names, k)
	}
	sort.Strings(names)
	var lines []string
	primaries := 0
	for _, n := range names {
		m := st.Members[n]
		role := strings.TrimSpace(m.StateStr)
		if role == "" {
			role = "state " + strconv.Itoa(m.State)
		}
		if strings.EqualFold(role, "PRIMARY") {
			primaries++
		}
		lines = append(lines, n+" "+role)
	}
	var issues []string
	if primaries == 0 {
		issues = append(issues, "no PRIMARY in member status")
	}
	return strings.Join(lines, "; "), issues
}

func mongoPodReadyNote(pods *PodLoader, ns, instance, component, replset string) (string, bool) {
	if pods == nil {
		return "", false
	}
	var notes []string
	bad := false
	found := 0
	for i := range pods.all {
		p := &pods.all[i]
		if strings.TrimSpace(p.Metadata.Namespace) != ns {
			continue
		}
		l := p.Metadata.Labels
		if l == nil {
			continue
		}
		if strings.TrimSpace(l["app.kubernetes.io/instance"]) != instance {
			continue
		}
		if strings.TrimSpace(l["app.kubernetes.io/component"]) != component {
			continue
		}
		if replset != "" && strings.TrimSpace(l["app.kubernetes.io/replset"]) != replset {
			continue
		}
		found++
		readyN, total := 0, len(p.Spec.Containers)
		for _, cs := range p.Status.ContainerStatuses {
			if cs.Ready {
				readyN++
			}
		}
		if total == 0 {
			total = len(p.Status.ContainerStatuses)
		}
		if readyN < total || !strings.EqualFold(p.Status.Phase, "Running") {
			bad = true
			notes = append(notes, fmt.Sprintf("%s %s containers %d/%d", p.Metadata.Name, orDash(p.Status.Phase), readyN, total))
		}
	}
	if found == 0 {
		return "no matching pods in pods.yaml", true
	}
	return strings.Join(notes, "; "), bad
}

func mongoSTSIssues(sts componentSTSCheck, specSize int) []string {
	if sts.STSMissing {
		return []string{"StatefulSet not in dump"}
	}
	var issues []string
	if sts.Mismatch {
		issues = append(issues, sts.CompareNote)
	}
	if sts.STSReady != strconv.Itoa(specSize) {
		issues = append(issues, fmt.Sprintf("StatefulSet ready %s of %d", sts.STSReady, specSize))
	}
	return issues
}

func mongoUnsafeOn(flags map[string]bool) string {
	var on []string
	for k, v := range flags {
		if v {
			on = append(on, k)
		}
	}
	sort.Strings(on)
	if len(on) == 0 {
		return ""
	}
	return strings.Join(on, ", ")
}

func mongoCRYAMLEscaped(fileBytes []byte, cr *mongoClusterYAML, fileIdx, itemIdx int) (string, string, bool) {
	if cr == nil || strings.TrimSpace(cr.Metadata.Name) == "" {
		return "", "", false
	}
	ns := strings.TrimSpace(cr.Metadata.Namespace)
	name := strings.TrimSpace(cr.Metadata.Name)
	id := "mongocryaml-" + sanitizeModalFragment(ns) + "-" + sanitizeModalFragment(name) +
		"-f" + strconv.Itoa(fileIdx) + "-i" + strconv.Itoa(itemIdx)
	raw, ok := extractListItemYAMLRaw(fileBytes, ns, name)
	if !ok {
		var err error
		raw, err = yaml.Marshal(cr)
		if err != nil || len(raw) == 0 {
			return "", "", false
		}
	}
	trunc := false
	if len(raw) > mongoCRYAMLModalMaxBytes {
		raw = raw[:mongoCRYAMLModalMaxBytes]
		trunc = true
	}
	s := string(raw)
	if trunc {
		s += "\n\n# … truncated for report embed\n"
	}
	return htmltemplate.HTMLEscapeString(s), id, true
}

func oneLine(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if max > 0 && len(s) > max {
		return s[:max] + "…"
	}
	return s
}
