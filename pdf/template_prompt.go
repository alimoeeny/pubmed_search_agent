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
- All CSS must be embedded in <style> tags — no external stylesheet URLs, no Google Fonts URLs
- Print-optimized: include @page { size: A4; margin: 2cm } and @media print rules
- Superscript citation numbers in the body are already formatted as [1], [2] etc — style them attractively
- The references section must display each reference with its number, title, journal, date, and a link using .URL
- Return ONLY the raw HTML document — no markdown code fences, no backticks, no explanation`

// buildTemplatePrompt returns the full LLM prompt for generating an HTML template
// from the given plain-English style description.
func buildTemplatePrompt(stylePrompt string) string {
	return fmt.Sprintf(templateSystemPrompt, stylePrompt)
}
