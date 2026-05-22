# PubMed Agent — Frontend Developer Guide

## Overview

The PubMed Agent is a Go service that runs a Gemini-backed research agent accessible over a REST + SSE API. A React (or any) frontend authenticates with Supabase, passes the JWT to this service, and streams the agent's response in real time.

```
Browser  →  POST /v1/sessions                    →  { session_id }
         →  POST /v1/sessions/{id}/messages      →  SSE stream
         ←  data: {"type":"text_delta","content":"...","partial":true}
         ←  data: {"type":"text_delta","content":"...","partial":false}
         ←  data: {"type":"pdf_ready","download_url":"..."}
         ←  data: {"type":"done"}
```

---

## Auth

### Getting a JWT

1. Sign the user in via Supabase (email/password, magic link, OAuth, etc).
2. Call `supabase.auth.getSession()` — the `access_token` is the JWT.
3. Include it on every request as a Bearer token:

```http
Authorization: Bearer <access_token>
```

The service validates the token using your Supabase project's **JWT secret** (HS256). On first call the user profile is auto-provisioned in `user_profiles`.

### Dev mode (no Supabase)

If `SUPABASE_JWT_SECRET` is not set, auth is bypassed and a synthetic `dev-user` identity is used. All API endpoints work without a token.

---

## Session Lifecycle

```
POST   /v1/sessions                    → create session
POST   /v1/sessions/{id}/messages      → send message, receive SSE
GET    /v1/sessions/{id}/stream        → replay session history as SSE (page-reload hydration)
GET    /v1/sessions/{id}               → get session + event history (JSON)
GET    /v1/sessions                    → list all sessions for the user
DELETE /v1/sessions/{id}               → delete session
GET    /healthz                        → liveness probe
```

Typical flow:

```js
// 1. Create session
const { session_id } = await api.post('/v1/sessions');

// 2. Send a message
const es = new EventSource(/* see below */);
await api.postSSE(`/v1/sessions/${session_id}/messages`, { text: question });
```

---

## Sending Messages

`POST /v1/sessions/{id}/messages`

**Request headers:**
```
Authorization: Bearer <token>
Content-Type: application/json
```

### Text message

```json
{ "text": "What does the literature say about aspirin and colorectal cancer?" }
```

### Function response (HITL — answer to `ask_user`)

```json
{
  "function_responses": [
    {
      "name": "ask_user",
      "id": "<call_id from ask_user event>",
      "response": { "result": "Randomized controlled trials only" }
    }
  ]
}
```

**Response:** `text/event-stream` — a sequence of SSE events.

---

## SSE Event Reference

Every event is sent as `data: <JSON>\n\n`.

### `text_delta` — agent producing text

```json
{ "type": "text_delta", "content": "Low-dose aspirin...", "partial": true }
```

`partial: true` → streaming chunk; `partial: false` → final text for this turn.

### `ask_user` — agent waiting for user input (HITL)

```json
{ "type": "ask_user", "call_id": "abc123", "question": "Which study types should we focus on?", "options": ["RCTs only", "All study types"] }
```

Emitted instead of `text_delta` when the agent calls `ask_user`. See **ask_user Interaction Pattern** below.

### `pdf_ready` — PDF report generated

```json
{ "type": "pdf_ready", "download_url": "https://storage.googleapis.com/..." }
```

Emitted when `generate_pdf` completes, before the final `text_delta` that contains the download link.

### `done` — turn complete

```json
{ "type": "done" }
```

Emitted when the agent has finished. Close the EventSource at this point.

### `error` — unrecoverable error

```json
{ "type": "error", "message": "session not found" }
```

### Reading the stream in JavaScript

```js
async function sendMessage(sessionId, text, token, onChunk) {
  const resp = await fetch(`/v1/sessions/${sessionId}/messages`, {
    method: 'POST',
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ text }),
  });

  const reader = resp.body.getReader();
  const decoder = new TextDecoder();
  let buf = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });

    const events = buf.split('\n\n');
    buf = events.pop(); // last partial chunk

    for (const raw of events) {
      if (!raw.startsWith('data: ')) continue;
      const event = JSON.parse(raw.slice(6));
      if (event.type === 'text_delta') onChunk(event.content, event.partial);
      if (event.type === 'ask_user') onAskUser(event);   // { call_id, question, options }
      if (event.type === 'pdf_ready') onPDFReady(event); // { download_url }
      if (event.type === 'done') return;
      if (event.type === 'error') throw new Error(event.message);
    }
  }
}
```

---

## ask_user Interaction Pattern

