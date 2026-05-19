package tools

import (
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// AskUserArgs is the input for the ask_user tool.
type AskUserArgs struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

// AskUserPending is the immediate response returned when the tool is first invoked.
// The ADK framework will emit a function-call event and the UI will prompt the user.
type AskUserPending struct {
	Status   string   `json:"status"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

// AskUserResult is the final response once the user has provided an answer.
type AskUserResult struct {
	Answer string `json:"answer"`
}

// NewAskUserTool creates the ask_user ADK tool.
// This is a long-running tool: the handler returns immediately with {status: "pending"}.
// The ADK launcher emits a FunctionCall event to the UI; the user's reply comes back
// as a FunctionResponse on the same call ID. The LLM sees the answer in the next turn.
func NewAskUserTool() (tool.Tool, error) {
	handler := func(ctx tool.Context, args AskUserArgs) (AskUserPending, error) {
		ctx.Actions().SkipSummarization = true
		return AskUserPending{
			Status:   "pending",
			Question: args.Question,
			Options:  args.Options,
		}, nil
	}

	return functiontool.New(functiontool.Config{
		Name:          "ask_user",
		Description:   "Ask the user a clarifying question and wait for their answer. Use this when the research question is too vague, ambiguous, or needs narrowing before a productive PubMed search can be performed.",
		IsLongRunning: true,
	}, handler)
}
