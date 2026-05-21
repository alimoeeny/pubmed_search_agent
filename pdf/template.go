package pdf

// fallbackTemplate is the built-in HTML/CSS template used when PDF_STYLE_PROMPT is
// not set or the LLM-generated template fails to parse.
//
// It uses Go html/template syntax. Required placeholders:
//
//	{{.Question}}         — research question string
//	{{.Date}}             — generation date (YYYY-MM-DD)
//	{{.BodyHTML}}         — pre-rendered HTML body (marked safe)
//	{{range .References}} — reference entries (.Number .Title .Journal .Date .URL .PMID)
const fallbackTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<style>
  @page {
    size: A4;
    margin: 2.2cm 2.5cm 2.5cm 2.5cm;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: Georgia, "Times New Roman", serif;
    font-size: 11pt;
    line-height: 1.65;
    color: #1a1a1a;
    background: #fff;
  }
  /* ── Global link styles ── */
  a {
    color: #0055aa;
    text-decoration: underline;
  }
  a:visited { color: #0055aa; }
  /* ── Print: force link visibility + print URL after each ref link ── */
  @media print {
    a[href] {
      color: #0055aa !important;
      text-decoration: underline !important;
    }
    .ref-body a[href]::after {
      content: " (" attr(href) ")";
      font-size: 7.5pt;
      color: #555;
      word-break: break-all;
      font-style: normal;
    }
  }
  /* ── Header ── */
  .report-header {
    background: #1a5f7a;
    color: #fff;
    padding: 18pt 0 14pt;
    margin-bottom: 20pt;
    border-radius: 3pt;
  }
  .report-header .label {
    font-family: Arial, Helvetica, sans-serif;
    font-size: 8pt;
    letter-spacing: 2pt;
    text-transform: uppercase;
    opacity: 0.8;
    padding: 0 18pt;
    display: block;
    margin-bottom: 4pt;
  }
  .report-header .question {
    font-size: 14pt;
    font-weight: bold;
    padding: 0 18pt;
    line-height: 1.35;
  }
  .report-header .meta {
    font-family: Arial, Helvetica, sans-serif;
    font-size: 8pt;
    opacity: 0.75;
    padding: 8pt 18pt 0;
  }
  /* ── Body ── */
  .body-section {
    margin-bottom: 20pt;
  }
  .body-section h1, .body-section h2, .body-section h3 {
    font-family: Arial, Helvetica, sans-serif;
    color: #1a5f7a;
    margin: 14pt 0 5pt;
  }
  .body-section h1 { font-size: 13pt; }
  .body-section h2 { font-size: 12pt; }
  .body-section h3 { font-size: 11pt; }
  .body-section p  { margin-bottom: 8pt; text-align: justify; }
  .body-section ul, .body-section ol {
    padding-left: 18pt;
    margin-bottom: 8pt;
  }
  .body-section li { margin-bottom: 3pt; }
  .body-section strong { font-weight: bold; }
  .body-section em    { font-style: italic; }
  /* citation superscripts */
  sup.cite {
    font-family: Arial, Helvetica, sans-serif;
    font-size: 7pt;
    font-weight: bold;
  }
  sup.cite a {
    color: #0055aa;
    text-decoration: none; /* superscripts don't need underline */
    font-weight: bold;
  }
  /* ── References ── */
  .references-section {
    border-top: 1.5pt solid #1a5f7a;
    margin-top: 22pt;
    padding-top: 12pt;
  }
  .references-section h2 {
    font-family: Arial, Helvetica, sans-serif;
    font-size: 11pt;
    color: #1a5f7a;
    margin-bottom: 10pt;
    text-transform: uppercase;
    letter-spacing: 1pt;
  }
  .ref-entry {
    display: flex;
    gap: 8pt;
    margin-bottom: 7pt;
    font-size: 9pt;
    line-height: 1.45;
  }
  .ref-num {
    flex-shrink: 0;
    font-weight: bold;
    color: #1a5f7a;
    min-width: 18pt;
  }
  .ref-body { flex: 1; }
  .ref-body a {
    color: #0055aa;
    text-decoration: underline;
    word-break: break-all;
  }
  /* ── Print color fidelity ── */
  @media print {
    .report-header { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
    .references-section { -webkit-print-color-adjust: exact; print-color-adjust: exact; }
  }
</style>
</head>
<body>

<div class="report-header">
  <span class="label">PubMed Research Report</span>
  <div class="question">{{.Question}}</div>
  <div class="meta">Generated {{.Date}} &nbsp;·&nbsp; Source: PubMed / NCBI</div>
</div>

<div class="body-section">
  {{.BodyHTML}}
</div>

{{if .References}}
<div class="references-section">
  <h2>References</h2>
  {{range .References}}
  <div class="ref-entry" id="ref-{{.Number}}">
    <span class="ref-num">[{{.Number}}]</span>
    <span class="ref-body">
      {{.Title}}. <em>{{.Journal}}</em>. {{.Date}}.
      <a href="{{.URL}}">PMID:&nbsp;{{.PMID}}</a>
    </span>
  </div>
  {{end}}
</div>
{{end}}

</body>
</html>`
