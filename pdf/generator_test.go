package pdf

import (
	"html/template"
	"os"
	"strings"
	"testing"

	"github.com/alimoeeny/pubmed_search_agent/storage"
)

func TestFallbackTemplateParsesAndExecutes(t *testing.T) {
	tmpl, err := template.New("fallback").Parse(fallbackTemplate)
	if err != nil {
		t.Fatalf("fallback template failed to parse: %v", err)
	}

	data := TemplateData{
		Question: "How long should anticoagulation continue after AF?",
		Date:     "2026-01-15",
		BodyHTML: template.HTML("<p>Summary body here.</p>"),
		References: []RefEntry{
			{Number: 1, PMID: "12345678", Title: "A study on AF", Journal: "NEJM", Date: "2024-01-01", URL: "https://pubmed.ncbi.nlm.nih.gov/12345678/"},
		},
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("template execute failed: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"How long should anticoagulation",
		"2026-01-15",
		"Summary body here",
		"A study on AF",
		"12345678",
		"NEJM",
		"pubmed.ncbi.nlm.nih.gov",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestConvertSummary_CitationsBecomeSuperscripts(t *testing.T) {
	md := "Some claim [PMID:11111111](https://pubmed.ncbi.nlm.nih.gov/11111111/) and another [PMID:22222222](https://pubmed.ncbi.nlm.nih.gov/22222222/)."
	articles := []ArticleRef{
		{PMID: "11111111", Title: "Paper One", Journal: "Lancet", Date: "2023-01-01"},
		{PMID: "22222222", Title: "Paper Two", Journal: "BMJ", Date: "2023-06-01"},
	}

	htmlBody, refs := convertSummary(md, articles)

	if !strings.Contains(htmlBody, `<sup`) {
		t.Errorf("expected superscript tags in HTML body, got: %s", htmlBody)
	}
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %d", len(refs))
	}
	if refs[0].PMID != "11111111" || refs[0].Number != 1 {
		t.Errorf("unexpected first ref: %+v", refs[0])
	}
	if refs[1].PMID != "22222222" || refs[1].Number != 2 {
		t.Errorf("unexpected second ref: %+v", refs[1])
	}
}

func TestConvertSummary_DuplicateCitationDeduped(t *testing.T) {
	md := "Claim A [PMID:99999999](https://pubmed.ncbi.nlm.nih.gov/99999999/) and claim B [PMID:99999999](https://pubmed.ncbi.nlm.nih.gov/99999999/)."
	htmlBody, refs := convertSummary(md, nil)

	if len(refs) != 1 {
		t.Errorf("expected 1 deduplicated ref, got %d", len(refs))
	}
	if strings.Count(htmlBody, "[1]") != 2 {
		t.Errorf("expected [1] to appear twice in body for the same PMID, got: %s", htmlBody)
	}
}

func TestConvertSummary_ReferenceSectionStripped(t *testing.T) {
	md := "Summary text.\n\n**References**\n- [12345678] Some paper\n"
	htmlBody, _ := convertSummary(md, nil)

	if strings.Contains(htmlBody, "References") {
		t.Errorf("agent References section should be stripped before rendering, got: %s", htmlBody)
	}
}

func TestBuildFilename(t *testing.T) {
	name := buildFilename("How long should anticoagulation continue?")
	if !strings.HasSuffix(name, ".pdf") {
		t.Errorf("filename should end in .pdf: %s", name)
	}
	if strings.Contains(name, " ") {
		t.Errorf("filename must not contain spaces: %s", name)
	}
	if strings.Contains(name, "?") {
		t.Errorf("filename must not contain special chars: %s", name)
	}
}

func TestStripMarkdownFences(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"```html\n<html></html>\n```", "<html></html>"},
		{"```\n<html></html>\n```", "<html></html>"},
		{"<html></html>", "<html></html>"},
	}
	for _, c := range cases {
		got := stripMarkdownFences(c.input)
		if got != c.want {
			t.Errorf("stripMarkdownFences(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestGeneratePDF_SkipWhenNoChrome(t *testing.T) {
	// Only run if Chrome/Chromium is available.
	_, err := findChrome()
	if err != nil {
		t.Skip("Chrome/Chromium not found in PATH, skipping PDF generation test")
	}

	outDir := t.TempDir()
	backend := &storage.LocalBackend{Dir: outDir, BaseURL: "http://localhost:8081"}
	gen := NewGenerator(nil) // no LLM needed; StylePrompt is empty
	result, err := gen.Generate(t.Context(), PDFRequest{
		Question: "Test question",
		Summary:  "Summary with [PMID:12345678](https://pubmed.ncbi.nlm.nih.gov/12345678/).",
		Articles: []ArticleRef{{PMID: "12345678", Title: "Test Paper", Journal: "Test J", Date: "2024"}},
		Backend:  backend,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	filePath := outDir + "/" + result.FilePath
	info, statErr := os.Stat(filePath)
	if statErr != nil {
		t.Fatalf("PDF file not found at %s: %v", filePath, statErr)
	}
	if info.Size() == 0 {
		t.Error("PDF file is empty")
	}
	if !strings.HasPrefix(result.DownloadURL, "http://localhost:8081/download/") {
		t.Errorf("unexpected download URL: %s", result.DownloadURL)
	}
}

// findChrome checks whether a Chrome-compatible binary is available.
func findChrome() (string, error) {
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	for _, c := range candidates {
		if path, err := os.Stat("/usr/bin/" + c); err == nil && !path.IsDir() {
			return "/usr/bin/" + c, nil
		}
		if path, err := os.Stat("/usr/local/bin/" + c); err == nil && !path.IsDir() {
			return "/usr/local/bin/" + c, nil
		}
	}
	// macOS: also check Applications
	macPaths := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, p := range macPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}
