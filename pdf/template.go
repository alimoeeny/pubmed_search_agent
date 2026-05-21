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
    margin: 2cm 2.5cm 2.5cm 2.5cm;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: Georgia, "Times New Roman", serif;
    font-size: 10.5pt;
    line-height: 1.7;
    color: #1e1e1e;
    background: #f7f6f2;
  }
  @media print {
    body {
      background: #f7f6f2 !important;
      -webkit-print-color-adjust: exact;
      print-color-adjust: exact;
    }
  }
  a { color: #0055aa; text-decoration: underline; }
  a:visited { color: #0055aa; }
  @media print {
    a[href] { color: #0055aa !important; text-decoration: underline !important; }
    .ref-body a[href]::after {
      content: " (" attr(href) ")";
      font-size: 7pt;
      color: #8a8278;
      word-break: break-all;
      font-style: normal;
    }
  }
  .report-header {
    background: #1c2b3a;
    border-top: 2pt solid #c8401a;
    padding: 16pt 20pt 20pt;
    margin-bottom: 26pt;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }
  .report-header .tool-label {
    font-family: "Courier New", Courier, monospace;
    font-size: 7pt;
    letter-spacing: 3pt;
    text-transform: uppercase;
    color: #c8401a;
    display: block;
    margin-bottom: 10pt;
  }
  .report-header .question {
    font-family: Georgia, "Times New Roman", serif;
    font-size: 15pt;
    font-weight: bold;
    color: #fff;
    line-height: 1.3;
    margin-bottom: 10pt;
    text-transform: capitalize;
  }
  .report-header .meta {
    font-family: "Courier New", Courier, monospace;
    font-size: 7.5pt;
    color: #8a8278;
  }
  .body-section { margin-bottom: 20pt; }
  .body-section h1,
  .body-section h2,
  .body-section h3 {
    font-family: Georgia, "Times New Roman", serif;
    color: #1c2b3a;
    border-left: 3pt solid #c8401a;
    padding-left: 10pt;
    margin: 16pt 0 6pt;
    line-height: 1.3;
  }
  .body-section h1 { font-size: 13pt; }
  .body-section h2 { font-size: 12pt; }
  .body-section h3 { font-size: 11pt; }
  .body-section p { margin-bottom: 8pt; text-align: justify; padding-left: 13pt; padding-right: 13pt; }
  .body-section ul, .body-section ol { padding-left: 18pt; margin-bottom: 8pt; }
  .body-section li { margin-bottom: 3pt; }
  .body-section strong { font-weight: bold; }
  .body-section em    { font-style: italic; }
  sup.cite { font-family: "Courier New", Courier, monospace; font-size: 7pt; }
  sup.cite a { color: #c8401a; text-decoration: none; }
  .references-section {
    border-top: 1.5pt solid #c8401a;
    margin-top: 24pt;
    padding-top: 12pt;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }
  .ref-heading {
    font-family: "Courier New", Courier, monospace;
    font-size: 7pt;
    letter-spacing: 3pt;
    text-transform: uppercase;
    color: #c8401a;
    display: block;
    margin-bottom: 10pt;
  }
  .ref-entry {
    display: flex;
    gap: 10pt;
    padding: 6pt 0;
    font-size: 8.5pt;
    line-height: 1.45;
  }
  .ref-entry + .ref-entry { border-top: 0.5pt solid #ddd9d3; }
  .ref-num {
    flex-shrink: 0;
    font-family: "Courier New", Courier, monospace;
    font-size: 8pt;
    color: #c8401a;
    min-width: 24pt;
    text-align: right;
    padding-top: 1pt;
  }
  .ref-body { flex: 1; color: #1e1e1e; }
  .ref-body a { color: #0055aa; text-decoration: underline; word-break: break-all; }
</style>
</head>
<body>

<div class="report-header">
  <span class="tool-label">PubMed Signal</span>
  <div class="question">{{.Question}}</div>
  <div class="meta">Generated {{.Date}} &nbsp;·&nbsp; Source: PubMed / NCBI</div>
</div>

<div class="body-section">
  {{.BodyHTML}}
</div>

{{if .References}}
<div class="references-section">
  <span class="ref-heading">References</span>
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
