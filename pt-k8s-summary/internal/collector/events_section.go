package collector

import (
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"pt-k8s-summary/internal/dumpctx"

	"gopkg.in/yaml.v3"
)

const eventsFileName = "events.yaml"
const eventsMessageMaxRunes = 400

// eventsDumpSection renders merged Kubernetes Event lists from namespace events.yaml dumps.
type eventsDumpSection struct{}

func (eventsDumpSection) ID() string    { return "dump-events" }
func (eventsDumpSection) Title() string { return "Cluster events" }

func (eventsDumpSection) Collect(ctx dumpctx.Context) (Section, error) {
	h, err := gatherEventsSectionHTML(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if h == "" {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(h)}, nil
}

type eventListYAML struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Items      []eventItemYAML `yaml:"items"`
}

type eventItemYAML struct {
	Type           string `yaml:"type"`
	Reason         string `yaml:"reason"`
	Message        string `yaml:"message"`
	LastTimestamp  string `yaml:"lastTimestamp"`
	FirstTimestamp string `yaml:"firstTimestamp"`
	EventTime      any    `yaml:"eventTime"` // string MicroTime or null
	Count          int    `yaml:"count"`
	InvolvedObject struct {
		Kind            string `yaml:"kind"`
		Name            string `yaml:"name"`
		Namespace       string `yaml:"namespace"`
		ResourceVersion string `yaml:"resourceVersion"`
	} `yaml:"involvedObject"`
	Metadata struct {
		Namespace         string `yaml:"namespace"`
		CreationTimestamp string `yaml:"creationTimestamp"`
	} `yaml:"metadata"`
}

type eventDisplayRow struct {
	SortTime   time.Time
	LastSeen   string
	Type       string
	Namespace  string
	Object     string // kind/name (involved object)
	Reason     string
	Message    string
	MessageEsc string // for title attribute
	Count      string
	RowClass   string
}

func findEventsYAMLPaths(dumpRoot string) ([]string, error) {
	ents, err := os.ReadDir(dumpRoot)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dumpRoot, e.Name(), eventsFileName)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}

func eventTimeString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func parseK8sEventTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "null") {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05.999999Z07:00",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z0700",
	}
	for _, layout := range layouts {
		if tt, err := time.Parse(layout, s); err == nil {
			return tt.UTC(), true
		}
	}
	return time.Time{}, false
}

func sortTimeForEvent(ev eventItemYAML) time.Time {
	candidates := []string{
		eventTimeString(ev.EventTime),
		ev.LastTimestamp,
		ev.FirstTimestamp,
		ev.Metadata.CreationTimestamp,
	}
	for _, c := range candidates {
		if t, ok := parseK8sEventTime(c); ok {
			return t
		}
	}
	return time.Time{}
}

