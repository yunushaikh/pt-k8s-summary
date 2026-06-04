package jpreport

import (
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

const podsFileName = "pods.yaml"

type podResourceList struct {
	CPU    interface{} `yaml:"cpu"`
	Memory interface{} `yaml:"memory"`
}

type podResources struct {
	Requests *podResourceList `yaml:"requests"`
	Limits   *podResourceList `yaml:"limits"`
}

type podContainerSpec struct {
	Name      string        `yaml:"name"`
	Image     string        `yaml:"image"`
	Resources *podResources `yaml:"resources"`
}

type podListDoc struct {
	Items []podItem `yaml:"items"`
}

type podItem struct {
	Metadata struct {
		Name              string            `yaml:"name"`
		Namespace         string            `yaml:"namespace"`
		CreationTimestamp string            `yaml:"creationTimestamp"`
		Labels            map[string]string `yaml:"labels"`
		Annotations       map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
	Spec struct {
		NodeName       string             `yaml:"nodeName"`
		InitContainers []podContainerSpec `yaml:"initContainers"`
		Containers     []podContainerSpec `yaml:"containers"`
	} `yaml:"spec"`
	Status struct {
		Phase                 string               `yaml:"phase"`
		PodIP                 string               `yaml:"podIP"`
		ContainerStatuses     []podContainerStatus `yaml:"containerStatuses"`
		InitContainerStatuses []podContainerStatus `yaml:"initContainerStatuses"`
	} `yaml:"status"`
}

type podContainerStatus struct {
	Name         string `yaml:"name"`
	Ready        bool   `yaml:"ready"`
	RestartCount int    `yaml:"restartCount"`
}

type PXCPodRowTmpl struct {
	Name             string
	Ready            string
	Status           string
	Restarts         string
	Age              string
	PodIP            string
	Node             string
	HasPodLog        bool
	PodLogEscaped    string
	PodLogModalID    string
	PodSpecModalID   string
	PodSpecEscaped   string
}

type PodLoader struct {
	all []podItem
}

func findPodsYAMLs(root string) ([]string, error) {
	root = filepath.Clean(root)
	var paths []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == podsFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func LoadPodLoader(root string) (*PodLoader, error) {
	paths, err := findPodsYAMLs(root)
	if err != nil {
		return nil, err
	}
	var merged []podItem
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		var list podListDoc
		if err := yaml.Unmarshal(data, &list); err != nil {
			return nil, fmt.Errorf("%s: yaml: %w", p, err)
		}
		merged = append(merged, list.Items...)
	}
	return &PodLoader{all: merged}, nil
}

// PodK8sMeta holds status columns for a pod from pods.yaml, aligned with the PXC inner pod table.
type PodK8sMeta struct {
	Ready, Status, Restarts, Age, IP, Node string
}

// K8sMetaByPod returns a map keyed "namespace\0name" for rows merged with on-disk PXC log discovery.
func (l *PodLoader) K8sMetaByPod(dumpRoot string, now time.Time) map[string]PodK8sMeta {
	if l == nil {
		return nil
	}
	out := make(map[string]PodK8sMeta, len(l.all))
	for i := range l.all {
		p := &l.all[i]
		r := podItemToRow(p, now, dumpRoot)
		k := p.Metadata.Namespace + "\x00" + p.Metadata.Name
		out[k] = PodK8sMeta{
			Ready:    r.Ready,
			Status:   r.Status,
			Restarts: r.Restarts,
			Age:      r.Age,
			IP:       r.PodIP,
			Node:     r.Node,
		}
	}
	return out
}

type podImageRef struct {
	Display string
	Norm    string
}

// distinctImagesForPXCInstance returns unique container/initContainer images from pods that belong
// to a PerconaXtraDBCluster (namespace + app.kubernetes.io/instance) for haproxy, proxysql, or pxc.
func (l *PodLoader) distinctImagesForPXCInstance(namespace, instance string) []podImageRef {
	if l == nil {
		return nil
	}
	ns := strings.TrimSpace(namespace)
	inst := strings.TrimSpace(instance)
	seen := make(map[string]string)
	for i := range l.all {
		p := &l.all[i]
		if strings.TrimSpace(p.Metadata.Namespace) != ns {
			continue
		}
		labels := p.Metadata.Labels
		if labels == nil {
			continue
		}
		if strings.TrimSpace(labels["app.kubernetes.io/instance"]) != inst {
			continue
		}
		comp := strings.TrimSpace(labels["app.kubernetes.io/component"])
		if comp != "haproxy" && comp != "proxysql" && comp != "pxc" {
			continue
		}
		for _, c := range p.Spec.InitContainers {
			addPodImageSeen(seen, c.Image)
		}
		for _, c := range p.Spec.Containers {
			addPodImageSeen(seen, c.Image)
		}
	}
	norms := make([]string, 0, len(seen))
	for n := range seen {
		norms = append(norms, n)
	}
	sort.Strings(norms)
	out := make([]podImageRef, 0, len(norms))
	for _, n := range norms {
		out = append(out, podImageRef{Display: seen[n], Norm: n})
	}
	return out
}

// podNameForBackupCR returns the pod metadata.name for a backup job whose annotation
// percona.com/backup-name matches the PerconaXtraDBClusterBackup CR name.
func (l *PodLoader) podNameForBackupCR(namespace, backupCRName string) string {
	if l == nil {
		return ""
	}
	ns := strings.TrimSpace(namespace)
	bn := strings.TrimSpace(backupCRName)
	if ns == "" || bn == "" {
		return ""
	}
	for i := range l.all {
		p := &l.all[i]
		if strings.TrimSpace(p.Metadata.Namespace) != ns {
			continue
		}
		if backupNameMatchesPodMetadata(p.Metadata.Labels, p.Metadata.Annotations, bn) {
			return strings.TrimSpace(p.Metadata.Name)
		}
	}
	return ""
}

func backupNameMatchesPodMetadata(labels, annotations map[string]string, backupCRName string) bool {
	key := "percona.com/backup-name"
	if m := labels; m != nil && strings.TrimSpace(m[key]) == backupCRName {
		return true
	}
	if m := annotations; m != nil && strings.TrimSpace(m[key]) == backupCRName {
		return true
	}
	return false
}

func addPodImageSeen(seen map[string]string, image string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return
	}
	norm := normalizeOCIImageRef(image)
	if norm == "" {
		return
	}
	if _, ok := seen[norm]; !ok {
		seen[norm] = image
	}
}

func (l *PodLoader) podsForPerconaComponent(namespace, instance, component string, now time.Time, dumpRoot string) []PXCPodRowTmpl {
	if l == nil {
		return nil
	}
	ns := strings.TrimSpace(namespace)
	inst := strings.TrimSpace(instance)
	comp := strings.TrimSpace(component)
	var matches []podItem
	for i := range l.all {
		p := &l.all[i]
		if podMatchesPerconaComponent(p, ns, inst, comp) {
			matches = append(matches, *p)
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Metadata.Name < matches[j].Metadata.Name
	})
	out := make([]PXCPodRowTmpl, 0, len(matches))
	for i := range matches {
		out = append(out, podItemToRow(&matches[i], now, dumpRoot))
	}
	return out
}

func podMatchesPerconaComponent(p *podItem, namespace, instance, component string) bool {
	if p == nil {
		return false
	}
	if strings.TrimSpace(p.Metadata.Namespace) != namespace {
		return false
	}
	l := p.Metadata.Labels
	if l == nil {
		return false
	}
	if strings.TrimSpace(l["app.kubernetes.io/instance"]) != instance {
		return false
	}
	if strings.TrimSpace(l["app.kubernetes.io/component"]) != component {
		return false
	}
	return true
}

func podItemToRow(p *podItem, now time.Time, dumpRoot string) PXCPodRowTmpl {
	total := len(p.Spec.Containers)
	readyByName := map[string]bool{}
	restarts := 0
	for _, cs := range p.Status.ContainerStatuses {
		readyByName[cs.Name] = cs.Ready
		restarts += cs.RestartCount
	}
	for _, cs := range p.Status.InitContainerStatuses {
		restarts += cs.RestartCount
	}
	ready := 0
	for _, c := range p.Spec.Containers {
		if readyByName[c.Name] {
			ready++
		}
	}
	if total == 0 {
		total = len(p.Status.ContainerStatuses)
	}
	readyStr := fmt.Sprintf("%d/%d", ready, total)
	if total == 0 {
		readyStr = "0/0"
	}

	age := "—"
	if p.Metadata.CreationTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.Metadata.CreationTimestamp)); err == nil {
			age = HumanizeDurationInState(t, now)
		}
	}

	ip := strings.TrimSpace(p.Status.PodIP)
	if ip == "" {
		ip = "—"
	}
	node := strings.TrimSpace(p.Spec.NodeName)
	if node == "" {
		node = "—"
	}
	phase := strings.TrimSpace(p.Status.Phase)
	if phase == "" {
		phase = "—"
	}

	row := PXCPodRowTmpl{
		Name:     p.Metadata.Name,
		Ready:    readyStr,
		Status:   phase,
		Restarts: strconv.Itoa(restarts),
		Age:      age,
		PodIP:    ip,
		Node:     node,
	}
	esc, has := readPodLogFromDump(dumpRoot, p.Metadata.Namespace, p.Metadata.Name)
	row.HasPodLog = has
	row.PodLogEscaped = esc
	row.PodLogModalID = safePodLogStoreID(p.Metadata.Namespace, p.Metadata.Name)
	row.PodSpecModalID = safePodSpecStoreID(p.Metadata.Namespace, p.Metadata.Name)
	row.PodSpecEscaped = htmltemplate.HTMLEscapeString(buildPodSpecDetailPlainText(p))
	return row
}

