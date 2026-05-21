package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"

	adksession "google.golang.org/adk/session"

	"github.com/alimoeeny/pubmed_search_agent/config"
	"github.com/alimoeeny/pubmed_search_agent/guard"
	"github.com/alimoeeny/pubmed_search_agent/pubmed"
	agentsession "github.com/alimoeeny/pubmed_search_agent/session"
	"github.com/alimoeeny/pubmed_search_agent/storage"
	agenttools "github.com/alimoeeny/pubmed_search_agent/tools"
	"github.com/alimoeeny/pubmed_search_agent/user"
)

func main() {
	ctx := context.Background()

	// --- Load config (config.json overridden by env vars) ---
	cfgFile := os.Getenv("CONFIG_FILE")
	if cfgFile == "" {
		cfgFile = "config.json"
	}
	appCfg, err := config.LoadAppConfig(cfgFile)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	userCfg := appCfg.DefaultUser

	// UserConfigProvider — swap for a DB-backed implementation for per-user overrides.
	_ = config.NewStaticProvider(userCfg)

	// --- Session service: Postgres when SUPABASE_DB_URL is set, in-memory otherwise ---
	var sessionSvc adksession.Service
	var userStore user.Store
	if appCfg.SupabaseDBURL != "" {
		pgSvc, err := agentsession.NewPostgresService(ctx, appCfg.SupabaseDBURL)
		if err != nil {
			log.Fatalf("Failed to connect to Supabase Postgres: %v", err)
		}
		sessionSvc = pgSvc
		userStore = user.NewPostgresStore(pgSvc.Pool())
		log.Printf("Session storage: Supabase Postgres")
	} else {
		sessionSvc = adksession.InMemoryService()
		log.Printf("Session storage: in-memory (set SUPABASE_DB_URL for persistence)")
	}

	// --- Build per-role models ---
	orchestratorModel, err := ModelFor(ctx, RoleOrchestrator, userCfg)
	if err != nil {
		log.Fatalf("Failed to create orchestrator model: %v", err)
	}
	validatorModel, err := ModelFor(ctx, RoleValidator, userCfg)
	if err != nil {
		log.Fatalf("Failed to create validator model: %v", err)
	}
	plannerModel, err := ModelFor(ctx, RolePlanner, userCfg)
	if err != nil {
		log.Fatalf("Failed to create planner model: %v", err)
	}

	// --- Build NCBI client with caching ---
	crt, err := pubmed.NewCachingRoundTripper(http.DefaultTransport)
	if err != nil {
		log.Fatalf("Failed to create caching transport: %v", err)
	}
	pubmedClient := pubmed.NewClient(pubmed.ClientConfig{
		Email:      appCfg.NCBIEmail,
		HTTPClient: &http.Client{Transport: crt, Timeout: 30 * time.Second},
	})
	defer pubmedClient.Close()

	// --- Build tools ---
	validateTool, err := agenttools.NewValidateQuestionTool(validatorModel)
	if err != nil {
		log.Fatalf("Failed to create validate_question tool: %v", err)
	}
	planTool, err := agenttools.NewPlanPubmedQueryTool(plannerModel)
	if err != nil {
		log.Fatalf("Failed to create plan_pubmed_query tool: %v", err)
	}
	searchTool, err := agenttools.NewPubmedSearchTool(pubmedClient)
	if err != nil {
		log.Fatalf("Failed to create pubmed_search tool: %v", err)
	}
	detailsTool, err := agenttools.NewPubmedFetchDetailsTool(pubmedClient)
	if err != nil {
		log.Fatalf("Failed to create pubmed_fetch_details tool: %v", err)
	}
	askUserTool, err := agenttools.NewAskUserTool()
	if err != nil {
		log.Fatalf("Failed to create ask_user tool: %v", err)
	}

	// --- PDF storage backend: GCS in production, local filesystem in dev ---
	var pdfBackend storage.StorageBackend
	if appCfg.PDFGCSBucket != "" {
		pdfBackend, err = storage.NewGCSBackend(ctx, appCfg.PDFGCSBucket)
		if err != nil {
			log.Fatalf("Failed to create GCS storage backend: %v", err)
		}
		log.Printf("PDF storage: GCS bucket %q", appCfg.PDFGCSBucket)
	} else {
		pdfBackend = &storage.LocalBackend{
			Dir:     appCfg.PDFOutputDir,
			BaseURL: appCfg.PDFDownloadBaseURL,
		}
		log.Printf("PDF storage: local dir %q", appCfg.PDFOutputDir)
		// Start a simple file server to serve generated PDFs for download.
		if appCfg.PDFPort != "" {
			go func() {
				mux := http.NewServeMux()
				fs := http.FileServer(http.Dir(appCfg.PDFOutputDir))
				mux.Handle("/download/", http.StripPrefix("/download/", fs))
				log.Printf("PDF download server listening on :%s (serving %s)", appCfg.PDFPort, appCfg.PDFOutputDir)
				if serveErr := http.ListenAndServe(":"+appCfg.PDFPort, mux); serveErr != nil {
					log.Printf("WARN: PDF file server stopped: %v", serveErr)
				}
			}()
		}
	}

	// --- PDF report tool ---
	pdfTool, err := agenttools.NewGeneratePDFTool(agenttools.PDFToolConfig{
		Backend:     pdfBackend,
		StylePrompt: userCfg.PDFStylePrompt,
		LLM:         orchestratorModel,
	})
	if err != nil {
		log.Fatalf("Failed to create generate_pdf tool: %v", err)
	}

	// --- Citation verifier + PMID guard: AfterModelCallback ---
	// On every model turn:
	//   1. Retrieve the set of PMIDs fetched this session from session state.
	//   2. For any unknown PMID citation: fetch real metadata from NCBI,
	//      ask the LLM to correct or remove the citation (up to 3 attempts),
	//      then fall back to a hallucination warning marker.
	//   3. Run the fast strip guard as a final safety net.
	verifier := guard.NewVerifier(pubmedClient, orchestratorModel)

	pmidGuardCallback := func(cbCtx agent.CallbackContext, resp *model.LLMResponse, respErr error) (*model.LLMResponse, error) {
		if resp == nil || resp.Content == nil {
			return nil, nil
		}
		// Only inspect text-bearing turns (skip pure tool-call turns).
		hasText := false
		for _, part := range resp.Content.Parts {
			if part.Text != "" {
				hasText = true
				break
			}
		}
		if !hasText {
			return nil, nil
		}

		// Retrieve allowed PMID set from session state.
		raw, _ := cbCtx.State().Get(agenttools.SessionKeyFetchedPMIDs)
		allowed := guard.PMIDSet(toStringSlice(raw))

		// Stage 1: verification + LLM correction loop for unknown PMIDs.
		// Operate on each text part independently.
		newParts := make([]*genai.Part, 0, len(resp.Content.Parts))
		changed := false
		for _, part := range resp.Content.Parts {
			if part.Text == "" {
				newParts = append(newParts, part)
				continue
			}
			fixed, err := verifier.FixCitations(ctx, part.Text, allowed)
			if err != nil {
				log.Printf("WARN: citation verifier error: %v", err)
				fixed = part.Text
			}
			if fixed != part.Text {
				changed = true
				newParts = append(newParts, &genai.Part{Text: fixed})
			} else {
				newParts = append(newParts, part)
			}
		}

		var workingContent *genai.Content
		if changed {
			workingContent = &genai.Content{Role: resp.Content.Role, Parts: newParts}
		} else {
			workingContent = resp.Content
		}

		// Stage 2: fast strip guard as final safety net.
		sanitized := guard.StripHallucinatedPMIDs(workingContent, allowed)
		if sanitized == resp.Content {
			return nil, nil // unchanged
		}
		return &model.LLMResponse{Content: sanitized}, nil
	}

	// --- Build agent ---
	researchAgent, err := llmagent.New(llmagent.Config{
		Name:        "pubmed_research_agent",
		Model:       orchestratorModel,
		Description: "A research assistant that searches PubMed and produces evidence-based summaries with citations.",
		Instruction: agentInstruction,
		Tools: []tool.Tool{
			validateTool,
			planTool,
			searchTool,
			detailsTool,
			askUserTool,
			pdfTool,
		},
		AfterModelCallbacks: []llmagent.AfterModelCallback{pmidGuardCallback},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	launchCfg := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(researchAgent),
		SessionService: sessionSvc,
	}
	_ = userStore // used in Phase 5 HTTP handler

	l := full.NewLauncher()
	if err = l.Execute(ctx, launchCfg, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

// toStringSlice safely converts a session-state value to []string.
func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	return nil
}

const agentInstruction = `You are a PubMed research assistant. Follow these steps for every research question:

1. VALIDATE: Call validate_question with the user's question.
   - If is_researchable is false, call ask_user with the suggested_refinements as options.
   - Do NOT proceed to searching until the question is researchable.

2. PLAN: Call plan_pubmed_query with the (refined) question.
   - On retry, pass prior_attempts and prior_result_counts so the query can be improved.

3. SEARCH: Call pubmed_search with the query plan.
   - If total_count == 0 or < 3: broaden the query — call plan_pubmed_query again with
     the failed attempt and retry. Maximum 3 attempts.
   - If total_count > 200: narrow the query — add MeSH terms, study-type filters, or
     date constraints, then retry.
   - After 3 failed attempts, call ask_user to get clarification before trying again.

4. FETCH: Call pubmed_fetch_details on the top PMIDs (up to 20).

5. SUMMARIZE: Write a concise human-readable answer grounded in the fetched abstracts.
   - Cite every factual claim inline as [PMID:XXXXXXXX](https://pubmed.ncbi.nlm.nih.gov/XXXXXXXX/)
     (replace XXXXXXXX with the actual PMID number in both places).
   - End with a "**References**" section listing each cited paper as:
     - [[XXXXXXXX](https://pubmed.ncbi.nlm.nih.gov/XXXXXXXX/)] Title — *Journal*, YYYY-MM-DD
   - Do NOT invent PMIDs, titles, or findings. Only cite PMIDs returned by pubmed_fetch_details.
   - If the abstracts do not contain enough information to answer the question, say so clearly.

6. REPORT: After delivering the summary, ALWAYS call generate_pdf with:
   - question: the original research question (verbatim)
   - summary: your full markdown summary text (including the References section)
   - articles: the list of articles returned by pubmed_fetch_details, each with pmid, title, journal, and date
   After generate_pdf returns, include the download link in your response as:
   📄 [Download PDF report](<download_url>)

Format rules:
- Use **bold** for the summary header.
- Keep the summary concise (300–500 words unless the question requires more depth).
- The References section must list every PMID cited in the summary.`

// Ensure fmt is used.
var _ = fmt.Sprintf
