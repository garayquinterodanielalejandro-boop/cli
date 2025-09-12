package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cli/cli/v2/pkg/iostreams"
)

//go:generate moq -rm -out log_mock.go . LogRenderer

type LogRenderer interface {
	Follow(fetcher func() ([]byte, error), w io.Writer, cs *iostreams.ColorScheme) error
	Render(logs []byte, w io.Writer, cs *iostreams.ColorScheme) (stop bool, err error)
}

type logRenderer struct{}

func NewLogRenderer() LogRenderer {
	return &logRenderer{}
}

func (r *logRenderer) Follow(fetcher func() ([]byte, error), w io.Writer, cs *iostreams.ColorScheme) error {
	var last string
	for {
		raw, err := fetcher()
		if err != nil {
			return err
		}

		logs := string(raw)
		if logs == last {
			continue
		}

		diff := strings.TrimSpace(logs[len(last):])

		if stop, err := r.Render([]byte(diff), w, cs); err != nil {
			return err
		} else if stop {
			return nil
		}

		last = logs
	}
}

func (r *logRenderer) Render(logs []byte, w io.Writer, cs *iostreams.ColorScheme) (bool, error) {
	lines := slices.DeleteFunc(strings.Split(string(logs), "\n"), func(line string) bool {
		return line == ""
	})

	for _, line := range lines {
		raw, found := strings.CutPrefix(line, "data: ")
		if !found {
			return false, errors.New("unexpected log format")
		}

		// The only log entry type we're interested in is a chat completion chunk,
		// which can be verified by a successful unmarshal into the corresponding
		// type AND the Object field being equal to "chat.completion.chunk". The
		// latter is to avoid accepting an empty JSON object (i.e. "{}"). Also,
		// if the entry is not what we expect, we should just skip and avoid
		// returning an error.
		var entry chatCompletionChunkEntry
		err := json.Unmarshal([]byte(raw), &entry)
		if err != nil || entry.Object != "chat.completion.chunk" {
			continue
		}

		if stop, err := renderLogEntry(entry, w, cs); err != nil {
			return false, fmt.Errorf("failed to process log entry: %w", err)
		} else if stop {
			return true, nil
		}
	}

	return false, nil
}

