package server

import (
	"encoding/json"
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/alimoeeny/pubmed_search_agent/server/authz"
)

func TestListSessionsIncludesFirstUserQuestionAsTitle(t *testing.T) {
	t.Parallel()

	const (
		appName   = "test_app"
		userID    = "dev-user"
		sessionID = "test-session"
	)

	sessionSvc := adksession.InMemoryService()
	created, err := sessionSvc.Create(t.Context(), &adksession.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	for _, question := range []string{
		"What are the cardiovascular effects of aspirin after STEMI?",
		"Focus on randomized trials.",
	} {
		ev := adksession.NewEvent("test-invocation")
		ev.Author = "user"
		ev.LLMResponse = model.LLMResponse{
			Content: genai.NewContentFromText(question, genai.RoleUser),
		}
		if err := sessionSvc.AppendEvent(t.Context(), created.Session, ev); err != nil {
			t.Fatalf("append user event: %v", err)
		}
	}

	srv := New(Config{
		AppName:      appName,
		SessionSvc:   sessionSvc,
		AuthzChecker: authz.NoOpChecker{},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/sessions", nil)
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Sessions []struct {
			Title string `json:"title"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Sessions) != 1 {
		t.Fatalf("session count = %d, body = %s", len(body.Sessions), rec.Body.String())
	}
	const want = "What are the cardiovascular effects of aspirin after STEMI?"
	if body.Sessions[0].Title != want {
		t.Fatalf("title = %q, want %q", body.Sessions[0].Title, want)
	}
}

func TestPostMessageStreamsDistinctProgressAfterPartialText(t *testing.T) {
	t.Parallel()

	body := postMessageBody(t, func(ctx agent.InvocationContext) []*adksession.Event {
		return []*adksession.Event{
			textEvent(ctx, true, "Starting search...\n"),
			textEvent(ctx, false, "Planning PubMed query...\n"),
			textEvent(ctx, false, "Final answer."),
		}
	})

	for _, want := range []string{
		"Starting search...",
		"Planning PubMed query...",
		"Final answer.",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("SSE body missing %q:\n%s", want, body)
		}
	}
}

func TestPostMessageSkipsExactAccumulatedTextDuplicate(t *testing.T) {
	t.Parallel()

	body := postMessageBody(t, func(ctx agent.InvocationContext) []*adksession.Event {
		return []*adksession.Event{
			textEvent(ctx, true, "Checking weather..."),
			textEvent(ctx, false, "Checking weather..."),
			textEvent(ctx, false, "Task completed."),
		}
	})

	if got := strings.Count(body, "Checking weather..."); got != 1 {
		t.Fatalf("duplicate accumulated text count = %d, body:\n%s", got, body)
	}
	if !strings.Contains(body, "Task completed.") {
		t.Fatalf("SSE body missing final text:\n%s", body)
	}
}

func TestStripPDFDownloadLinksRemovesPlainGCSReportLinks(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"Summary stays.",
		"",
		"Download PDF report: [Open PDF](https://storage.googleapis.com/private-bucket/2026-06-05_report.pdf)",
		"",
		"References stay.",
	}, "\n")

	got := stripPDFDownloadLinks(input)

	if strings.Contains(got, "storage.googleapis.com") {
		t.Fatalf("plain GCS PDF URL was not stripped:\n%s", got)
	}
	if strings.Contains(got, "Download PDF report") {
		t.Fatalf("PDF download line was not stripped:\n%s", got)
	}
	for _, want := range []string{"Summary stays.", "References stay."} {
		if !strings.Contains(got, want) {
			t.Fatalf("sanitized text missing %q:\n%s", want, got)
		}
	}
}

func postMessageBody(t *testing.T, events func(agent.InvocationContext) []*adksession.Event) string {
	t.Helper()

	const (
		appName   = "test_app"
		userID    = "dev-user"
		sessionID = "test-session"
	)

	testAgent, err := agent.New(agent.Config{
		Name:        "test_agent",
		Description: "emits deterministic SSE test events",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*adksession.Event, error] {
			return func(yield func(*adksession.Event, error) bool) {
				for _, ev := range events(ctx) {
					if !yield(ev, nil) {
						return
					}
				}
			}
		},
	})
	if err != nil {
		t.Fatalf("agent.New: %v", err)
	}

	sessionSvc := adksession.InMemoryService()
	if _, err := sessionSvc.Create(t.Context(), &adksession.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}

	testRunner, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          testAgent,
		SessionService: sessionSvc,
	})
	if err != nil {
		t.Fatalf("runner.New: %v", err)
	}

	srv := New(Config{
		AppName:      appName,
		Runner:       testRunner,
		SessionSvc:   sessionSvc,
		AuthzChecker: authz.NoOpChecker{},
	})

	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/sessions/"+sessionID+"/messages",
		strings.NewReader(`{"text":"aspirin after STEMI"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	return rec.Body.String()
}

func textEvent(ctx agent.InvocationContext, partial bool, text string) *adksession.Event {
	ev := adksession.NewEvent(ctx.InvocationID())
	ev.Author = "test_agent"
	ev.LLMResponse = model.LLMResponse{
		Content: &genai.Content{
			Role:  "model",
			Parts: []*genai.Part{{Text: text}},
		},
		Partial: partial,
	}
	return ev
}