func buildPodSpecDetailPlainText(p *podItem) string {
	if p == nil {
		return "No pod data."
	}
	var b strings.Builder
	writePodContainerSection(&b, "Init containers", p.Spec.InitContainers)
	writePodContainerSection(&b, "Containers", p.Spec.Containers)
	s := strings.TrimSpace(b.String())
	if s == "" {
		return "No container specs in this pod document."
	}
	return s
}

func writePodContainerSection(b *strings.Builder, title string, list []podContainerSpec) {
	if len(list) == 0 {
		return
	}
	b.WriteString("=== ")
	b.WriteString(title)
	b.WriteString(" ===\n\n")
	for _, c := range list {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = "—"
		}
		img := strings.TrimSpace(c.Image)
		if img == "" {
			img = "—"
		}
		rCPU, rMem := podResourceCPUAndMem(c.Resources, "requests")
		lCPU, lMem := podResourceCPUAndMem(c.Resources, "limits")
		b.WriteString(name)
		b.WriteString("\n")
		fmt.Fprintf(b, "  image:     %s\n", img)
		fmt.Fprintf(b, "  requests:  CPU %s, memory %s\n", rCPU, rMem)
		fmt.Fprintf(b, "  limits:    CPU %s, memory %s\n", lCPU, lMem)
		b.WriteString("\n")
	}
}

