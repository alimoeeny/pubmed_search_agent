// Package session provides a Supabase Postgres-backed implementation of the
// ADK session.Service interface.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"google.golang.org/adk/model"
	adksession "google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

// ─── PostgresService ─────────────────────────────────────────────────────────

// PostgresService implements adksession.Service backed by Supabase Postgres.
type PostgresService struct {
	pool *pgxpool.Pool
}

// NewPostgresService creates a new PostgresService and verifies connectivity.
func NewPostgresService(ctx context.Context, connURL string) (*PostgresService, error) {
	pool, err := pgxpool.New(ctx, connURL)
	if err != nil {
		return nil, fmt.Errorf("session postgres: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("session postgres: ping: %w", err)
	}
	return &PostgresService{pool: pool}, nil
}

// ─── session.Service interface ───────────────────────────────────────────────

// Create implements adksession.Service.
func (s *PostgresService) Create(ctx context.Context, req *adksession.CreateRequest) (*adksession.CreateResponse, error) {
	if req.AppName == "" || req.UserID == "" {
		return nil, fmt.Errorf("session postgres: app_name and user_id are required")
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	initState := req.State
	if initState == nil {
		initState = make(map[string]any)
	}

	stateJSON, err := json.Marshal(initState)
	if err != nil {
		return nil, fmt.Errorf("session postgres: marshal initial state: %w", err)
	}

	const q = `
		INSERT INTO pubmed_sessions (session_id, app_name, user_id, state)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`

	if _, err := s.pool.Exec(ctx, q, sessionID, req.AppName, req.UserID, stateJSON); err != nil {
		return nil, fmt.Errorf("session postgres: create: %w", err)
	}

	sess := &pgSession{
		sid:       sessionID,
		appName:   req.AppName,
		userID:    req.UserID,
		state:     initState,
		evs:       nil,
		updatedAt: time.Now(),
	}
	return &adksession.CreateResponse{Session: sess}, nil
}

// Get implements adksession.Service.
func (s *PostgresService) Get(ctx context.Context, req *adksession.GetRequest) (*adksession.GetResponse, error) {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return nil, fmt.Errorf("session postgres: app_name, user_id, session_id are required")
	}

	const q = `
		SELECT state, events, updated_at
		FROM pubmed_sessions
		WHERE app_name = $1 AND user_id = $2 AND session_id = $3`

	var stateJSON, eventsJSON []byte
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, q, req.AppName, req.UserID, req.SessionID).
		Scan(&stateJSON, &eventsJSON, &updatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("session postgres: session %s not found", req.SessionID)
		}
		return nil, fmt.Errorf("session postgres: get: %w", err)
	}

	state, err := unmarshalState(stateJSON)
	if err != nil {
		return nil, fmt.Errorf("session postgres: unmarshal state: %w", err)
	}

	allEvents, err := unmarshalEvents(eventsJSON)
	if err != nil {
		return nil, fmt.Errorf("session postgres: unmarshal events: %w", err)
	}

	// Apply NumRecentEvents and After filters in memory.
	filtered := allEvents
	if req.NumRecentEvents > 0 && len(filtered) > req.NumRecentEvents {
		filtered = filtered[len(filtered)-req.NumRecentEvents:]
	}
	if !req.After.IsZero() {
		start := 0
		for i, e := range filtered {
			if !e.Timestamp.Before(req.After) {
				start = i
				break
			}
		}
		filtered = filtered[start:]
	}

	sess := &pgSession{
		sid:       req.SessionID,
		appName:   req.AppName,
		userID:    req.UserID,
		state:     state,
		evs:       filtered,
		updatedAt: updatedAt,
	}
	return &adksession.GetResponse{Session: sess}, nil
}

