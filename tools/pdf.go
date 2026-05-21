package tools

import (
	"fmt"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	agentpdf "github.com/alimoeeny/pubmed_search_agent/pdf"
)

// PDFToolConfig holds runtime configuration for the generate_pdf tool.
type PDFToolConfig struct {
	OutDir          string // directory to write PDFs; created if absent
	BaseDownloadURL string // base URL for download links, e.g. http://localhost:8081
	StylePrompt     string // plain-English PDF style description; "" = built-in default
	LLM             model.LLM
}

// GeneratePDFArgs is the input schema for the generate_pdf tool.
type GeneratePDFArgs struct {
	Question string              `json:"question"` // the original research question
	Summary  string              `json:"summary"`  // full markdown summary including References section
	Articles []agentpdf.ArticleRef `json:"articles"` // articles fetched during the session
}

// GeneratePDFResult is the output schema for the generate_pdf tool.
type GeneratePDFResult struct {
	FilePath    string `json:"file_path"`
	DownloadURL string `json:"download_url"`
	Message     string `json:"message"`
}

// NewGeneratePDFTool creates the generate_pdf ADK tool.
func NewGeneratePDFTool(cfg PDFToolConfig) (tool.Tool, error) {
	gen := agentpdf.NewGenerator(cfg.LLM)

	handler := func(ctx tool.Context, args GeneratePDFArgs) (GeneratePDFResult, error) {
		if args.Question == "" {
			return GeneratePDFResult{}, fmt.Errorf("generate_pdf: question must not be empty")
		}
		if args.Summary == "" {
			return GeneratePDFResult{}, fmt.Errorf("generate_pdf: summary must not be empty")
		}

		result, err := gen.Generate(ctx, agentpdf.PDFRequest{
			Question:    args.Question,
			Summary:     args.Summary,
			Articles:    args.Articles,
			OutDir:      cfg.OutDir,
			StylePrompt: cfg.StylePrompt,
		}, cfg.BaseDownloadURL)
		if err != nil {
			return GeneratePDFResult{}, fmt.Errorf("generate_pdf: %w", err)
		}

		return GeneratePDFResult{
			FilePath:    result.FilePath,
			DownloadURL: result.DownloadURL,
			Message:     fmt.Sprintf("PDF report saved to %s", result.FilePath),
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:        "generate_pdf",
		Description: "Generate a polished PDF research report from the summary and article list. Call this after delivering the final summary.",
	}, handler)
}
