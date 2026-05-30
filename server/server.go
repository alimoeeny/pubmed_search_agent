// Package server implements the custom HTTP server for the PubMed agent.
// It exposes a REST + SSE API protected by Supabase JWT auth.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"google.golang.org/adk/agent"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"google.golang.org/adk/runner"

	"github.com/alimoeeny/pubmed_search_agent/server/authz"
	"github.com/alimoeeny/pubmed_search_agent/server/middleware"
	agenttools "github.com/alimoeeny/pubmed_search_agent/tools"
	"github.com/alimoeeny/pubmed_search_agent/user"
)

// Config holds all dependencies for the HTTP server.
type Config struct {
	AppName      string
	Runner       *runner.Runner
	SessionSvc   adksession.Service
	UserStore    user.Store // nil in dev mode (no Supabase)
	AuthzChecker authz.AuthorizationChecker
	SupabaseURL  string // e.g. https://<project>.supabase.co; used to derive JWKS endpoint
	CORSOrigins  string // comma-separated; empty = wildcard
}

// Server is the HTTP handler for the PubMed agent API.
type Server struct {
	cfg Config
}

// New creates a new Server from the given config.
func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Handler returns the root http.Handler.
// /health is public. All /v1/ routes require a valid Supabase JWT.
// Note: /healthz is intercepted by Google's *.run.app edge, so we use /health.
func (s *Server) Handler() http.Handler {
	apiMux := http.NewServeMux()
	s.registerRoutes(apiMux)

	var apiHandler http.Handler = apiMux
	if s.cfg.SupabaseURL != "" {
		jwksURL := s.cfg.SupabaseURL + "/auth/v1/.well-known/jwks.json"
		apiHandler = middleware.JWKSAuth(jwksURL)(apiHandler)
	}

	root := http.NewServeMux()
	root.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	root.Handle("/", apiHandler)

	return middleware.CORS(s.cfg.CORSOrigins)(root)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/sessions", s.handleCreateSession)
	mux.HandleFunc("GET /v1/sessions", s.handleListSessions)
	mux.HandleFunc("GET /v1/sessions/{id}", s.handleGetSession)
	mux.HandleFunc("DELETE /v1/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("POST /v1/sessions/{id}/messages", s.handlePostMessage)
	mux.HandleFunc("GET /v1/sessions/{id}/stream", s.handleStreamSession)
}

// ─── Route handlers ───────────────────────────────────────────────────────────

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	identity := s.requireIdentity(w, r)
	if identity == (middleware.UserIdentity{}) {
		return
	}
	if !s.authorizeRequest(w, r, identity) {
		return
	}

	resp, err := s.cfg.SessionSvc.Create(r.Context(), &adksession.CreateRequest{
		AppName: s.cfg.AppName,
		UserID:  identity.ID,
		State:   map[string]any{agenttools.SessionKeyNCBIEmail: identity.Email},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"session_id": resp.Session.ID()})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	identity := s.requireIdentity(w, r)
	if identity == (middleware.UserIdentity{}) {
		return
	}

	resp, err := s.cfg.SessionSvc.List(r.Context(), &adksession.ListRequest{
		AppName: s.cfg.AppName,
		UserID:  identity.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	type sessionSummary struct {
		ID          string `json:"session_id"`
		LastUpdated string `json:"last_updated"`
	}
	out := make([]sessionSummary, 0, len(resp.Sessions))
	for _, sess := range resp.Sessions {
		out = append(out, sessionSummary{
			ID:          sess.ID(),
			LastUpdated: sess.LastUpdateTime().Format("2006-01-02T15:04:05Z"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	identity := s.requireIdentity(w, r)
	if identity == (middleware.UserIdentity{}) {
		return
	}
	sessionID := r.PathValue("id")

	resp, err := s.cfg.SessionSvc.Get(r.Context(), &adksession.GetRequest{
		AppName:   s.cfg.AppName,
		UserID:    identity.ID,
		SessionID: sessionID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	type eventJSON struct {
		ID        string `json:"id"`
		Author    string `json:"author"`
		Timestamp string `json:"timestamp"`
		Text      string `json:"text,omitempty"`
	}
	var events []eventJSON
	for ev := range resp.Session.Events().All() {
		e := eventJSON{
			ID:        ev.ID,
			Author:    ev.Author,
			Timestamp: ev.Timestamp.Format("2006-01-02T15:04:05Z"),
		}
		if ev.Content != nil {
			for _, p := range ev.Content.Parts {
				if p.Text != "" {
					e.Text += p.Text
				}
			}
		}
		events = append(events, e)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":   resp.Session.ID(),
		"last_updated": resp.Session.LastUpdateTime().Format("2006-01-02T15:04:05Z"),
		"events":       events,
	})
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	identity := s.requireIdentity(w, r)
	if identity == (middleware.UserIdentity{}) {
		return
	}
	sessionID := r.PathValue("id")

	if err := s.cfg.SessionSvc.Delete(r.Context(), &adksession.DeleteRequest{
		AppName:   s.cfg.AppName,
		UserID:    identity.ID,
		SessionID: sessionID,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	identity := s.requireIdentity(w, r)
	if identity == (middleware.UserIdentity{}) {
		return
	}

	// c1: upsert user profile
	if !s.authorizeRequest(w, r, identity) {
		return
	}

	sessionID := r.PathValue("id")

	// c3: verify session exists and belongs to this user
	if _, err := s.cfg.SessionSvc.Get(r.Context(), &adksession.GetRequest{
		AppName:   s.cfg.AppName,
		UserID:    identity.ID,
		SessionID: sessionID,
	}); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Parse request body
	var req postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	msg, err := req.toContent()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// c4: run agent and stream SSE
	s.streamSSE(w, r, identity.ID, sessionID, msg)
}

// ─── SSE streaming ────────────────────────────────────────────────────────────

type sseEventType string

const (
	sseTypeTextDelta   sseEventType = "text_delta"
	sseTypeUserMessage sseEventType = "user_message"
	sseTypeAskUser     sseEventType = "ask_user"
	sseTypePDFReady    sseEventType = "pdf_ready"
	sseTypeDone        sseEventType = "done"
	sseTypeError       sseEventType = "error"
)

type ssePayload struct {
	Type        sseEventType `json:"type"`
	Content     string       `json:"content,omitempty"`
	Partial     bool         `json:"partial,omitempty"`
	CallID      string       `json:"call_id,omitempty"`
	Question    string       `json:"question,omitempty"`
	Options     []string     `json:"options,omitempty"`
	DownloadURL string       `json:"download_url,omitempty"`
	Message     string       `json:"message,omitempty"`
}

func (s *Server) streamSSE(w http.ResponseWriter, r *http.Request, userID, sessionID string, msg *genai.Content) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Session-ID", sessionID)

	send := func(p ssePayload) {
		b, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	// sentPartialText tracks whether we emitted any partial text chunks in the current
	// streaming sequence. Gemini's StreamingResponseAggregator emits both the incremental
	// chunks (partial=true) and a final accumulated blob (partial=false). The accumulated
	// blob is a duplicate of what we already streamed, so skip it when chunks were sent.
	sentPartialText := false
	for event, err := range s.cfg.Runner.Run(
		r.Context(),
		userID, sessionID, msg,
		agent.RunConfig{StreamingMode: agent.StreamingModeSSE},
	) {
		if err != nil {
			send(ssePayload{Type: sseTypeError, Message: err.Error()})
			return
		}
		if event == nil {
			continue
		}
		// ask_user HITL: long-running tool call — emit dedicated event and stop text emission.
		if len(event.LongRunningToolIDs) > 0 {
			emitAskUser(event, send)
			continue
		}
		if event.Content == nil {
			continue
		}
		// pdf_ready: FunctionResponse from generate_pdf
		emitPDFReady(event, send)
		// text_delta: streamed or final agent text
		for _, p := range event.Content.Parts {
			if p.Text == "" {
				continue
			}
			if event.Partial {
				send(ssePayload{Type: sseTypeTextDelta, Content: p.Text, Partial: true})
				sentPartialText = true
			} else if !sentPartialText {
				send(ssePayload{Type: sseTypeTextDelta, Content: p.Text, Partial: false})
			}
			// else: non-partial after partials → accumulated duplicate, skip
		}
		if !event.Partial {
			sentPartialText = false
		}
	}
	send(ssePayload{Type: sseTypeDone})
}

// handleStreamSession replays a session's past events as SSE, enabling page-reload hydration.
func (s *Server) handleStreamSession(w http.ResponseWriter, r *http.Request) {
	identity := s.requireIdentity(w, r)
	if identity == (middleware.UserIdentity{}) {
		return
	}
	sessionID := r.PathValue("id")

	resp, err := s.cfg.SessionSvc.Get(r.Context(), &adksession.GetRequest{
		AppName:   s.cfg.AppName,
		UserID:    identity.ID,
		SessionID: sessionID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(p ssePayload) {
		b, _ := json.Marshal(p)
		fmt.Fprintf(w, "data: %s\n\n", b)
		flusher.Flush()
	}

	for ev := range resp.Session.Events().All() {
		if ev.Author == "user" {
			if ev.Content != nil {
				for _, p := range ev.Content.Parts {
					if p.Text != "" {
						send(ssePayload{Type: sseTypeUserMessage, Content: p.Text})
					}
				}
			}
			continue
		}
		if len(ev.LongRunningToolIDs) > 0 {
			emitAskUser(ev, send)
			continue
		}
		if ev.Content == nil {
			continue
		}
		emitPDFReady(ev, send)
		for _, p := range ev.Content.Parts {
			if p.Text != "" {
				send(ssePayload{Type: sseTypeTextDelta, Content: p.Text, Partial: false})
			}
		}
	}
	send(ssePayload{Type: sseTypeDone})
}

// emitAskUser extracts ask_user args from a long-running tool-call event and sends the SSE payload.
func emitAskUser(event *adksession.Event, send func(ssePayload)) {
	if event.Content == nil {
		return
	}
	for _, p := range event.Content.Parts {
		if p.FunctionCall == nil || p.FunctionCall.Name != "ask_user" {
			continue
		}
		args := p.FunctionCall.Args
		question, _ := args["question"].(string)
		var options []string
		if raw, ok := args["options"].([]any); ok {
			for _, o := range raw {
				if s, ok := o.(string); ok {
					options = append(options, s)
				}
			}
		}
		send(ssePayload{
			Type:     sseTypeAskUser,
			CallID:   p.FunctionCall.ID,
			Question: question,
			Options:  options,
		})
		return
	}
}

// emitPDFReady scans an event for a generate_pdf FunctionResponse and sends pdf_ready if found.
func emitPDFReady(event *adksession.Event, send func(ssePayload)) {
	if event.Content == nil {
		return
	}
	for _, p := range event.Content.Parts {
		if p.FunctionResponse == nil || p.FunctionResponse.Name != "generate_pdf" {
			continue
		}
		url, _ := p.FunctionResponse.Response["download_url"].(string)
		if url != "" {
			send(ssePayload{Type: sseTypePDFReady, DownloadURL: url})
		}
		return
	}
}

// ─── Request parsing ──────────────────────────────────────────────────────────

type postMessageRequest struct {
	Text              string             `json:"text,omitempty"`
	FunctionResponses []functionResponse `json:"function_responses,omitempty"`
}

type functionResponse struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	Response any    `json:"response"`
}

func (req postMessageRequest) toContent() (*genai.Content, error) {
	if len(req.FunctionResponses) > 0 {
		parts := make([]*genai.Part, 0, len(req.FunctionResponses))
		for _, fr := range req.FunctionResponses {
			respMap, _ := fr.Response.(map[string]any)
			parts = append(parts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fr.Name,
					ID:       fr.ID,
					Response: respMap,
				},
			})
		}
		return &genai.Content{Role: "user", Parts: parts}, nil
	}
	if req.Text == "" {
		return nil, errors.New("text or function_responses is required")
	}
	return &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: req.Text}},
	}, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// requireIdentity extracts UserIdentity from context (set by auth middleware).
// In dev mode (no SupabaseURL configured), it returns a synthetic identity so
// local testing works without a real Supabase project.
func (s *Server) requireIdentity(w http.ResponseWriter, r *http.Request) middleware.UserIdentity {
	if s.cfg.SupabaseURL == "" {
		return middleware.UserIdentity{ID: "dev-user", Email: "dev@localhost"}
	}
	identity, ok := middleware.UserIdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing identity")
		return middleware.UserIdentity{}
	}
	return identity
}

// authorizeRequest upserts the user profile and runs the authorization check.
func (s *Server) authorizeRequest(w http.ResponseWriter, r *http.Request, identity middleware.UserIdentity) bool {
	var profile user.UserProfile
	if s.cfg.UserStore != nil {
		var err error
		profile, err = s.cfg.UserStore.Upsert(r.Context(), identity.ID, identity.Email)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load user profile")
			return false
		}
	} else {
		profile = user.UserProfile{
			UserID:  identity.ID,
			Email:   identity.Email,
			Plan:    user.PlanMax,
			Enabled: true,
		}
	}

	if err := s.cfg.AuthzChecker.Check(r.Context(), profile); err != nil {
		var az *authz.AuthzError
		if errors.As(err, &az) {
			writeError(w, az.Status, az.Message)
		} else {
			writeError(w, http.StatusForbidden, "authorization failed")
		}
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