// List implements adksession.Service.
func (s *PostgresService) List(ctx context.Context, req *adksession.ListRequest) (*adksession.ListResponse, error) {
	if req.AppName == "" {
		return nil, fmt.Errorf("session postgres: app_name is required")
	}

	const q = `
		SELECT session_id, user_id, state, updated_at
		FROM pubmed_sessions
		WHERE app_name = $1 AND user_id = $2
		ORDER BY updated_at DESC`

	rows, err := s.pool.Query(ctx, q, req.AppName, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("session postgres: list: %w", err)
	}
	defer rows.Close()

	var sessions []adksession.Session
	for rows.Next() {
		var sessionID, userID string
		var stateJSON []byte
		var updatedAt time.Time
		if err := rows.Scan(&sessionID, &userID, &stateJSON, &updatedAt); err != nil {
			return nil, fmt.Errorf("session postgres: list scan: %w", err)
		}
		state, err := unmarshalState(stateJSON)
		if err != nil {
			return nil, fmt.Errorf("session postgres: list unmarshal state: %w", err)
		}
		sessions = append(sessions, &pgSession{
			sid:       sessionID,
			appName:   req.AppName,
			userID:    userID,
			state:     state,
			updatedAt: updatedAt,
		})
	}
	return &adksession.ListResponse{Sessions: sessions}, nil
}

// Delete implements adksession.Service.
func (s *PostgresService) Delete(ctx context.Context, req *adksession.DeleteRequest) error {
	if req.AppName == "" || req.UserID == "" || req.SessionID == "" {
		return fmt.Errorf("session postgres: app_name, user_id, session_id are required")
	}
	const q = `DELETE FROM pubmed_sessions WHERE app_name = $1 AND user_id = $2 AND session_id = $3`
	if _, err := s.pool.Exec(ctx, q, req.AppName, req.UserID, req.SessionID); err != nil {
		return fmt.Errorf("session postgres: delete: %w", err)
	}
	return nil
}

// AppendEvent implements adksession.Service.
func (s *PostgresService) AppendEvent(ctx context.Context, sess adksession.Session, event *adksession.Event) error {
	if event == nil || event.Partial {
		return nil
	}

	// Apply state delta in memory.
	if pg, ok := sess.(*pgSession); ok {
		pg.mu.Lock()
		applyStateDelta(pg.state, event.Actions.StateDelta)
		pg.evs = append(pg.evs, event)
		pg.updatedAt = event.Timestamp
		pg.mu.Unlock()
	}

	// Serialize event for persistence.
	evJSON, err := marshalEvent(event)
	if err != nil {
		return fmt.Errorf("session postgres: marshal event: %w", err)
	}

	// Serialize updated state.
	var stateJSON []byte
	if pg, ok := sess.(*pgSession); ok {
		pg.mu.RLock()
		stateJSON, err = json.Marshal(pg.state)
		pg.mu.RUnlock()
	} else {
		stateJSON = []byte("{}")
	}
	if err != nil {
		return fmt.Errorf("session postgres: marshal state: %w", err)
	}

	const q = `
		UPDATE pubmed_sessions
		SET events     = events || $1::jsonb,
		    state      = $2,
		    updated_at = now()
		WHERE app_name = $3 AND user_id = $4 AND session_id = $5`

	if _, err := s.pool.Exec(ctx, q,
		json.RawMessage("["+string(evJSON)+"]"),
		stateJSON,
		sess.AppName(), sess.UserID(), sess.ID(),
	); err != nil {
		return fmt.Errorf("session postgres: append event: %w", err)
	}
	return nil
}

// Pool returns the underlying connection pool, allowing it to be shared with
// other stores (e.g. user.PostgresStore) that connect to the same database.
func (s *PostgresService) Pool() *pgxpool.Pool { return s.pool }

// Compile-time interface check.
var _ adksession.Service = (*PostgresService)(nil)

// ─── pgSession ────────────────────────────────────────────────────────────────

type pgSession struct {
	sid       string
	appName   string
	userID    string
	mu        sync.RWMutex
	state     map[string]any
	evs       []*adksession.Event
	updatedAt time.Time
}

func (s *pgSession) ID() string      { return s.sid }
func (s *pgSession) AppName() string { return s.appName }
func (s *pgSession) UserID() string  { return s.userID }
func (s *pgSession) LastUpdateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}
func (s *pgSession) State() adksession.State {
	return &pgState{mu: &s.mu, m: s.state}
}
func (s *pgSession) Events() adksession.Events {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]*adksession.Event, len(s.evs))
	copy(cp, s.evs)
	return pgEvents(cp)
}

// ─── pgState ─────────────────────────────────────────────────────────────────

