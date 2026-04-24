package collector

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"pt-k8s-summary/internal/dumpctx"
)

// pxcPodLogsSection lists PXC-related pod folders, all discoverable log files, and
// offers formatted (cleaned) or full (unfiltered) views in a modal.
type pxcPodLogsSection struct{}

func (pxcPodLogsSection) ID() string    { return "pxc-pod-logs" }
func (pxcPodLogsSection) Title() string { return "PXC · pod logs" }

func (pxcPodLogsSection) Collect(ctx dumpctx.Context) (Section, error) {
	h, err := gatherPodLogsSectionHTML(ctx.Root(), ctx.GaleraSince())
	if err != nil {
		return Section{}, err
	}
	if h == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(h)}, nil
}

const pxcLogMaxBytes = 750 * 1024 // per file embed
const pxcLogMaxRunes  = 400_000

// podLogFile holds one log file's embedded text (cleaned + raw) for the static report.
type podLogFile struct {
	RelInPod  string
	RelInDump string
	Bytes     int
	LinesFmt  int
	Trunc     bool
	EscClean  string
	EscRaw    string
}

// podLogRow is one table row (one PXC pod with one or more log files).
type podLogRow struct {
	Namespace string
	Pod       string
	Files     []podLogFile
}

func isShellXtraceLine(line string) bool {
	s := strings.TrimLeft(line, " \t")
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "++") {
		return true
	}
	if s[0] == '+' {
		if len(s) < 2 {
			return true
		}
		c := s[1]
		if c == ' ' || c == '\t' {
			return true
		}
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '/' || c == '(' {
			return true
		}
	}
	return false
}

type logJSON struct {
	Log  string `json:"log"`
	File string `json:"file"`
}

func cleanPodLogText(raw string) (string, int) {
	var linesOut []string
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(line, " \r\t")
		if isShellXtraceLine(line) {
			continue
		}
		trimL := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimL, "{") && strings.Contains(trimL, `"log"`) {
			var j logJSON
			if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &j); err == nil {
				ln := strings.TrimSuffix(j.Log, "\n")
				if j.File != "" {
					ln = "[" + j.File + "] " + ln
				}
				linesOut = append(linesOut, ln)
				continue
			}
		}
		linesOut = append(linesOut, line)
	}
	var acc []string
	blankStreak := 0
	for _, ln := range linesOut {
		if strings.TrimSpace(ln) == "" {
			blankStreak++
			if blankStreak == 1 {
				acc = append(acc, "")
			}
			continue
		}
		blankStreak = 0
		acc = append(acc, ln)
	}
	out := strings.Join(acc, "\n")
	lines := 0
	if out != "" {
		lines = 1 + strings.Count(out, "\n")
	}
	return out, lines
}

func includePodLogFile(base string) bool {
	b := strings.ToLower(base)
	if b == "logs.txt" || b == "summary.txt" {
		return true
	}
	return strings.HasSuffix(b, ".log")
}

func logFileSortKey(rel string) string {
	switch {
	case rel == "logs.txt":
		return "0"
	case rel == "summary.txt":
		return "1"
	default:
		return "2" + rel
	}
}

func discoverLogFilePathsInPod(podPath string) []string {
	var found []string
	_ = filepath.WalkDir(podPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !includePodLogFile(name) {
			return nil
		}
		rel, err := filepath.Rel(podPath, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		found = append(found, rel)
		return nil
	})
	sort.Slice(found, func(i, j int) bool {
		return logFileSortKey(found[i]) < logFileSortKey(found[j])
	})
	return found
}

func readAndLimit(rawBytes []byte) (raw string, trunc bool) {
	n := len(rawBytes)
	if n > pxcLogMaxBytes {
		return string(rawBytes[:pxcLogMaxBytes]) + "\n… [truncated for report embed]", true
	}
	return string(rawBytes), false
}

func capRunes(s string) string {
	if utf8.RuneCountInString(s) <= pxcLogMaxRunes {
		return s
	}
	return string([]rune(s)[:pxcLogMaxRunes]) + "\n… [truncated for report embed]"
}

