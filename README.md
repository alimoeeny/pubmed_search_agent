# PubMed Research Agent

A Go ADK agent that accepts a biomedical research question, searches PubMed via the NCBI E-utilities API, and returns a human-readable summary with inline PMID citations.

## Architecture

Single `llmagent` with five tools wired in sequence:

| Tool | Purpose |
|------|---------|
| `validate_question` | Checks whether the question is researchable on PubMed |
| `plan_pubmed_query` | Generates a structured boolean query with MeSH terms and filters |
| `pubmed_search` | Runs esearch and returns PMIDs + total count |
| `pubmed_fetch_details` | Fetches abstracts and metadata via efetch |
| `ask_user` | Long-running HITL tool for clarifying vague or ambiguous questions |

A post-model callback (`pmid_guard`) strips any hallucinated `[PMID:N]` citations from the final summary — only PMIDs returned by `pubmed_fetch_details` survive.

HTTP responses from NCBI are cached on disk for 7 days under `${XDG_CACHE_HOME:-$HOME/.cache}/pubmed_search_agent/v1/`.

## Requirements

- Go 1.26+
- A Google API key with Gemini access

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `NCBI_EMAIL` | **yes** | — | Your email address (NCBI polite-access policy) |
| `GOOGLE_API_KEY` | **yes** | — | Google API key for Gemini |
| `PUBMED_AGENT_MODEL_ORCHESTRATOR` | no | `gemini:gemini-flash-latest` | Model spec for the orchestrator |
| `PUBMED_AGENT_MODEL_VALIDATOR` | no | falls back to `PUBMED_AGENT_MODEL_DEFAULT` | Model spec for `validate_question` |
| `PUBMED_AGENT_MODEL_PLANNER` | no | falls back to `PUBMED_AGENT_MODEL_DEFAULT` | Model spec for `plan_pubmed_query` |
| `PUBMED_AGENT_MODEL_DEFAULT` | no | `gemini:gemini-flash-latest` | Default for any unset role |
| `PUBMED_CACHE_DISABLE` | no | — | Set to `1` to disable on-disk HTTP caching |

Model specs use the format `provider:model-id`, e.g. `gemini:gemini-2.5-pro`.

## Running

```bash
export NCBI_EMAIL="you@example.com"
export GOOGLE_API_KEY="your-key"

# Interactive CLI
go run .

# Web UI
go run . web api webui
```

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
├── main.go                  # Agent wiring, env validation, PMID guard callback
├── model_factory.go         # Per-role LLM factory with env-var overrides
├── guard/
│   └── pmid_guard.go        # Hallucination guard — strips unverified PMIDs
├── pubmed/
│   ├── types.go             # Enums and domain types
│   ├── client.go            # NCBI E-utilities HTTP client (rate-limited, retrying)
│   ├── cache.go             # On-disk RoundTripper cache (7-day TTL)
│   └── xml.go               # efetch XML parser
└── tools/
    ├── validate.go          # validate_question tool
    ├── plan_query.go        # plan_pubmed_query tool
    ├── search.go            # pubmed_search tool
    ├── details.go           # pubmed_fetch_details tool
    ├── ask_user.go          # ask_user HITL tool
    └── llm_helper.go        # Shared LLM text-generation helper
```
