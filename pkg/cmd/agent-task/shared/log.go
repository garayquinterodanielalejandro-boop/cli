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

		// TODO: should ignore the error since the entries can be different.
		var entry logEntry
		err := json.Unmarshal([]byte(raw), &entry)
		if err != nil {
			return false, fmt.Errorf("unexpected log entry: %w", err)
		}

		if stop, err := renderLogEntry(entry, w, cs); err != nil {
			return false, fmt.Errorf("failed to process log entry: %w", err)
		} else if stop {
			return true, nil
		}
	}

	return false, nil
}

func renderLogEntry(entry logEntry, w io.Writer, cs *iostreams.ColorScheme) (bool, error) {
	var stop bool
	for _, choice := range entry.Choices {
		if choice.FinishReason == "stop" {
			stop = true
		}

		if choice.Delta.Content == "" {
			continue
		}

		if choice.Delta.Role != "" && choice.Delta.Role != "assistant" {
			// Because...
			continue
		}

		if choice.Delta.ToolCalls == nil {
			// message
			fmt.Fprintln(w, "")
			if _, err := fmt.Fprintf(w, "> %s\n", choice.Delta.Content); err != nil {
				return false, err
			}
			continue
		}

		for _, tc := range choice.Delta.ToolCalls {
			fmt.Fprintln(w, "")
			switch tc.Function.Name {
			case "run_setup":
				args := toolCallRunSetup{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'run_setup' tool call arguments: %w", err)
				}
				fmt.Fprintf(w, "- %s\n", cs.Bold(args.Name))
			case "view":
				args := toolCallView{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'view' tool call arguments: %w", err)
				}
				// TODO: detect if it's the repository root or just a file to show the right message
				// NOTE: omit the output since it's a git diff
				fmt.Fprintf(w, "- View %s\n", cs.Bold(args.Path))
			case "bash":
				args := toolCallBash{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'bash' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content to reduce noise
				fmt.Fprintf(w, "- Bash: %s\n", cs.Bold(args.Description))
			case "think":
				args := toolCallThink{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'think' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content since it's the same as thought
				fmt.Fprintf(w, "? %s: %s\n", cs.Bold("Thought:"), args.Thought)
			case "report_progress":
				args := toolCallReportProgress{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'report_progress' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content to reduce noise
				fmt.Fprintf(w, "! Progress update: %s\n", cs.Bold(args.CommitMessage))
			case "create":
				args := toolCallCreate{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'create' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content since it's a diff
				fmt.Fprintf(w, "- Create %s\n", cs.Bold(args.Path))
			case "str_replace":
				args := toolCallStrReplace{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					return false, fmt.Errorf("failed to parse 'str_replace' tool call arguments: %w", err)
				}
				// NOTE: omit the delta.content since it's a diff
				fmt.Fprintf(w, "- Edit %s\n", cs.Bold(args.Path))
			default:
				// Unknown tool call. For example for "codeql_checker":
				// NOTE: omit the delta.content since we don't know how large could that be
				fmt.Fprintf(w, "- Call to %s\n", cs.Bold(tc.Function.Name))
			}
		}
	}
	return stop, nil
}

type logEntry struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Object  string `json:"object"`
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			Role      string `json:"role"`
			ToolCalls []struct {
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

type toolCallRunSetup struct {
	Name string `json:"name"`
}

type toolCallView struct {
	Path string `json:"path"`
}

type toolCallBash struct {
	Async       bool   `json:"async"`
	Command     string `json:"command"`
	Description string `json:"description"`
	SessionID   string `json:"sessionId"`
}

type toolCallThink struct {
	Thought string `json:"thought"`
}

type toolCallReportProgress struct {
	CommitMessage string `json:"commitMessage"`
	PrDescription string `json:"prDescription"`
}

type toolCallCreate struct {
	FileText string `json:"file_text"`
	Path     string `json:"path"`
}

type toolCallStrReplace struct {
	NewStr string `json:"new_str"`
	OldStr string `json:"old_str"`
	Path   string `json:"path"`
}
