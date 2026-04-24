package collector

import (
	"html"
	"html/template"
	"strings"

	"pt-k8s-summary/internal/dumpctx"
)

// pxcUnsafePauseRowSection renders unsafeFlags and spec.pause in one section,
// stacked vertically (unsafeFlags first, then spec.pause).
type pxcUnsafePauseRowSection struct{}

func (pxcUnsafePauseRowSection) ID() string    { return "pxc-unsafe-flags-pause" }
func (pxcUnsafePauseRowSection) Title() string { return "" }

func (pxcUnsafePauseRowSection) Collect(ctx dumpctx.Context) (Section, error) {
	unsafeRows, err := gatherUnsafeFlagRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	pauseRows, err := gatherPauseRows(ctx.Root())
	if err != nil {
		return Section{}, err
	}
	if len(unsafeRows) == 0 && len(pauseRows) == 0 {
		return Section{}, nil
	}
	return Section{HTML: template.HTML(renderUnsafePauseRow(unsafeRows, pauseRows))}, nil
}

const rowPXCSep = "\x00"

func rowPXCKey(name, namespace string) string {
	return name + rowPXCSep + namespace
}

func splitRowPXCKey(k string) (name, namespace string) {
	i := strings.Index(k, rowPXCSep)
	if i < 0 {
		return k, ""
	}
	return k[:i], k[i+len(rowPXCSep):]
}