When the agent needs clarification it emits a dedicated `ask_user` SSE event with
the `call_id`, `question`, and `options` fields populated directly in the stream.

**Detecting a HITL pause:**

```js
if (event.type === 'ask_user') {
  showOptions(event.question, event.options, (chosen) => {
    resumeSession(sessionId, event.call_id, chosen);
  });
}
```

**Resuming** — post a function response to the same session:

```js
async function resumeSession(sessionId, callId, answer) {
  await sendMessage(sessionId, null, token, onChunk, {
    function_responses: [{
      name: 'ask_user',
      id: callId,
      response: { result: answer },
    }],
  });
}
```

See **Sending Messages** above for the full function response request format.

---

## PDF Download

When the agent calls `generate_pdf`, the server emits a `pdf_ready` event **before** the
final `text_delta`:

```json
{ "type": "pdf_ready", "download_url": "https://storage.googleapis.com/..." }
```

The final `text_delta` also contains a markdown link `📄 [Download PDF report](...)` — use
whichever is more convenient for your UI.

- **GCS backend** (`PDF_GCS_BUCKET` set): URLs are public GCS object URLs. No expiry.
- **Local backend** (dev mode): URLs point to `http://localhost:<PDF_PORT>/reports/<file>`.

---

## Error Handling

| HTTP status | Meaning |
|-------------|---------|
| `401` | Missing or invalid JWT |
| `403` | Account disabled or plan limit reached |
| `404` | Session not found or belongs to another user |
| `400` | Invalid request body |
| `500` | Internal server error |

SSE errors arrive as `{ "type": "error", "message": "..." }` before the stream closes.

---

## Local Dev Setup

### Prerequisites

- Go 1.26+
- Chromium (for PDF generation — `brew install chromium`)
- A Supabase project (for auth + Postgres) or skip for dev mode

### Minimal config

```bash
# Required
export GOOGLE_API_KEY=<Gemini API key>
export NCBI_EMAIL=you@example.com

# Optional — omit for in-memory / no-auth dev mode
export SUPABASE_JWT_SECRET=<from Supabase project settings>
export SUPABASE_DB_URL=postgresql://postgres:<pass>@db.<ref>.supabase.co:5432/postgres

# Optional GCS PDF storage
export PDF_GCS_BUCKET=<project>-pubmed-pdfs
```

### Run

```bash
# ADK built-in web UI + CLI (default — great for interactive integration testing)
go run .
# open http://localhost:8080 in your browser

# Custom REST+SSE server (auth, Postgres, SSE API)
SERVER=true go run .
# Listening on :8080
```

### Supabase local emulator (optional)

```bash
npx supabase start
# Apply migrations
psql "$(npx supabase db url)" -f db/migrations/001_create_user_profiles.sql
psql "$(npx supabase db url)" -f db/migrations/002_create_sessions.sql
```

### Quick smoke test (dev mode, no token needed)

```bash
# Start custom server
SERVER=true go run .

# Create session
SESSION=$(curl -s -X POST http://localhost:8080/v1/sessions | jq -r .session_id)

# Send message (streams SSE)
curl -N -X POST http://localhost:8080/v1/sessions/$SESSION/messages \
  -H 'Content-Type: application/json' \
  -d '{"text":"What is aspirin used for?"}'

# Replay session history
curl -N http://localhost:8080/v1/sessions/$SESSION/stream
```

---

## Environment Variables Reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_API_KEY` | Yes | — | Gemini API key |
| `NCBI_EMAIL` | Yes | — | Email sent to NCBI with every request |
| `SUPABASE_JWT_SECRET` | No | — | HS256 secret; omit for dev mode (no auth) |
| `SUPABASE_DB_URL` | No | — | Postgres URL; omit for in-memory sessions |
| `CORS_ALLOWED_ORIGINS` | No | `*` | Comma-separated allowed origins |
| `PDF_GCS_BUCKET` | No | — | GCS bucket name; omit for local PDF files |
| `SERVER` | No | — | When set (any non-empty value), starts custom REST+SSE server; unset = ADK web UI |
| `PORT` | No | `8080` | HTTP listen port |
| `CONFIG_FILE` | No | `config.json` | Path to JSON config file |
| `PUBMED_AGENT_MODEL_DEFAULT` | No | `gemini-2.5-pro` | Default model spec |
| `PUBMED_AGENT_MODEL_ORCHESTRATOR` | No | — | Orchestrator model override |
| `PUBMED_AGENT_MODEL_VALIDATOR` | No | — | Validator model override |
| `PUBMED_AGENT_MODEL_PLANNER` | No | — | Planner model override |