type pgState struct {
	mu *sync.RWMutex
	m  map[string]any
}

func (s *pgState) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	if !ok {
		return nil, adksession.ErrStateKeyNotExist
	}
	return v, nil
}

func (s *pgState) Set(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

func (s *pgState) All() iter.Seq2[string, any] {
	s.mu.RLock()
	cp := make(map[string]any, len(s.m))
	for k, v := range s.m {
		cp[k] = v
	}
	s.mu.RUnlock()
	return func(yield func(string, any) bool) {
		for k, v := range cp {
			if !yield(k, v) {
				return
			}
		}
	}
}

// ─── pgEvents ─────────────────────────────────────────────────────────────────

type pgEvents []*adksession.Event

func (e pgEvents) All() iter.Seq[*adksession.Event] {
	return func(yield func(*adksession.Event) bool) {
		for _, ev := range e {
			if !yield(ev) {
				return
			}
		}
	}
}
func (e pgEvents) Len() int { return len(e) }
func (e pgEvents) At(i int) *adksession.Event {
	if i >= 0 && i < len(e) {
		return e[i]
	}
	return nil
}

// ─── Serialization helpers ────────────────────────────────────────────────────

// storedEvent is the JSON representation persisted in the events JSONB array.
type storedEvent struct {
	ID                 string             `json:"id"`
	InvocationID       string             `json:"invocation_id"`
	Author             string             `json:"author"`
	Timestamp          time.Time          `json:"timestamp"`
	Branch             string             `json:"branch,omitempty"`
	Actions            storedEventActions `json:"actions"`
	LongRunningToolIDs []string           `json:"long_running_tool_ids,omitempty"`
	Content            json.RawMessage    `json:"content,omitempty"`
	GroundingMetadata  json.RawMessage    `json:"grounding_metadata,omitempty"`
	CustomMetadata     json.RawMessage    `json:"custom_metadata,omitempty"`
	UsageMetadata      json.RawMessage    `json:"usage_metadata,omitempty"`
	CitationMetadata   json.RawMessage    `json:"citation_metadata,omitempty"`
	Partial            bool               `json:"partial"`
	TurnComplete       bool               `json:"turn_complete"`
	Interrupted        bool               `json:"interrupted"`
	ErrorCode          string             `json:"error_code,omitempty"`
	ErrorMessage       string             `json:"error_message,omitempty"`
}

type storedEventActions struct {
	StateDelta                 map[string]any                               `json:"state_delta,omitempty"`
	ArtifactDelta              map[string]int64                             `json:"artifact_delta,omitempty"`
	RequestedToolConfirmations map[string]toolconfirmation.ToolConfirmation `json:"requested_tool_confirmations,omitempty"`
	SkipSummarization          bool                                         `json:"skip_summarization,omitempty"`
	TransferToAgent            string                                       `json:"transfer_to_agent,omitempty"`
	Escalate                   bool                                         `json:"escalate,omitempty"`
}

func marshalOptional(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func marshalEvent(e *adksession.Event) (json.RawMessage, error) {
	content, err := marshalOptional(e.Content)
	if err != nil {
		return nil, fmt.Errorf("marshal content: %w", err)
	}
	grounding, err := marshalOptional(e.GroundingMetadata)
	if err != nil {
		return nil, fmt.Errorf("marshal grounding metadata: %w", err)
	}
	custom, err := marshalOptional(e.CustomMetadata)
	if err != nil {
		return nil, fmt.Errorf("marshal custom metadata: %w", err)
	}
	usage, err := marshalOptional(e.UsageMetadata)
	if err != nil {
		return nil, fmt.Errorf("marshal usage metadata: %w", err)
	}
	citation, err := marshalOptional(e.CitationMetadata)
	if err != nil {
		return nil, fmt.Errorf("marshal citation metadata: %w", err)
	}

	se := storedEvent{
		ID:           e.ID,
		InvocationID: e.InvocationID,
		Author:       e.Author,
		Timestamp:    e.Timestamp,
		Branch:       e.Branch,
		Actions: storedEventActions{
			StateDelta:                 e.Actions.StateDelta,
			ArtifactDelta:              e.Actions.ArtifactDelta,
			RequestedToolConfirmations: e.Actions.RequestedToolConfirmations,
			SkipSummarization:          e.Actions.SkipSummarization,
			TransferToAgent:            e.Actions.TransferToAgent,
			Escalate:                   e.Actions.Escalate,
		},
		LongRunningToolIDs: e.LongRunningToolIDs,
		Content:            content,
		GroundingMetadata:  grounding,
		CustomMetadata:     custom,
		UsageMetadata:      usage,
		CitationMetadata:   citation,
		Partial:            e.Partial,
		TurnComplete:       e.TurnComplete,
		Interrupted:        e.Interrupted,
		ErrorCode:          e.ErrorCode,
		ErrorMessage:       e.ErrorMessage,
	}

	b, err := json.Marshal(se)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func unmarshalEvents(raw []byte) ([]*adksession.Event, error) {
	if len(raw) == 0 || string(raw) == "[]" {
		return nil, nil
	}
	var stored []storedEvent
	if err := json.Unmarshal(raw, &stored); err != nil {
		return nil, fmt.Errorf("unmarshal events array: %w", err)
	}

	events := make([]*adksession.Event, 0, len(stored))
	for _, se := range stored {
		e, err := unmarshalEvent(se)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func unmarshalEvent(se storedEvent) (*adksession.Event, error) {
	var content *genai.Content
	if len(se.Content) > 0 {
		if err := json.Unmarshal(se.Content, &content); err != nil {
			return nil, fmt.Errorf("unmarshal content: %w", err)
		}
	}

	var grounding *genai.GroundingMetadata
	if len(se.GroundingMetadata) > 0 {
		if err := json.Unmarshal(se.GroundingMetadata, &grounding); err != nil {
			return nil, fmt.Errorf("unmarshal grounding metadata: %w", err)
		}
	}

	var custom map[string]any
	if len(se.CustomMetadata) > 0 {
		if err := json.Unmarshal(se.CustomMetadata, &custom); err != nil {
			return nil, fmt.Errorf("unmarshal custom metadata: %w", err)
		}
	}

	var usage *genai.GenerateContentResponseUsageMetadata
	if len(se.UsageMetadata) > 0 {
		if err := json.Unmarshal(se.UsageMetadata, &usage); err != nil {
			return nil, fmt.Errorf("unmarshal usage metadata: %w", err)
		}
	}

	var citation *genai.CitationMetadata
	if len(se.CitationMetadata) > 0 {
		if err := json.Unmarshal(se.CitationMetadata, &citation); err != nil {
			return nil, fmt.Errorf("unmarshal citation metadata: %w", err)
		}
	}

	return &adksession.Event{
		ID:           se.ID,
		InvocationID: se.InvocationID,
		Author:       se.Author,
		Timestamp:    se.Timestamp,
		Branch:       se.Branch,
		Actions: adksession.EventActions{
			StateDelta:                 se.Actions.StateDelta,
			ArtifactDelta:              se.Actions.ArtifactDelta,
			RequestedToolConfirmations: se.Actions.RequestedToolConfirmations,
			SkipSummarization:          se.Actions.SkipSummarization,
			TransferToAgent:            se.Actions.TransferToAgent,
			Escalate:                   se.Actions.Escalate,
		},
		LongRunningToolIDs: se.LongRunningToolIDs,
		LLMResponse: model.LLMResponse{
			Content:           content,
			GroundingMetadata: grounding,
			CustomMetadata:    custom,
			UsageMetadata:     usage,
			CitationMetadata:  citation,
			Partial:           se.Partial,
			TurnComplete:      se.TurnComplete,
			Interrupted:       se.Interrupted,
			ErrorCode:         se.ErrorCode,
			ErrorMessage:      se.ErrorMessage,
		},
	}, nil
}

func unmarshalState(raw []byte) (map[string]any, error) {
	m := make(map[string]any)
	if len(raw) > 0 && string(raw) != "{}" {
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// applyStateDelta merges delta into the state map, stripping temp: keys.
func applyStateDelta(state, delta map[string]any) {
	const tempPrefix = "temp:"
	for k, v := range delta {
		if !strings.HasPrefix(k, tempPrefix) {
			state[k] = v
		}
	}
}