func findPodLogRows(dumpRoot string) ([]podLogRow, error) {
	ents, err := os.ReadDir(dumpRoot)
	if err != nil {
		return nil, err
	}
	var pairs []struct{ ns, pod, podPath string }
	for _, nsEnt := range ents {
		if !nsEnt.IsDir() {
			continue
		}
		nsName := nsEnt.Name()
		nsPath := filepath.Join(dumpRoot, nsName)
		pods, err := os.ReadDir(nsPath)
		if err != nil {
			continue
		}
		for _, pEnt := range pods {
			if !pEnt.IsDir() {
				continue
			}
			podName := pEnt.Name()
			if !isPXCWorkloadPodName(podName) {
				continue
			}
			podPath := filepath.Join(nsPath, podName)
			relL := discoverLogFilePathsInPod(podPath)
			if len(relL) == 0 {
				continue
			}
			pairs = append(pairs, struct{ ns, pod, podPath string }{
				ns: nsName, pod: podName, podPath: podPath,
			})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].ns != pairs[j].ns {
			return pairs[i].ns < pairs[j].ns
		}
		return pairs[i].pod < pairs[j].pod
	})
	var out []podLogRow
	for _, p := range pairs {
		relL := discoverLogFilePathsInPod(p.podPath)
		var files []podLogFile
		for _, relInPod := range relL {
			abs := filepath.Join(p.podPath, filepath.FromSlash(relInPod))
			data, err := os.ReadFile(abs)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", abs, err)
			}
			relDump, _ := filepath.Rel(dumpRoot, abs)
			relDump = filepath.ToSlash(relDump)
			raw, t1 := readAndLimit(data)
			raw = capRunes(raw)
			clean, lineCount := cleanPodLogText(raw)
			clean = capRunes(clean)
			t2 := t1
			files = append(files, podLogFile{
				RelInPod:  relInPod,
				RelInDump: relDump,
				Bytes:     len(data),
				LinesFmt:  lineCount,
				Trunc:     t2,
				EscClean:  html.EscapeString(clean),
				EscRaw:    html.EscapeString(raw),
			})
		}
		out = append(out, podLogRow{
			Namespace: p.ns,
			Pod:       p.pod,
			Files:     files,
		})
	}
	return out, nil
}

