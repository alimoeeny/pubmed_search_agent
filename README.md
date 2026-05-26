# PubMed Research Agent

A Go service that accepts a biomedical research question, searches PubMed via the NCBI E-utilities API, and returns a human-readable summary with inline PMID citations. Runs as an interactive ADK web UI locally, or as a REST + SSE HTTP server for frontend integration.

## Architecture

Single `llmagent` with seven tools:

| Tool | Purpose |
|------|---------|
| `validate_question` | Checks whether the question is researchable on PubMed |
| `plan_pubmed_query` | Generates a structured boolean query with MeSH terms and filters |
| `pubmed_search` | Runs esearch and returns PMIDs + total count |
| `pubmed_fetch_details` | Fetches abstracts and metadata via efetch |
| `review_summary` | Reviews the draft summary for citation coverage; triggers a gap-fill loop (≤2 rounds) |
| `ask_user` | Long-running HITL tool for clarifying vague or ambiguous questions |
| `generate_pdf` | Generates a polished PDF report from the summary |

After writing the initial summary the agent calls `review_summary`, which scores citation coverage and returns evidence gaps. If the verdict is `NEEDS_MORE_EVIDENCE` the agent fetches additional articles and re-synthesises (up to 2 rounds). A post-model callback (`pmid_guard`) strips any hallucinated `[PMID:N]` citations — only PMIDs returned by `pubmed_fetch_details` survive.

HTTP responses from NCBI are cached on disk for 7 days under `${XDG_CACHE_HOME:-$HOME/.cache}/pubmed_search_agent/v1/`.

Session state is persisted in Supabase Postgres when `SUPABASE_DB_URL` is set; otherwise in-memory (lost on restart).

## Requirements

- Go 1.26+
- Chromium (`brew install chromium` on macOS) — required for PDF generation
- A Google API key with Gemini access
- Terraform + gcloud CLI — required for deployment only

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_API_KEY` | **Yes** | — | Gemini API key |
| `NCBI_EMAIL` | **Yes** | — | Email sent to NCBI with every request (polite-access policy) |
| `SUPABASE_JWT_SECRET` | No | — | HS256 secret; omit for dev mode (no auth) |
| `SUPABASE_DB_URL` | No | — | Postgres connection URL; omit for in-memory sessions |
| `CORS_ALLOWED_ORIGINS` | No | `*` | Comma-separated allowed origins for the HTTP server |
| `PDF_GCS_BUCKET` | No | — | GCS bucket name for PDF storage; omit to store locally |
| `SERVER` | No | — | Set to any non-empty value to start the REST+SSE server; unset = ADK web UI |
| `PORT` | No | `8080` | HTTP listen port |
| `PUBMED_AGENT_MODEL_DEFAULT` | No | `gemini:gemini-3.5-flash` | Default model for all roles |
| `PUBMED_AGENT_MODEL_ORCHESTRATOR` | No | — | Model override for the orchestrator |
| `PUBMED_AGENT_MODEL_VALIDATOR` | No | — | Model override for `validate_question` |
| `PUBMED_AGENT_MODEL_PLANNER` | No | — | Model override for `plan_pubmed_query` |
| `PUBMED_CACHE_DISABLE` | No | — | Set to `1` to disable on-disk HTTP caching |

Model specs use the format `provider:model-id`, e.g. `gemini:gemini-2.5-pro`.

## Running Locally

### Mode 1 — ADK web UI (recommended for development and integration testing)

The default mode. Starts the ADK-built-in chat interface at `http://localhost:8080`. No Supabase needed.

```bash
export GOOGLE_API_KEY="your-key"
export NCBI_EMAIL="you@example.com"

go run . web api --sse-write-timeout=10m webui
# Open http://localhost:8080 in your browser
```

> **Note:** The `--sse-write-timeout` flag belongs to the `api` sublauncher and must appear immediately after `api`. The default is 2 minutes, which is tight for queries that trigger the review/gap-fill loop. `10m` is a comfortable value for local development.

### Mode 2 — Custom REST + SSE server

Used in production and for testing the API directly. Requires `SERVER` to be set.

```bash
export GOOGLE_API_KEY="your-key"
export NCBI_EMAIL="you@example.com"
export SERVER=true

go run .
# Listening on :8080

# Quick smoke test (no token needed in dev mode)
SESSION=$(curl -s -X POST http://localhost:8080/v1/sessions | jq -r .session_id)
curl -N -X POST http://localhost:8080/v1/sessions/$SESSION/messages \
  -H 'Content-Type: application/json' \
  -d '{"text":"What is aspirin used for?"}'
```

See [`docs/developer-guide.md`](docs/developer-guide.md) for the full REST + SSE API reference.

## Running Tests

```bash
go test ./...
```

## Example Session

```
You: What are the effects of aspirin on cardiovascular mortality after STEMI?

Agent: [validates question] ✓ researchable
       [plans query]        aspirin[MeSH] AND myocardial infarction[MeSH] AND mortality[MeSH]
       [searches PubMed]    48 results
       [fetches details]    top 20 articles
       [summarises]

**Aspirin and Post-STEMI Mortality**

Low-dose aspirin significantly reduces all-cause and cardiovascular mortality in patients
following ST-elevation myocardial infarction (STEMI). A large RCT demonstrated a 23%
relative risk reduction in 30-day mortality [PMID:12345678]. Meta-analyses confirm this
benefit across diverse populations [PMID:87654321].

**References**
- [12345678] Aspirin and Cardiovascular Events after STEMI — *N Engl J Med*, 2023-11-16
- [87654321] Meta-analysis of antiplatelet therapy in acute coronary syndromes — *Circulation*, 2022
```

## Project Structure

```
.
├── main.go                  # Agent wiring, runner, server/launcher startup
├── model_factory.go         # Per-role LLM factory with env-var overrides
├── config/                  # AppConfig loading and env-var overrides
├── guard/                   # PMID hallucination guard
├── pubmed/                  # NCBI E-utilities client, cache, XML parser
├── tools/                   # All ADK tools (validate, plan, search, details, ask_user, pdf)
├── pdf/                     # PDF generation (Chromium renderer + GCS/local backend)
├── storage/                 # StorageBackend interface (GCS + local implementations)
├── session/                 # Postgres-backed ADK session.Service
├── user/                    # User profile store (Postgres)
├── server/
│   ├── server.go            # Custom HTTP server — REST endpoints + SSE streaming
│   ├── authz/               # Authorization checker (per-plan request limits)
│   └── middleware/          # JWT auth + CORS middleware
├── db/migrations/           # SQL migrations for Supabase Postgres
├── infra/                   # Terraform — GCP resources (modules + prod/dev envs)
├── .github/workflows/       # GitHub Actions CI/CD pipeline
└── docs/
    ├── developer-guide.md   # REST + SSE API reference for frontend engineers
    └── deployment-guide.md  # Step-by-step first-deploy walkthrough
```

## Further Reading

- **[`docs/developer-guide.md`](docs/developer-guide.md)** — API reference, SSE event schema, auth, session lifecycle
- **[`docs/deployment-guide.md`](docs/deployment-guide.md)** — deploying to GCP Cloud Run from scratch
