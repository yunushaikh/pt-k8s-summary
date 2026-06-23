package collector

import (
	"fmt"
	"html"
	"strings"
)

// psCollapsibleTable wraps table HTML in a collapsed-by-default details block with optional client filter.
func psCollapsibleTable(sectionID, title, meta, tableHTML, tbodyClass, filterPlaceholder string) string {
	if strings.TrimSpace(tableHTML) == "" {
		return ""
	}
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<details class="nodes-coll" id="`)
	b.WriteString(esc(sectionID))
	b.WriteString(`"><summary class="nodes-coll-sum" aria-label="Expand or collapse `)
	b.WriteString(esc(title))
	b.WriteString(`"><span class="nodes-coll-exp" aria-hidden="true"></span><span class="nodes-coll-sum-body"><strong class="nodes-coll-sum-h">`)
	b.WriteString(esc(title))
	b.WriteString(`</strong><span class="nodes-coll-sum-meta">`)
	b.WriteString(esc(meta))
	b.WriteString(`</span></span></summary><div class="nodes-coll-inner">`)
	if filterPlaceholder != "" && tbodyClass != "" {
		b.WriteString(`<div class="report-tbl-toolbar"><label>Filter <input type="search" class="report-tbl-filter" id="`)
		b.WriteString(esc(sectionID))
		b.WriteString(`-filter" placeholder="`)
		b.WriteString(esc(filterPlaceholder))
		b.WriteString(`" autocomplete="off" spellcheck="false"></label><span class="report-tbl-meta" id="`)
		b.WriteString(esc(sectionID))
		b.WriteString(`-visible"></span></div>`)
	}
	b.WriteString(`<div class="report-tbl-scroll">`)
	b.WriteString(tableHTML)
	b.WriteString(`</div></div></details>`)
	if filterPlaceholder != "" && tbodyClass != "" {
		b.WriteString(`<script>(function(){
  var sec=document.getElementById("`)
		b.WriteString(esc(sectionID))
		b.WriteString(`");
  if(!sec)return;
  var inp=sec.querySelector("#`)
		b.WriteString(esc(sectionID))
		b.WriteString(`-filter");
  var tbody=sec.querySelector(".`)
		b.WriteString(tbodyClass)
		b.WriteString(`");
  var vis=sec.querySelector("#`)
		b.WriteString(esc(sectionID))
		b.WriteString(`-visible");
  if(!inp||!tbody)return;
  var total=tbody.querySelectorAll("tr").length;
  function updateCount(n){ if(vis) vis.textContent=n===total?total+" shown":n+" of "+total+" shown"; }
  updateCount(total);
  inp.addEventListener("input",function(){
    var q=inp.value.trim().toLowerCase(),n=0;
    tbody.querySelectorAll("tr").forEach(function(tr){
      var show=q===""||tr.textContent.toLowerCase().indexOf(q)>=0;
      tr.style.display=show?"":"none";
      if(show)n++;
    });
    updateCount(n);
  });
})();</script>`)
	}
	return b.String()
}

func psTableOpen(className string) string {
	if className == "" {
		className = "pxc-cert-table"
	}
	return fmt.Sprintf(`<table class="%s"><thead>`, className)
}