func renderUnsafePauseRow(unsafeClusters []unsafeFlagCluster, pauseRows []pauseRow) string {
	pauseBy := make(map[string]pauseRow, len(pauseRows))
	for _, p := range pauseRows {
		pauseBy[rowPXCKey(p.Name, p.Namespace)] = p
	}
	unsafeBy := make(map[string]unsafeFlagCluster, len(unsafeClusters))
	for _, c := range unsafeClusters {
		unsafeBy[rowPXCKey(c.Name, c.Namespace)] = c
	}

	seen := make(map[string]struct{})
	var keys []string
	for _, c := range unsafeClusters {
		k := rowPXCKey(c.Name, c.Namespace)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for _, p := range pauseRows {
		k := rowPXCKey(p.Name, p.Namespace)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}

	var b strings.Builder
	b.WriteString(`<style>
.pxc-up-stack { font-family: ui-sans-serif, system-ui, sans-serif; font-size: 0.78rem; color: #1e293b; box-sizing: border-box; }
.pxc-up-subh3 { font-size: 1.05rem; margin: 0 0 0.45rem 0; font-weight: 650; line-height: 1.25; }
.pxc-up-note { margin: 0 0 0.75rem 0; font-size: 0.72rem; color: #64748b; line-height: 1.45; }
.pxc-up-block + .pxc-up-block { margin-top: 1.25rem; padding-top: 1.1rem; border-top: 1px solid #e5e7eb; }
.pxc-up-cluster { margin-bottom: 1rem; padding-bottom: 0.85rem; border-bottom: 1px solid #e5e7eb; }
.pxc-up-cluster:last-child { margin-bottom: 0; padding-bottom: 0; border-bottom: none; }
.pxc-cluster-line { display: flex; align-items: baseline; flex-wrap: wrap; gap: 0.35rem 0.5rem; margin-bottom: 0.45rem; }
.pxc-cluster-cr { font-family: ui-monospace, Menlo, monospace; font-weight: 700; font-size: 0.8rem; color: #0f172a; }
.pxc-cluster-ns { font-size: 0.65rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
.pxc-cluster-sep { color: #cbd5e1; font-weight: 400; user-select: none; }
.pxc-pause-table { width: 100%; border-collapse: collapse; font-size: 0.78rem; margin-top: 0.15rem; }
.pxc-pause-table th { text-align: left; padding: 0.4rem 0.55rem; background: #f1f5f9; border: 1px solid #e2e8f0; font-weight: 650; color: #334155; white-space: nowrap; }
.pxc-pause-table td { padding: 0.45rem 0.55rem; border: 1px solid #e2e8f0; vertical-align: middle; }
.pxc-pause-table td.pxc-pause-cr { font-family: ui-monospace, Menlo, monospace; font-weight: 700; color: #0f172a; }
.pxc-pause-table td.pxc-pause-ns { font-size: 0.65rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; }
.pxc-pause-table td.pxc-pause-cell { text-align: left; }
.unsafe-tab-row { display: flex; gap: 0.35rem; flex-wrap: wrap; align-items: stretch; }
.unsafe-tab { flex: 1 1 5.5rem; max-width: 9rem; border: 1px solid #e2e8f0; border-radius: 10px 10px 8px 8px; overflow: hidden; box-shadow: 0 1px 2px rgba(15,23,42,.05); }
.unsafe-tab-name { background: linear-gradient(180deg,#f8fafc,#f1f5f9); padding: 0.28rem 0.35rem; font-size: 0.68rem; font-weight: 700; text-align: center; border-bottom: 1px solid #e2e8f0; font-family: ui-monospace, Menlo, monospace; color: #334155; }
.unsafe-tab-st { padding: 0.38rem 0.35rem; text-align: center; font-size: 0.7rem; font-weight: 700; letter-spacing: 0.02em; }
.unsafe-tab-st.active { background: linear-gradient(180deg,#fff1f2,#ffe4e6); color: #be123c; }
.unsafe-tab-st.inactive { background: linear-gradient(180deg,#f0fdf4,#dcfce7); color: #166534; }
.unsafe-tab-st.unset { background: #f8fafc; color: #64748b; font-weight: 600; }
.pause-val { margin: 0; font-size: 0.78rem; font-weight: 800; letter-spacing: 0.03em; }
.pause-val.true { color: #b91c1c; background: linear-gradient(180deg,#fef2f2,#fee2e2); padding: 0.25rem 0.45rem; border-radius: 8px; display: inline-block; border: 1px solid #fecaca; }
.pause-val.false { color: #166534; background: linear-gradient(180deg,#f0fdf4,#dcfce7); padding: 0.25rem 0.45rem; border-radius: 8px; display: inline-block; border: 1px solid #bbf7d0; }
.pause-val.unset { color: #64748b; background: #f8fafc; padding: 0.25rem 0.45rem; border-radius: 8px; display: inline-block; border: 1px solid #e2e8f0; font-weight: 700; }
#pxc-unsafe-flags-pause .unsafe-tab-row { flex-wrap: nowrap; gap: 0.28rem; }
#pxc-unsafe-flags-pause .unsafe-tab { flex: 1 1 0; min-width: 0; max-width: none; }
#pxc-unsafe-flags-pause .unsafe-tab-name { font-size: 0.6rem; padding: 0.22rem 0.2rem; word-break: break-word; hyphens: auto; }
#pxc-unsafe-flags-pause .unsafe-tab-st { font-size: 0.62rem; padding: 0.28rem 0.2rem; }
@media (max-width: 560px) {
	#pxc-unsafe-flags-pause .unsafe-tab-row { flex-wrap: wrap; }
}
</style>`)

	b.WriteString(`<div class="pxc-up-stack">`)

	// —— unsafeFlags ——
	b.WriteString(`<div class="pxc-up-block pxc-up-unsafe"><h3 class="pxc-up-subh3">PXC · unsafeFlags</h3>`)
	b.WriteString(`<p class="pxc-up-note">Values from <code>spec.unsafeFlags</code> in <code>perconaxtradbclusters.pxc.percona.com.yaml</code>. <strong>Active</strong> means the flag is <code>true</code> (unsafe override on). <strong>Inactive</strong> is <code>false</code>. <strong>Not set</strong> means the key is absent.</p>`)
	if len(keys) == 0 {
		b.WriteString(`<p class="meta">No cluster rows.</p>`)
	}
	var emptyUnsafe unsafeFlagCluster
	for _, k := range keys {
		name, ns := splitRowPXCKey(k)
		uc, ok := unsafeBy[k]
		if !ok {
			uc = emptyUnsafe
			uc.Name = name
			uc.Namespace = ns
		}
		b.WriteString(`<div class="pxc-up-cluster"><div class="pxc-cluster-line">`)
		b.WriteString(`<span class="pxc-cluster-cr">`)
		b.WriteString(html.EscapeString(name))
		b.WriteString(`</span><span class="pxc-cluster-sep">·</span><span class="pxc-cluster-ns">`)
		b.WriteString(html.EscapeString(ns))
		b.WriteString(`</span></div>`)
		b.WriteString(renderUnsafeTabRowHTML(uc))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)

	// —— spec.pause ——
	b.WriteString(`<div class="pxc-up-block pxc-up-pause"><h3 class="pxc-up-subh3">PXC · spec.pause</h3>`)
	b.WriteString(`<p class="pxc-up-note">From <code>spec.pause</code> in each <code>PerconaXtraDBCluster</code> CR. <code>true</code> pauses reconciliation (highlighted in red). <code>false</code> is green.</p>`)
	if len(keys) == 0 {
		b.WriteString(`<p class="meta">No cluster rows.</p>`)
	} else {
		b.WriteString(`<table class="pxc-pause-table"><thead><tr>`)
		b.WriteString(`<th scope="col">CR name</th><th scope="col">Namespace</th><th scope="col">spec.pause</th>`)
		b.WriteString(`</tr></thead><tbody>`)
		for _, k := range keys {
			name, ns := splitRowPXCKey(k)
			pr, ok := pauseBy[k]
			if !ok {
				pr = pauseRow{Name: name, Namespace: ns}
			}
			b.WriteString(`<tr><td class="pxc-pause-cr">`)
			b.WriteString(html.EscapeString(name))
			b.WriteString(`</td><td class="pxc-pause-ns">`)
			b.WriteString(html.EscapeString(ns))
			b.WriteString(`</td><td class="pxc-pause-cell">`)
			b.WriteString(renderPauseBadgeHTML(pr))
			b.WriteString(`</td></tr>`)
		}
		b.WriteString(`</tbody></table>`)
	}
	b.WriteString(`</div></div>`)

	return b.String()
}