func truncateRunesMsg(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

func gatherEventsSectionHTML(dumpRoot string) (string, error) {
	paths, err := findEventsYAMLPaths(dumpRoot)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", nil
	}
	var rows []eventDisplayRow
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var list eventListYAML
		if err := yaml.Unmarshal(data, &list); err != nil || len(list.Items) == 0 {
			continue
		}
		for _, ev := range list.Items {
			ns := ev.Metadata.Namespace
			if ns == "" {
				ns = ev.InvolvedObject.Namespace
			}
			inv := strings.TrimSpace(ev.InvolvedObject.Kind + "/" + ev.InvolvedObject.Name)
			if inv == "/" {
				inv = "—"
			}
			st := sortTimeForEvent(ev)
			last := "—"
			if !st.IsZero() {
				last = st.UTC().Format("2006-01-02 15:04:05 UTC")
			}
			msg := strings.TrimSpace(ev.Message)
			msgEsc := html.EscapeString(msg)
			msgCell := html.EscapeString(truncateRunesMsg(msg, eventsMessageMaxRunes))
			rc := ""
			if strings.EqualFold(strings.TrimSpace(ev.Type), "Warning") {
				rc = "dump-ev-warn"
			}
			cnt := "1"
			if ev.Count > 1 {
				cnt = fmt.Sprintf("%d", ev.Count)
			}
			rows = append(rows, eventDisplayRow{
				SortTime:   st,
				LastSeen:   last,
				Type:       strings.TrimSpace(ev.Type),
				Namespace: strings.TrimSpace(ns),
				Object:     inv,
				Reason:     strings.TrimSpace(ev.Reason),
				Message:    msgCell,
				MessageEsc: msgEsc,
				Count:      cnt,
				RowClass:   rc,
			})
		}
	}
	if len(rows) == 0 {
		return "", nil
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ti, tj := rows[i].SortTime, rows[j].SortTime
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return rows[i].LastSeen < rows[j].LastSeen
	})
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<style>
#dump-events .dump-ev-inner { padding-top: 0.35rem; }
#dump-events .dump-ev-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 0.65rem 1rem; margin: 0 0 0.65rem 0; }
#dump-events .dump-ev-toolbar label { font-size: 0.78rem; color: #475569; font-weight: 600; display: inline-flex; align-items: center; gap: 0.4rem; }
#dump-events .dump-ev-filter { min-width: min(18rem, 100%); padding: 0.38rem 0.55rem; font-size: 0.82rem; border: 1px solid #cbd5e1; border-radius: 8px; background: #fff; }
#dump-events .dump-ev-filter:focus { outline: 2px solid #818cf8; border-color: #6366f1; }
#dump-events .dump-ev-meta { font-size: 0.72rem; color: #64748b; }
#dump-events .dump-ev-scroll { max-height: min(70vh, 38rem); overflow: auto; border: 1px solid #e2e8f0; border-radius: 10px; background: #fff; box-shadow: inset 0 1px 2px rgba(15,23,42,0.04); }
#dump-events .dump-ev-table { width: 100%; border-collapse: collapse; font-size: 0.74rem; }
#dump-events .dump-ev-table th { position: sticky; top: 0; z-index: 2; background: linear-gradient(180deg, #f1f5f9 0%, #e2e8f0 100%); color: #334155; font-weight: 650; text-align: left; padding: 0.45rem 0.5rem; border-bottom: 2px solid #cbd5e1; white-space: nowrap; }
#dump-events .dump-ev-table td { padding: 0.38rem 0.5rem; border-bottom: 1px solid #f1f5f9; vertical-align: top; }
#dump-events .dump-ev-table tr:hover td { background: #f8fafc; }
#dump-events .dump-ev-table .dump-ev-mono { font-family: ui-monospace, Menlo, Consolas, monospace; font-size: 0.7rem; }
#dump-events .dump-ev-table .dump-ev-msg { max-width: 28rem; word-break: break-word; white-space: normal; line-height: 1.35; color: #1e293b; }
#dump-events .dump-ev-table td.dump-ev-type { font-weight: 650; }
#dump-events .dump-ev-table tr.dump-ev-warn td.dump-ev-type { color: #b91c1c; }
#dump-events .dump-ev-note { font-size: 0.7rem; color: #64748b; margin: 0.5rem 0 0 0; line-height: 1.4; }
</style>`)
	b.WriteString(`<p class="dump-ev-note">Merged from <code>events.yaml</code> under each namespace folder in the dump. Newest events first (by <code>eventTime</code> / <code>lastTimestamp</code> / <code>firstTimestamp</code> / creation time). Use the filter to match any column text.</p>`)
	b.WriteString(`<details class="nodes-coll dump-ev-outer"><summary class="nodes-coll-sum" aria-label="Expand or collapse the events table"><span class="nodes-coll-exp" aria-hidden="true"></span><span class="nodes-coll-sum-body"><strong class="nodes-coll-sum-h">Kubernetes events</strong><span class="nodes-coll-sum-meta">`)
	b.WriteString(esc(fmt.Sprintf("%d event(s) · newest first · filterable grid", len(rows))))
	b.WriteString(`</span></span></summary><div class="nodes-coll-inner dump-ev-inner">`)
	b.WriteString(`<div class="dump-ev-toolbar"><label>Filter <input type="search" class="dump-ev-filter" id="dump-ev-filter" placeholder="Reason, message, object, namespace…" autocomplete="off" spellcheck="false"></label><span class="dump-ev-meta" id="dump-ev-visible"></span></div>`)
	b.WriteString(`<div class="dump-ev-scroll"><table class="dump-ev-table"><thead><tr><th>Last seen</th><th>Type</th><th>Namespace</th><th>Object</th><th>Reason</th><th>Message</th><th>Count</th></tr></thead><tbody class="dump-ev-tbody">`)
	for _, r := range rows {
		b.WriteString(`<tr class="`)
		b.WriteString(esc(r.RowClass))
		b.WriteString(`"><td class="dump-ev-mono">`)
		b.WriteString(esc(r.LastSeen))
		b.WriteString(`</td><td class="dump-ev-type">`)
		b.WriteString(esc(r.Type))
		b.WriteString(`</td><td>`)
		b.WriteString(esc(r.Namespace))
		b.WriteString(`</td><td class="dump-ev-mono">`)
		b.WriteString(esc(r.Object))
		b.WriteString(`</td><td>`)
		b.WriteString(esc(r.Reason))
		b.WriteString(`</td><td class="dump-ev-msg" title="`)
		b.WriteString(r.MessageEsc)
		b.WriteString(`">`)
		b.WriteString(r.Message)
		b.WriteString(`</td><td>`)
		b.WriteString(esc(r.Count))
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table></div></div></details>`)
	b.WriteString(`<script>(function(){
  var sec=document.getElementById("dump-events");
  if(!sec)return;
  var inp=sec.querySelector("#dump-ev-filter");
  var tbody=sec.querySelector(".dump-ev-tbody");
  var vis=sec.querySelector("#dump-ev-visible");
  if(!inp||!tbody)return;
  var total=tbody.querySelectorAll("tr").length;
  function updateCount(n){
    if(vis) vis.textContent=n===total ? total+" shown" : n+" of "+total+" shown";
  }
  updateCount(total);
  inp.addEventListener("input", function(){
    var q=inp.value.trim().toLowerCase();
    var n=0;
    tbody.querySelectorAll("tr").forEach(function(tr){
      var show=q===""||tr.textContent.toLowerCase().indexOf(q)>=0;
      tr.style.display=show?"":"none";
      if(show) n++;
    });
    updateCount(n);
  });
})();</script>`)
	return b.String(), nil
}
