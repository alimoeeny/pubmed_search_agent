package pdf

import "fmt"

// templateSystemPrompt is the instruction sent to the LLM when PDF_STYLE_PROMPT is set.
// It enforces the required Go html/template placeholders so the output can be parsed
// and executed with TemplateData.
const templateSystemPrompt = `You are an expert HTML/CSS designer specializing in print documents.
Generate a complete, self-contained HTML document (with all CSS embedded in <style> tags) for a PubMed medical research report PDF.

STYLE REQUIREMENTS:
%s

REQUIRED Go html/template placeholders — you MUST include ALL of these exactly as shown:
  {{.Question}}         — replaced with the research question string
  {{.Date}}             — replaced with the generation date (YYYY-MM-DD)
  {{.BodyHTML}}         — replaced with pre-rendered HTML content (use {{.BodyHTML}} directly — do NOT add extra escaping)
  {{range .References}} — loop over references; inside use: .Number .Title .Journal .Date .URL .PMID
  {{end}}               — closes the range loop

CONSTRAINTS:
- Self-contained: embed all CSS inline in <style> tags; no external URLs
- Print-optimized: use @media print and @page { size: A4; margin: 2cm }
- The BodyHTML section must render superscript citation numbers like [1], [2]
- Hyperlinks MUST be visually obvious: a { color: #0055aa; text-decoration: underline; }
- Include an @media print { a[href] { color: #0055aa !important; text-decoration: underline !important; } } rule
- For reference list links, add a CSS rule to print the URL as visible text: .ref-body a[href]::after { content: " (" attr(href) ")"; font-size: 7.5pt; color: #555; }
- Do NOT use pointer-events: none anywhere — all <a> tags must remain interactive
- Return ONLY the raw HTML — no markdown code fences, no explanation`

// buildTemplatePrompt returns the full LLM prompt for generating an HTML template
// from the given plain-English style description.
func buildTemplatePrompt(stylePrompt string) string {
	return fmt.Sprintf(templateSystemPrompt, stylePrompt)
}