func podResourceCPUAndMem(res *podResources, kind string) (cpu, mem string) {
	cpu, mem = "—", "—"
	if res == nil {
		return
	}
	var rl *podResourceList
	if kind == "requests" {
		if res.Requests != nil {
			rl = res.Requests
		}
	} else if kind == "limits" {
		if res.Limits != nil {
			rl = res.Limits
		}
	}
	if rl == nil {
		return
	}
	if s := podQuantityString(rl.CPU); s != "" {
		cpu = s
	}
	if s := podQuantityString(rl.Memory); s != "" {
		mem = s
	}
	return
}

func podQuantityString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strings.TrimSpace(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%g", t), "0"), "."))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func safePodSpecStoreID(ns, podName string) string {
	var b strings.Builder
	b.WriteString("podspect-")
	for _, r := range ns + "-" + podName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// podYAMLModalMaxBytes caps YAML embedded in the static report modal.
const podYAMLModalMaxBytes = 512 * 1024

func safePodYAMLModalStoreID(ns, podName string) string {
	var b strings.Builder
	b.WriteString("plgpodyaml-")
	for _, r := range ns + "-" + podName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// PodYAMLEscapedForModal returns HTML-escaped YAML for one pod: prefers on-disk
// dumpRoot/namespace/pod/pod.yaml when present, otherwise marshals the matching
// document from merged pods.yaml. ok is false when the pod is not in pods.yaml.
func (l *PodLoader) PodYAMLEscapedForModal(dumpRoot, ns, podName string) (escaped string, modalID string, ok bool) {
	ns = strings.TrimSpace(ns)
	podName = strings.TrimSpace(podName)
	if l == nil || ns == "" || podName == "" {
		return "", "", false
	}
	var found *podItem
	for i := range l.all {
		p := &l.all[i]
		if p.Metadata.Namespace == ns && p.Metadata.Name == podName {
			found = p
		}
	}
	if found == nil {
		return "", "", false
	}
	modalID = safePodYAMLModalStoreID(ns, podName)
	base := filepath.Clean(strings.TrimSpace(dumpRoot))
	if base != "" && base != "." {
		for _, name := range []string{"pod.yaml", "Pod.yaml"} {
			p := filepath.Join(base, ns, podName, name)
			raw, err := os.ReadFile(p)
			if err != nil || len(raw) == 0 {
				continue
			}
			trunc := false
			if len(raw) > podYAMLModalMaxBytes {
				raw = raw[:podYAMLModalMaxBytes]
				trunc = true
			}
			s := string(raw)
			if trunc {
				s += "\n\n# … truncated for report embed (see raw cluster dump for full file)"
			}
			return htmltemplate.HTMLEscapeString(s), modalID, true
		}
	}
	raw, err := yaml.Marshal(found)
	if err != nil || len(raw) == 0 {
		return "", "", false
	}
	trunc := false
	if len(raw) > podYAMLModalMaxBytes {
		raw = raw[:podYAMLModalMaxBytes]
		trunc = true
	}
	s := string(raw)
	if trunc {
		s += "\n\n# … truncated for report embed (see raw cluster dump for full document)"
	}
	return htmltemplate.HTMLEscapeString(s), modalID, true
}

func readPodLogFromDump(dumpRoot, namespace, podName string) (escaped string, found bool) {
	ns := strings.TrimSpace(namespace)
	pn := strings.TrimSpace(podName)
	if ns == "" || pn == "" || strings.TrimSpace(dumpRoot) == "" {
		return "", false
	}
	base := filepath.Clean(dumpRoot)
	candidates := []string{
		filepath.Join(base, ns, pn, "logs.txt"),
		filepath.Join(base, ns, pn, "log"),
	}
	var raw []byte
	var err error
	for _, cand := range candidates {
		raw, err = os.ReadFile(cand)
		if err == nil {
			break
		}
	}
	if err != nil || len(raw) == 0 {
		return "", false
	}
	cleaned := flattenPodLogJSONLines(string(raw))
	return htmltemplate.HTMLEscapeString(cleaned), true
}

// flattenPodLogJSONLines turns Fluent-style JSON lines like
// {"log":"...message...\n","file":"/var/lib/mysql/mysqld-error.log"} into plain message lines.
// Non-JSON lines and JSON without a non-empty "log" field are left unchanged.
func flattenPodLogJSONLines(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if !strings.HasPrefix(t, "{") {
			continue
		}
		var wrap struct {
			Log string `json:"log"`
		}
		if err := json.Unmarshal([]byte(t), &wrap); err != nil || wrap.Log == "" {
			continue
		}
		lines[i] = wrap.Log
	}
	return strings.Join(lines, "\n")
}

func safePodLogStoreID(ns, podName string) string {
	var b strings.Builder
	b.WriteString("podlog-")
	for _, r := range ns + "-" + podName {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	s := strings.Trim(b.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}
