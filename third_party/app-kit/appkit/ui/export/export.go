// Package export renders an opt-in, client-side export toolbar as a single
// inline <script>: "Save as PDF" (one continuous, page-break-free sheet sized
// to the content, with theme colors preserved) and "Save HTML" (a
// self-contained copy of the page). It is the kit's answer to "make this page
// shareable offline" — the recurring need behind every visual design doc and
// every dashboard a stakeholder wants to email.
//
// It is deliberately the one kit component that ships JS. No Node is added (D2
// is about the build, not runtime), and it introduces no shell-specific binding
// (D5): the script is plain page JS — window.print, a Blob download — so it is
// loopback-safe. The toolbar injects its OWN minimal inline styles (it does not
// use Tailwind role utilities), so a single call works in a standalone,
// self-contained HTML doc AND inside a themed kit page. It hides itself in the
// print output and forces print-color-adjust so dark themes render in the PDF.
//
// Usage: append Toolbar(Options{}) to a page's <body> (before </body>). The
// zero Options yields both buttons at sensible defaults.
package export

import (
	"fmt"
	"regexp"
	"strings"
)

// Options configures the export toolbar. The zero value is valid (both buttons,
// kit defaults) — see norm.
type Options struct {
	PDF         bool   // include "Save as PDF"
	HTML        bool   // include "Save HTML"
	PageWidthPx int    // PDF page width in px (the content is centered in it); default 1160
	Accent      string // hover accent as a #hex color; default the kit accent
	FileName    string // base download name (no extension); default = slug of document.title
}

const (
	defaultAccent = "#4fd1c9"
	defaultWidth  = 1160
)

var (
	hexRe  = regexp.MustCompile(`^#[0-9a-fA-F]{3,8}$`)
	slugRe = regexp.MustCompile(`[^a-z0-9]+`)
)

// norm fills defaults and sanitizes untrusted inputs so nothing the caller
// passes can break out of the generated JS (accent must be a bare hex; the
// filename is reduced to a slug). If neither button is requested, both are
// enabled (an empty toolbar is never what the caller meant).
func (o Options) norm() Options {
	if !o.PDF && !o.HTML {
		o.PDF, o.HTML = true, true
	}
	if o.PageWidthPx <= 0 {
		o.PageWidthPx = defaultWidth
	}
	if !hexRe.MatchString(o.Accent) {
		o.Accent = defaultAccent
	}
	o.FileName = slugRe.ReplaceAllString(strings.ToLower(o.FileName), "-")
	o.FileName = strings.Trim(o.FileName, "-")
	return o
}

// Toolbar returns a single inline <script>. On load it injects a small fixed
// top-right toolbar with the requested export buttons.
//
//   - "Save as PDF" momentarily fixes the layout width, measures the rendered
//     height, sets a single @page sized to exactly that, and calls print — so
//     the result is ONE continuous sheet with no page breaks. print-color-adjust
//     is forced so dark backgrounds and chart colors survive. (Custom @page
//     sizing is best supported in Chromium browsers.)
//   - "Save HTML" downloads a self-contained copy of the current document (the
//     toolbar itself is excluded from the saved markup).
//
// The toolbar is hidden in the print/PDF output. Output is deterministic.
func Toolbar(o Options) string {
	o = o.norm()

	var buttons strings.Builder
	if o.PDF {
		buttons.WriteString(`mkbtn('⤓ Save as PDF',savePDF);`)
	}
	if o.HTML {
		buttons.WriteString(`mkbtn('⤓ Save HTML',saveHTML);`)
	}

	dl := `(document.title||'page').replace(/[^a-z0-9]+/gi,'-').replace(/^-|-$/g,'').toLowerCase()+'.html'`
	if o.FileName != "" {
		dl = fmt.Sprintf("%q+'.html'", o.FileName)
	}

	js := jsTemplate
	js = strings.ReplaceAll(js, "__AC__", fmt.Sprintf("%q", o.Accent))
	js = strings.ReplaceAll(js, "__W__", fmt.Sprintf("%d", o.PageWidthPx))
	js = strings.ReplaceAll(js, "__BUTTONS__", buttons.String())
	js = strings.ReplaceAll(js, "__DL__", dl)
	return "<script>\n" + js + "\n</script>"
}

// jsTemplate is the toolbar logic. Tokens __AC__ (accent, quoted), __W__ (page
// width int), __BUTTONS__ (button wiring), and __DL__ (download-name expr) are
// substituted by Toolbar. It uses only standard DOM APIs (loopback-safe, D5).
const jsTemplate = `(function(){
  var AC=__AC__,W=__W__;
  var s=document.createElement('style');
  s.textContent='@media print{.__bar{display:none!important}html,body{-webkit-print-color-adjust:exact!important;print-color-adjust:exact!important}}'
    +'.__bar{position:fixed;top:14px;right:16px;z-index:9999;display:flex;gap:8px;font-family:ui-monospace,Menlo,monospace}'
    +'.__bar button{font-size:11px;letter-spacing:.04em;color:#e3f3f0;background:rgba(12,26,26,.92);border:1px solid #2a4a46;border-radius:8px;padding:7px 11px;cursor:pointer}'
    +'.__bar button:hover{border-color:'+AC+';color:'+AC+'}';
  document.head.appendChild(s);
  var bar=document.createElement('div');bar.className='__bar';
  function mkbtn(label,fn){var b=document.createElement('button');b.textContent=label;b.onclick=fn;bar.appendChild(b);return b;}
  function savePDF(){
    var b=document.body,prev=b.style.width;b.style.width=W+'px';
    var H=Math.ceil(b.scrollHeight)+24;
    var ps=document.getElementById('__ps')||document.createElement('style');ps.id='__ps';
    ps.textContent='@media print{@page{size:'+W+'px '+H+'px;margin:0}}';
    document.head.appendChild(ps);
    window.print();
    setTimeout(function(){b.style.width=prev;},300);
  }
  function saveHTML(){
    bar.remove();
    var doc='<!DOCTYPE html>\n'+document.documentElement.outerHTML;
    document.body.appendChild(bar);
    var a=document.createElement('a');
    a.href=URL.createObjectURL(new Blob([doc],{type:'text/html'}));
    a.download=__DL__;
    document.body.appendChild(a);a.click();a.remove();
    setTimeout(function(){URL.revokeObjectURL(a.href);},2000);
  }
  __BUTTONS__
  document.body.appendChild(bar);
})();`