func gatherPodLogsSectionHTML(dumpRoot, galeraSince string) (string, error) {
	rows, err := findPodLogRows(dumpRoot)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}
	glePaths, err := findPXCMysqldErrorLogPaths(dumpRoot)
	if err != nil {
		return "", err
	}
	var (
		gleOut     string
		gleErr     string
		gleErrText string
	)
	if len(glePaths) > 0 {
		out, serr, e := runPTGaleraLogExplainer(glePaths, galeraSince)
		if e != nil {
			gleErrText = e.Error()
			if serr != "" {
				gleErr = serr
			}
		} else {
			gleOut = out
			if serr != "" {
				gleErr = serr
			}
		}
	} else {
		gleErrText = "" // not an error; use gleInfo in UI
	}
	gleInfo := ""
	if len(glePaths) == 0 {
		gleInfo = "No PXC member mysqld-error.log files were found (look for pods matching *-pxc-<n> with var/lib/mysql/mysqld-error.log)."
	}
	var b strings.Builder
	esc := html.EscapeString
	b.WriteString(`<style>
#pxc-pod-logs .pxc-plg-note { font-size: 0.72rem; color: #64748b; margin: 0 0 0.75rem 0; line-height: 1.45; }
#pxc-pod-logs .pxc-plg-table { width: 100%; border-collapse: collapse; font-size: 0.78rem; }
#pxc-pod-logs .pxc-plg-table th, #pxc-pod-logs .pxc-plg-table td { padding: 0.4rem 0.5rem; border: 1px solid #e2e8f0; text-align: left; vertical-align: middle; }
#pxc-pod-logs .pxc-plg-table th { background: #f1f5f9; font-weight: 650; color: #334155; position: relative; overflow: visible; }
#pxc-pod-logs th.pxc-plg-view { white-space: nowrap; }
#pxc-pod-logs .pxc-plg-helptip { position: relative; display: inline-block; margin-left: 0.2rem; vertical-align: middle; }
#pxc-pod-logs .pxc-plg-helptip__btn { width: 1.1rem; height: 1.1rem; padding: 0; line-height: 1; font-size: 0.7rem; font-style: italic; font-weight: 700; font-family: Georgia, "Times New Roman", serif; color: #0369a1; background: #e0f2fe; border: 1px solid #7dd3fc; border-radius: 50%; cursor: help; vertical-align: text-top; }
#pxc-pod-logs .pxc-plg-helptip__btn:hover, #pxc-pod-logs .pxc-plg-helptip__btn:focus { background: #bae6fd; outline: none; box-shadow: 0 0 0 2px #7dd3fc; }
#pxc-pod-logs .pxc-plg-helptip__box { display: none; position: absolute; z-index: 20; right: 0; top: calc(100% + 0.35rem); min-width: 12.5rem; max-width: min(22rem, 92vw); padding: 0.55rem 0.65rem; font-size: 0.7rem; font-weight: 450; line-height: 1.45; text-align: left; color: #0f172a; background: #fff; border: 1px solid #cbd5e1; border-radius: 8px; box-shadow: 0 8px 20px rgba(15, 23, 42, 0.12); }
#pxc-pod-logs .pxc-plg-helptip__box strong { font-weight: 650; color: #1e293b; }
#pxc-pod-logs .pxc-plg-helptip:hover .pxc-plg-helptip__box,
#pxc-pod-logs .pxc-plg-helptip:focus-within .pxc-plg-helptip__box,
#pxc-pod-logs .pxc-plg-helptip__box.pxc-plg-helptip__box--open { display: block; }
#pxc-pod-logs .pxc-plg-table td.pxc-plg-mono { font-family: ui-monospace, Menlo, monospace; font-size: 0.7rem; }
#pxc-pod-logs .pxc-plg-sel { max-width: 32rem; width: 100%; min-width: 12rem; font-size: 0.74rem; padding: 0.3rem 0.45rem; border: 1px solid #cbd5e1; border-radius: 8px; background: #fff; }
#pxc-pod-logs .pxc-plg-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 0.4rem; }
#pxc-pod-logs .pxc-plg-btn { font-size: 0.72rem; font-weight: 650; padding: 0.32rem 0.55rem; border-radius: 8px; border: 1px solid #0ea5e9; background: #f0f9ff; color: #0369a1; cursor: pointer; }
#pxc-pod-logs .pxc-plg-btn:hover { background: #e0f2fe; }
#pxc-pod-logs .pxc-plg-btn.pxc-plg-raw { border-color: #94a3b8; background: #f8fafc; color: #475569; }
#pxc-pod-logs .pxc-plg-btn.pxc-plg-raw:hover { background: #f1f5f9; }
#pxc-pod-logs .pxc-plg-blob { display: none; }
#pxc-log-modal-pxc { position: fixed; inset: 0; z-index: 9998; display: flex; align-items: center; justify-content: center; background: rgba(15, 23, 42, 0.45); opacity: 0; pointer-events: none; transition: opacity 0.12s; }
#pxc-log-modal-pxc[aria-hidden="false"] { opacity: 1; pointer-events: auto; }
#pxc-log-modal-pxc .pxc-plg-dlg { background: #fff; color: #0f172a; max-width: min(96vw, 62rem); max-height: 88vh; display: flex; flex-direction: column; border-radius: 12px; box-shadow: 0 20px 50px rgba(0,0,0,0.25); overflow: hidden; border: 1px solid #e2e8f0; }
#pxc-log-modal-pxc .pxc-plg-dlg-h { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; padding: 0.55rem 0.9rem; background: #f8fafc; border-bottom: 1px solid #e2e8f0; }
#pxc-log-modal-pxc .pxc-plg-dlg-t { font-size: 0.92rem; font-weight: 650; margin: 0; line-height: 1.3; }
#pxc-log-modal-pxc .pxc-plg-dlg-c { display: block; width: 2.25rem; height: 2.25rem; line-height: 1; border: none; background: transparent; color: #64748b; font-size: 1.5rem; cursor: pointer; border-radius: 8px; }
#pxc-log-modal-pxc .pxc-plg-dlg-c:hover { background: #e2e8f0; color: #0f172a; }
#pxc-log-modal-pxc .pxc-plg-pre { margin: 0; padding: 0.9rem; overflow: auto; max-height: calc(88vh - 3.2rem); font-size: 0.72rem; line-height: 1.4; font-family: ui-monospace, Menlo, Consolas, monospace; white-space: pre-wrap; word-break: break-word; background: #0f172a; color: #e2e8f0; }
#pxc-pod-logs .pxc-plg-meta { font-size: 0.65rem; color: #94a3b8; margin-top: 0.2rem; }
#pxc-pod-logs .pxc-gle-wrap { margin: 0 0 1rem 0; padding: 0.9rem 1rem; background: linear-gradient(165deg,#f8fafc 0%,#f1f5f9 100%); border: 1px solid #e2e8f0; border-radius: 12px; }
#pxc-pod-logs .pxc-gle-h4 { margin: 0 0 0.5rem 0; font-size: 0.95rem; font-weight: 650; color: #0f172a; }
#pxc-pod-logs .pxc-gle-h4 a { color: #0f172a; text-decoration: none; }
#pxc-pod-logs .pxc-gle-h4 a:hover { text-decoration: underline; }
#pxc-pod-logs .pxc-gle-h4 code { font-size: 0.8rem; font-weight: 550; }
#pxc-pod-logs .pxc-gle-btn { font-size: 0.78rem; font-weight: 650; padding: 0.4rem 0.75rem; border-radius: 8px; border: 1px solid #0d9488; background: #ecfdf5; color: #0f766e; cursor: pointer; }
#pxc-pod-logs .pxc-gle-btn:hover { background: #d1fae5; }
#pxc-pod-logs .pxc-gle-btn:disabled { opacity: 0.55; cursor: not-allowed; }
#pxc-pod-logs .pxc-gle-meta { font-size: 0.64rem; color: #94a3b8; margin-top: 0.4rem; }
#pxc-pod-logs .pxc-gle-err { font-size: 0.7rem; color: #b91c1c; margin: 0.4rem 0 0 0; line-height: 1.35; }
#pxc-pod-logs .pxc-gle-warn { font-size: 0.68rem; color: #a16207; margin: 0.35rem 0 0 0; line-height: 1.35; }
#pxc-gle-modal-pxc { position: fixed; inset: 0; z-index: 9999; display: flex; align-items: center; justify-content: center; background: rgba(15, 23, 42, 0.5); opacity: 0; pointer-events: none; transition: opacity 0.12s; }
#pxc-gle-modal-pxc[aria-hidden="false"] { opacity: 1; pointer-events: auto; }
#pxc-gle-modal-pxc .pxc-gle-dlg { background: #fff; color: #0f172a; width: min(99vw, 80rem); max-width: 99vw; max-height: 92vh; display: flex; flex-direction: column; border-radius: 12px; box-shadow: 0 20px 50px rgba(0,0,0,0.28); overflow: hidden; border: 1px solid #e2e8f0; }
#pxc-gle-modal-pxc .pxc-gle-dlg-h { display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; padding: 0.5rem 0.85rem; background: #f0fdfa; border-bottom: 1px solid #99f6e4; }
#pxc-gle-modal-pxc .pxc-gle-dlg-t { font-size: 0.9rem; font-weight: 650; margin: 0; }
#pxc-gle-modal-pxc .pxc-gle-dlg-c { display: block; width: 2.25rem; height: 2.25rem; line-height: 1; border: none; background: transparent; color: #64748b; font-size: 1.5rem; cursor: pointer; border-radius: 8px; }
#pxc-gle-modal-pxc .pxc-gle-dlg-c:hover { background: #e2e8f0; color: #0f172a; }
#pxc-gle-modal-pxc .pxc-gle-tab { margin: 0; padding: 0.65rem 0.8rem; overflow: auto; max-height: calc(92vh - 2.6rem); font-size: 0.66rem; line-height: 1.1; font-family: ui-monospace, Menlo, "Consolas", "Liberation Mono", monospace; white-space: pre; tab-size: 2; word-break: normal; color: #0f172a; background: #fff; border: none; }
#pxc-pod-logs .pxc-gle-blob { display: none; }
</style>`)
	b.WriteString(`<p class="pxc-plg-note">PXC-related pods (name contains <code>-pxc-</code>, <code>pxc-operator</code>, <code>xtradb-cluster-operator</code>, or <code>haproxy</code> / <code>proxysql</code>) are scanned for log files: <code>logs.txt</code>, <code>summary.txt</code>, and all <code>*.log</code> files under the pod directory (e.g. <code>var/lib/mysql/mysqld-error.log</code>). Choose a file in the <strong>Log file</strong> list, then open it as <strong>Formatted</strong> (removes <code>set -x</code> noise, expands JSON log lines) or <strong>Full</strong> (exact dump content, subject to the same size cap for embedding—use the raw cluster dump to inspect without limits).</p>`)
	b.WriteString(`<div class="pxc-gle-wrap"><h4 class="pxc-gle-h4"><a href="https://docs.percona.com/percona-toolkit/pt-galera-log-explainer.html" rel="noopener noreferrer" target="_blank">pt-galera-log-explainer</a> <code>list --all</code></h4>`)
	if strings.TrimSpace(galeraSince) != "" {
		b.WriteString(`<p class="pxc-gle-meta" style="margin:0 0 0.5rem 0;">` + esc("--since "+galeraSince) + `</p>`)
	}
	if gleOut != "" {
		b.WriteString(`<button type="button" class="pxc-gle-btn" id="pxc-gle-open">View timeline</button>`)
		if gleErr != "" {
			b.WriteString(`<p class="pxc-gle-warn" title="` + esc(gleErr) + `">` + esc(truncateString("stderr: "+gleErr, 500)) + `</p>`)
		}
	} else {
		if gleInfo != "" {
			b.WriteString(`<p class="pxc-gle-meta" style="color:#64748b;">` + esc(gleInfo) + `</p>`)
		}
		if gleErr != "" {
			b.WriteString(`<p class="pxc-gle-err" title="` + esc(gleErr) + `">` + esc(truncateString(gleErr, 500)) + `</p>`)
		}
		if gleErrText != "" {
			b.WriteString(`<p class="pxc-gle-err">` + esc(gleErrText) + `</p>`)
		}
		b.WriteString(`<button type="button" class="pxc-gle-btn" id="pxc-gle-open" disabled>View timeline</button>`)
	}
	b.WriteString(`</div>`)
	if gleOut != "" {
		b.WriteString(`<pre class="pxc-gle-blob" id="pxc-gle-stash">` + esc(gleOut) + `</pre>`)
	} else {
		b.WriteString(`<pre class="pxc-gle-blob" id="pxc-gle-stash"></pre>`)
	}
	b.WriteString(`<div id="pxc-gle-modal-pxc" aria-hidden="true" role="dialog" aria-modal="true" aria-label="pt-galera-log-explainer output">`)
	b.WriteString(`<div class="pxc-gle-dlg" role="document" tabindex="-1">`)
	b.WriteString(`<div class="pxc-gle-dlg-h"><h4 class="pxc-gle-dlg-t" id="pxc-gle-dlg-title-pxc">pt-galera-log-explainer</h4>`)
	b.WriteString(`<button type="button" class="pxc-gle-dlg-c" data-pxc-gle-x="" aria-label="Close">×</button></div>`)
	b.WriteString(`<pre class="pxc-gle-tab" id="pxc-gle-dlg-body-pxc" tabindex="0" aria-live="polite" aria-atomic="true" aria-labelledby="pxc-gle-dlg-title-pxc">`)
	b.WriteString(`</pre></div></div>`)

	b.WriteString(`<table class="pxc-plg-table"><thead><tr><th>Namespace</th><th>Pod</th><th>Log file</th><th class="pxc-plg-view">View<span class="pxc-plg-helptip"><button type="button" class="pxc-plg-helptip__btn" id="pxc-plg-helptip-btn" aria-expanded="false" aria-controls="pxc-plg-helptip-box" title="What Formatted and Full mean (hover or click)">i</button><div class="pxc-plg-helptip__box" id="pxc-plg-helptip-box" role="tooltip" aria-hidden="true"><strong>Formatted</strong> &mdash; Drops shell <code>set -x</code> noise and expands one-line JSON log lines so the file is easier to read.<br><br><strong>Full</strong> &mdash; The exact file content as collected, with no line filtering (subject to the same per-file size cap in this report; use the raw dump to see everything).</div></span></th></tr></thead><tbody>`)
	for podIdx, row := range rows {
		b.WriteString(`<tr data-pxc-plg-p="`)
		b.WriteString(esc(fmt.Sprintf("%d", podIdx)))
		b.WriteString(`"><td>`)
		b.WriteString(esc(row.Namespace))
		b.WriteString(`</td><td class="pxc-plg-mono">`)
		b.WriteString(esc(row.Pod))
		b.WriteString(`</td><td><select class="pxc-plg-sel" aria-label="Log file for `)
		b.WriteString(esc(row.Pod))
		b.WriteString(`">`)
		for fIdx, f := range row.Files {
			optTitle := f.RelInDump + " | " + fmt.Sprintf("%d lines (formatted) | %s", f.LinesFmt, humanSize(f.Bytes))
			if f.Trunc {
				optTitle += " | truncated in report"
			}
			b.WriteString(`<option value="`)
			b.WriteString(esc(fmt.Sprintf("%d", fIdx)))
			b.WriteString(`" data-fpath="`)
			b.WriteString(esc(f.RelInPod))
			b.WriteString(`" title="`)
			b.WriteString(esc(optTitle))
			b.WriteString(`">`)
			b.WriteString(esc(f.RelInPod + " | " + humanSize(f.Bytes)))
			if f.Trunc {
				b.WriteString(esc(" (part.)"))
			}
			b.WriteString(`</option>`)
		}
		b.WriteString(`</select><div class="pxc-plg-meta">`)
		b.WriteString(esc(fmt.Sprintf("%d file(s) in this pod", len(row.Files))))
		b.WriteString(`</div></td><td><div class="pxc-plg-actions"><button type="button" class="pxc-plg-btn pxc-plg-fmt">Formatted</button><button type="button" class="pxc-plg-btn pxc-plg-raw">Full</button></div></td></tr>`)
	}
	b.WriteString(`</tbody></table>`)
	for podIdx, row := range rows {
		for fIdx, f := range row.Files {
			b.WriteString(`<pre class="pxc-plg-blob" id="pxc-plg-stash-`)
			b.WriteString(esc(fmt.Sprintf("%d-%d-clean", podIdx, fIdx)))
			b.WriteString(`">`)
			b.WriteString(f.EscClean)
			b.WriteString(`</pre><pre class="pxc-plg-blob" id="pxc-plg-stash-`)
			b.WriteString(esc(fmt.Sprintf("%d-%d-raw", podIdx, fIdx)))
			b.WriteString(`">`)
			b.WriteString(f.EscRaw)
			b.WriteString(`</pre>`)
		}
	}
	b.WriteString(`<div id="pxc-log-modal-pxc" class="pxc-plg-backdrop" aria-hidden="true" role="dialog" aria-modal="true" aria-label="Pod log viewer">`)
	b.WriteString(`<div class="pxc-plg-dlg" role="document" tabindex="-1">`)
	b.WriteString(`<div class="pxc-plg-dlg-h"><h4 class="pxc-plg-dlg-t" id="pxc-plg-dlg-title-pxc">Log</h4><button type="button" class="pxc-plg-dlg-c" data-pxc-plg-x="" aria-label="Close">×</button></div>`)
	b.WriteString(`<pre class="pxc-plg-pre" id="pxc-plg-dlg-body-pxc" tabindex="0" aria-live="polite" aria-atomic="true" aria-labelledby="pxc-plg-dlg-title-pxc">`)
	b.WriteString(`</pre></div></div>`)
	b.WriteString("<script>(function(){\n")
	b.WriteString(`  function closePlgHelptip(){
    var b=document.getElementById("pxc-plg-helptip-box");
    var btn=document.getElementById("pxc-plg-helptip-btn");
    if(b && b.classList.contains("pxc-plg-helptip__box--open")){
      b.classList.remove("pxc-plg-helptip__box--open");
      if(btn) btn.setAttribute("aria-expanded", "false");
    }
  }
  var hBtn=document.getElementById("pxc-plg-helptip-btn");
  var hBox=document.getElementById("pxc-plg-helptip-box");
  if(hBtn && hBox){
    hBtn.addEventListener("click", function(ev){
      ev.stopPropagation();
      var o=!hBox.classList.contains("pxc-plg-helptip__box--open");
      hBox.classList.toggle("pxc-plg-helptip__box--open", o);
      hBtn.setAttribute("aria-expanded", o?"true":"false");
    });
  }
  document.addEventListener("click", function(ev){
    if(ev.target && ev.target.closest && ev.target.closest("#pxc-plg-helptip-btn, #pxc-plg-helptip-box")) return;
    closePlgHelptip();
  });
  var m=document.getElementById("pxc-log-modal-pxc");
  if(!m) return;
  var body=document.getElementById("pxc-plg-dlg-body-pxc");
  var title=document.getElementById("pxc-plg-dlg-title-pxc");
  function openLog(p,fi,mode,sub){
    var id="pxc-plg-stash-"+p+"-"+fi+"-"+mode;
    var s=document.getElementById(id);
    body.textContent=s?s.textContent:"(empty)";
    title.textContent=sub||"Log";
    m.setAttribute("aria-hidden","false");
    body.focus();
  }
  function closeModal(){
    m.setAttribute("aria-hidden","true");
    body.textContent="";
  }
  m.querySelectorAll("[data-pxc-plg-x]").forEach(function(el){el.addEventListener("click",closeModal);});
  m.addEventListener("click",function(ev){if(ev.target===m)closeModal();});
  document.addEventListener("keydown", function(ev){
    if(ev.key!=="Escape") return;
    if(m.getAttribute("aria-hidden")==="false") closeModal();
    closePlgHelptip();
  });
  var mGle=document.getElementById("pxc-gle-modal-pxc");
  var bGle=document.getElementById("pxc-gle-open");
  var stashGle=document.getElementById("pxc-gle-stash");
  var bodyGle=document.getElementById("pxc-gle-dlg-body-pxc");
  if(mGle && bGle && bodyGle) {
    function openGle(){
      if(bGle.disabled) return;
      var s=stashGle?stashGle.textContent:"";
      bodyGle.textContent=s||"(empty)";
      mGle.setAttribute("aria-hidden","false");
      bodyGle.focus();
    }
    function closeGle(){
      mGle.setAttribute("aria-hidden","true");
      bodyGle.textContent="";
    }
    bGle.addEventListener("click", openGle);
    mGle.querySelectorAll("[data-pxc-gle-x]").forEach(function(el){ el.addEventListener("click", closeGle); });
    mGle.addEventListener("click", function(ev){ if(ev.target===mGle) closeGle(); });
    document.addEventListener("keydown", function(ev){
      if(ev.key!=="Escape") return;
      if(mGle.getAttribute("aria-hidden")!=="false") return;
      closeGle();
    });
  }
  var sec=document.getElementById("pxc-pod-logs");
  if(!sec)return;
  sec.querySelectorAll("tr[data-pxc-plg-p]").forEach(function(tr){
    function doOpen(mode, fmtLabel){
      var p=tr.getAttribute("data-pxc-plg-p")||"0";
      var sel=tr.querySelector("select.pxc-plg-sel");
      var fi=sel?sel.value:"0";
      var opt=sel&&sel.options[sel.selectedIndex];
      var fpath=opt&&opt.getAttribute?opt.getAttribute("data-fpath"):"";
      if(!fpath && opt) fpath=opt.value;
      var podEl=tr.querySelector("td.pxc-plg-mono");
      var nsEl=tr&&tr.querySelector?tr.querySelector("td:first-child"):null;
      var podT=podEl?podEl.textContent.trim():"";
      var nsT=nsEl?nsEl.textContent.trim():"";
      var sub=nsT+" / "+podT+" / "+fpath+" \u2014 "+fmtLabel;
      openLog(p,fi,mode,sub);
    }
    var bf=tr.querySelector(".pxc-plg-fmt");
    var br=tr.querySelector(".pxc-plg-raw");
    if(bf) bf.addEventListener("click",function(){ doOpen("clean", "formatted"); });
    if(br) br.addEventListener("click", function(){ doOpen("raw", "full (unfiltered in embed)"); });
  });
})();`)
	b.WriteString("</sc" + "ript>")
	return b.String(), nil
}

func truncateString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	// Truncate on byte boundary could split UTF-8; cut runes.
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func isPXCWorkloadPodName(pod string) bool {
	p := strings.ToLower(pod)
	if strings.Contains(p, "-pxc-") {
		return true
	}
	if strings.Contains(p, "haproxy") {
		return true
	}
	if strings.Contains(p, "proxysql") {
		return true
	}
	// Percona PXC / XtraDB cluster operator (e.g. pxc-operator-…, percona-xtradb-cluster-operator-…)
	if strings.Contains(p, "pxc-operator") {
		return true
	}
	if strings.Contains(p, "xtradb-cluster-operator") {
		return true
	}
	return false
}

func humanSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	}
	return fmt.Sprintf("%.2f MiB", float64(n)/(1024*1024))
}
