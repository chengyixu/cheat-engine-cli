package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	agentdocs "github.com/chengyixu/cheat-engine-cli"
)

type commandResult struct {
	Data  any
	Human string
	Meta  map[string]any
}

type successEnvelope struct {
	OK      bool                 `json:"ok"`
	Command string               `json:"command"`
	Data    any                  `json:"data"`
	Meta    map[string]any       `json:"meta"`
	Rules   []agentdocs.Document `json:"rules"`
	Skills  []agentdocs.Document `json:"skills"`
	Issue   agentdocs.IssueGuide `json:"issue"`
}

type errorEnvelope struct {
	Error      bool           `json:"error"`
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Suggestion string         `json:"suggestion"`
	Command    string         `json:"command,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

func (application *app) writeSuccess(command string, result commandResult, startedAt time.Time) error {
	if application.options.quiet {
		return nil
	}
	if application.options.human {
		_, err := fmt.Fprintln(application.stdout, strings.TrimRight(result.Human, "\n"))
		return err
	}
	meta := map[string]any{
		"version":     Version,
		"commit":      Commit,
		"build_date":  BuildDate,
		"duration_ms": time.Since(startedAt).Milliseconds(),
	}
	if commandUsesServer(command) {
		meta["endpoint"] = application.options.endpoint
	}
	for key, value := range result.Meta {
		meta[key] = value
	}
	data := result.Data
	if len(application.options.fields) > 0 {
		filtered, missing, err := filterOutputFields(result.Data, application.options.fields)
		if err != nil {
			return err
		}
		data = filtered
		meta["fields"] = application.options.fields
		if len(missing) > 0 {
			meta["missing_fields"] = missing
		}
	}
	envelope := successEnvelope{
		OK: true, Command: command, Data: data, Meta: meta,
		Rules: agentdocs.Rules(), Skills: agentdocs.Skills(), Issue: agentdocs.Issue(),
	}
	return writeJSON(application.stdout, envelope, application.options.pretty)
}

func (application *app) writeError(command string, err error) int {
	normalized := normalizeError(err)
	if application.options.quiet {
		return normalized.ExitCode
	}
	if application.options.human {
		fmt.Fprintf(application.stderr, "Error [%s]: %s\nSuggestion: %s\n", normalized.Code, normalized.Message, normalized.Suggestion)
		return normalized.ExitCode
	}
	envelope := errorEnvelope{
		Error: true, Code: normalized.Code, Message: normalized.Message,
		Suggestion: normalized.Suggestion, Command: command, Details: normalized.Details,
	}
	_ = writeJSON(application.stderr, envelope, application.options.pretty)
	return normalized.ExitCode
}

func filterOutputFields(data any, fields []string) (map[string]any, []string, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, nil, fmt.Errorf("encode output for --fields: %w", err)
	}
	var source map[string]any
	if err := json.Unmarshal(encoded, &source); err != nil {
		return nil, nil, fmt.Errorf("--fields requires object-shaped command data: %w", err)
	}
	filtered := make(map[string]any)
	missing := make([]string, 0)
	for _, field := range fields {
		segments := strings.Split(field, ".")
		value, found := lookupOutputField(source, segments)
		if !found {
			missing = append(missing, field)
			continue
		}
		setOutputField(filtered, segments, value)
	}
	return filtered, missing, nil
}

func lookupOutputField(source map[string]any, segments []string) (any, bool) {
	var current any = source
	for _, segment := range segments {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func setOutputField(destination map[string]any, segments []string, value any) {
	current := destination
	for _, segment := range segments[:len(segments)-1] {
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[segment] = next
		}
		current = next
	}
	current[segments[len(segments)-1]] = value
}

func writeJSON(writer io.Writer, value any, pretty bool) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(value)
}

func commandUsesServer(command string) bool {
	return strings.HasPrefix(command, "server ") || strings.HasPrefix(command, "process ") || strings.HasPrefix(command, "module ") || strings.HasPrefix(command, "thread ") || strings.HasPrefix(command, "debug ") || strings.HasPrefix(command, "memory ") || strings.HasPrefix(command, "remote ") || strings.HasPrefix(command, "pipe ") || strings.HasPrefix(command, "symbol ")
}
