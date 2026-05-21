package pdf

import "fmt"

// templateSystemPrompt is the instruction sent to the LLM when PDF_STYLE_PROMPT is set.
// It enforces the required Go html/template placeholders so the output can be parsed
// and executed with TemplateData.
const templateSystemPrompt = `You are an expert HTML/CSS designer specializing in beautiful, print-ready research documents.
Generate a complete, self-contained HTML document (with all CSS embedded in <style> tags) for a PubMed medical research report PDF.

HOUSE STYLE — "Signal":
This document uses a distinctive visual identity called "Signal" — the look of a well-funded research lab's internal memo: precise, confident, and rigorous. Follow it exactly unless the style override below specifies otherwise.

Color palette:
  Background:  #f7f6f2  — warm off-white page and body background
  Primary:     #1c2b3a  — header block, heading text
  Accent:      #c8401a  — rule lines, left-border on headings, citation superscripts, reference numbers
  Body text:   #1e1e1e  — all prose
  Muted:       #8a8278  — meta lines, journal names, secondary labels

Typography:
  Body prose:   Georgia, "Times New Roman", serif — 10.5pt, line-height 1.7
  Headings:     Georgia, "Times New Roman", serif — same family as body for visual coherence
  Tool label:   "Courier New", Courier, monospace — 7pt, letter-spacing 3pt, uppercase
  References:   Georgia, "Times New Roman", serif — 8.5pt, line-height 1.45
  Cite numbers: "Courier New", Courier, monospace — 8pt

Header block (full-width #1c2b3a background, no border-radius):
  - A 2pt burnt-sienna (#c8401a) horizontal rule at the very top of the header
  - "PUBMED SIGNAL" label in monospace caps, 7pt, letter-spacing 3pt, color #c8401a
  - Research question: bold, 15pt, white, Georgia serif, line-height 1.3, text-transform: capitalize
  - Date and source: 7.5pt monospace, color #8a8278
  - Padding: 16pt top, 20pt sides, 20pt bottom
  - Use -webkit-print-color-adjust: exact and print-color-adjust: exact

Body headings (h2, h3):
  - Font: Georgia, "Times New Roman", serif; font-weight: bold; color: #1c2b3a
  - border-left: 3pt solid #c8401a; padding-left: 10pt
  - No background, no underline; margin: 16pt above, 6pt below

Body paragraphs:
  - padding-left: 13pt (aligns text with heading text after its border + padding)

Citation superscripts (sup.cite a):
  - Font: "Courier New", Courier, monospace — 7pt; color: #c8401a; no underline

References section:
  - 1.5pt top border in #c8401a
  - "REFERENCES" label: monospace, 7pt, letter-spacing 3pt, uppercase, color #c8401a
  - Each entry: 24pt-wide right-aligned monospace number in #c8401a, then citation in Georgia 8.5pt
  - A 0.5pt #ddd9d3 horizontal rule between entries (border-top on entries after the first)

Background:
  - body { background: #f7f6f2 }
  - @media print { body { background: #f7f6f2 !important; -webkit-print-color-adjust: exact; print-color-adjust: exact; } }

STYLE OVERRIDE (applied on top of house style — may be empty):
%s

REQUIRED Go html/template placeholders — you MUST include ALL of these exactly as shown:
  {{.Question}}         — replaced with the research question string
  {{.Date}}             — replaced with the generation date (YYYY-MM-DD)
  {{.BodyHTML}}         — replaced with pre-rendered HTML content (use {{.BodyHTML}} directly — do NOT add extra escaping)
  {{range .References}} — loop over references; inside use: .Number .Title .Journal .Date .URL .PMID
  {{end}}               — closes the range loop

TECHNICAL CONSTRAINTS:
- Self-contained: embed all CSS inline in <style> tags; no external URLs
- Print-optimized: @page { size: A4; margin: 2cm 2.5cm 2.5cm 2.5cm }
- The BodyHTML section renders superscript citation numbers — style them with the accent color
- Hyperlinks: a { color: #0055aa; text-decoration: underline; }
- Include @media print { a[href] { color: #0055aa !important; text-decoration: underline !important; } }
- Reference list URLs printed inline: .ref-body a[href]::after { content: " (" attr(href) ")"; font-size: 7pt; color: #8a8278; word-break: break-all; }
- Do NOT use pointer-events: none anywhere — all <a> tags must remain interactive
- Return ONLY the raw HTML — no markdown code fences, no explanation`

// buildTemplatePrompt returns the full LLM prompt for generating an HTML template
// from the given plain-English style description.
func buildTemplatePrompt(stylePrompt string) string {
	return fmt.Sprintf(templateSystemPrompt, stylePrompt)
}