func renderLogEntry(entry chatCompletionChunkEntry, w io.Writer, cs *iostreams.ColorScheme) (bool, error) {
	var stop bool
	for _, choice := range entry.Choices {
		if choice.FinishReason == "stop" {
			stop = true
		}

		if len(choice.Delta.ToolCalls) == 0 {
			if choice.Delta.Content != "" && choice.Delta.Role == "assistant" {
				// Copilot message and we should display.
				renderCopilotMessage(w, cs, choice.Delta.Content)
			}
			continue
		}

		// Since we don't want to clear-and-reprint live progress of events, we
		// need to only process entries that correspond to a finished tool call.
		// Such entries have a non-empty Content field.
		if choice.Delta.Content == "" {
			continue
		}

		if choice.Delta.ReasoningText != "" {
			// Note that this should be formatted as a normal Copilot message.
			renderCopilotMessage(w, cs, choice.Delta.ReasoningText)
		}

		render := func(s string) {
			_, _ = fmt.Fprintf(w, "%s\n", s)
		}

		for _, tc := range choice.Delta.ToolCalls {
			name := tc.Function.Name
			if name == "" {
				continue
			}

			args := tc.Function.Arguments

			switch name {
			case "run_setup":
				if v := unmarshal[runSetupToolArgs](args); v != nil {
					render(v.Name) // e.g. "Start 'github-mcp-server' MCP server"
					continue
				}
			case "view":
				args := viewToolArgs{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'view' tool call arguments: %w", err)
				}
				// TODO: detect if it's the repository root or just a file to show the right message
				// NOTE: omit the output since it's a git diff
				fmt.Fprintf(w, "- View %s\n", cs.Bold(args.Path))
			case "bash":
				if v := unmarshal[bashToolArgs](args); v != nil {
					if v.Description != "" {
						render("Bash: " + cs.Bold(v.Description))
					} else {
						render("Run Bash command")
					}
					continue
				}
			case "write_bash":
				if v := unmarshal[writeBashToolArgs](args); v != nil {
					render("Send input to Bash session " + v.SessionID)
					continue
				}
			case "read_bash":
				if v := unmarshal[readBashToolArgs](args); v != nil {
					render("Read logs from Bash session " + v.SessionID)
					continue
				}
			case "stop_bash":
				if v := unmarshal[stopBashToolArgs](args); v != nil {
					render("Stop Bash session " + v.SessionID)
					continue
				}
			case "async_bash":
				if v := unmarshal[asyncBashToolArgs](args); v != nil {
					render("Start or send input to long-running Bash session " + v.SessionID)
					continue
				}
			case "read_async_bash":
				if v := unmarshal[readAsyncBashToolArgs](args); v != nil {
					render("View logs from long-running Bash session " + v.SessionID)
					continue
				}
			case "stop_async_bash":
				if v := unmarshal[stopAsyncBashToolArgs](args); v != nil {
					render("Stop long-running Bash session " + v.SessionID)
					continue
				}
			case "think":
				if v := unmarshal[thinkToolArgs](args); v != nil {
					render("Stop long-running Bash session " + v.SessionID)
					continue
				}

				args := thinkToolArgs{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'think' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content since it's the same as thought
				fmt.Fprintf(w, "? %s: %s\n", cs.Bold("Thought:"), args.Thought)
			case "report_progress":
				args := reportProgressToolArgs{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'report_progress' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content to reduce noise
				fmt.Fprintf(w, "! Progress update: %s\n", cs.Bold(args.CommitMessage))
			case "create":
				args := createToolArgs{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'create' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content since it's a diff
				fmt.Fprintf(w, "- Create %s\n", cs.Bold(args.Path))
			case "str_replace":
				args := strReplaceToolArgs{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'str_replace' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content since it's a diff
				fmt.Fprintf(w, "- Edit %s\n", cs.Bold(args.Path))
			}

			// Unknown tool call. For example for "codeql_checker":
			// NOTE: omit the delta.content since we don't know how large could that be
			renderGenericToolCall(w, name)
		}
	}
	return stop, nil
}

func unmarshal[T any](raw string) *T {
	var t T
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil
	}
	return &t
}

func renderCopilotMessage(w io.Writer, message string) {
	_, _ = fmt.Fprintf(w, "%s\n", message)
}

func renderToolCall(w io.Writer, name, title string) {
	if name != "" && title != "" {
		_, _ = fmt.Fprintf(w, "%s %s\n", name, title)
	} else if title == "" {
		_, _ = fmt.Fprintf(w, "%s\n", name)
	} else {
		_, _ = fmt.Fprintf(w, "%s\n", title)
	}
}

func renderGenericToolCall(w io.Writer, name string) {
	_, _ = fmt.Fprintf(w, "Call to %s\n", name)
}

type chatCompletionChunkEntry struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Object  string `json:"object"`
	Choices []struct {
		Delta struct {
			ReasoningText string `json:"reasoning_text"`
			Content       string `json:"content"`
			Role          string `json:"role"`
			ToolCalls     []struct {
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
				Index int    `json:"index"`
				ID    string `json:"id"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
	} `json:"choices"`
}

type runSetupToolArgs struct {
	Name string `json:"name"`
}

type bashToolArgs struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

type readBashToolArgs struct {
	SessionID string `json:"sessionId"`
}

type writeBashToolArgs struct {
	SessionID string `json:"sessionId"`
	Input     string `json:"input"`
}

type stopBashToolArgs struct {
	SessionID string `json:"sessionId"`
}

type asyncBashToolArgs struct {
	Command   string `json:"command"`
	SessionID string `json:"sessionId"`
}

type readAsyncBashToolArgs struct {
	SessionID string `json:"sessionId"`
}

type stopAsyncBashToolArgs struct {
	SessionID string `json:"sessionId"`
}

type viewToolArgs struct {
	Path string `json:"path"`
}
type thinkToolArgs struct {
	Thought string `json:"thought"`
}

type reportProgressToolArgs struct {
	CommitMessage string `json:"commitMessage"`
	PrDescription string `json:"prDescription"`
}

type createToolArgs struct {
	FileText string `json:"file_text"`
	Path     string `json:"path"`
}

type strReplaceToolArgs struct {
	NewStr string `json:"new_str"`
	OldStr string `json:"old_str"`
	Path   string `json:"path"`
}
