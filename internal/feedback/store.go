package feedback

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var validTypes = map[string]bool{
	"bug": true, "requirement": true, "suggestion": true, "bad-output": true,
}

var validStatuses = map[string]bool{
	"open": true, "in-progress": true, "resolved": true, "closed": true,
}

type Issue struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Status    string         `json:"status"`
	Message   string         `json:"message"`
	Context   map[string]any `json:"context,omitempty"`
	Version   string         `json:"version"`
	ExitCode  *int           `json:"exit_code,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Store struct {
	directory string
	clock     func() time.Time
}

func NewStore(directory string) (*Store, error) {
	if directory == "" {
		configured := os.Getenv("CECLI_STATE_DIR")
		if configured != "" {
			directory = configured
		} else {
			userConfigDirectory, err := os.UserConfigDir()
			if err != nil {
				return nil, fmt.Errorf("resolve user config directory: %w", err)
			}
			directory = filepath.Join(userConfigDirectory, "cecli")
		}
	}
	issuesDirectory := filepath.Join(directory, "issues")
	if err := os.MkdirAll(issuesDirectory, 0o700); err != nil {
		return nil, fmt.Errorf("create issue store: %w", err)
	}
	return &Store{directory: issuesDirectory, clock: time.Now}, nil
}

func (store *Store) Create(issueType, message, version string, context map[string]any, exitCode *int) (Issue, error) {
	issueType = strings.ToLower(strings.TrimSpace(issueType))
	if !validTypes[issueType] {
		return Issue{}, fmt.Errorf("invalid issue type %q", issueType)
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return Issue{}, errors.New("issue message is required")
	}
	if len(message) > 16_384 {
		return Issue{}, errors.New("issue message exceeds 16384 characters")
	}
	now := store.clock().UTC()
	identifier, err := newIdentifier(now)
	if err != nil {
		return Issue{}, err
	}
	issue := Issue{
		ID: identifier, Type: issueType, Status: "open", Message: message,
		Context: context, Version: version, ExitCode: exitCode, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.write(issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func (store *Store) List() ([]Issue, error) {
	entries, err := os.ReadDir(store.directory)
	if err != nil {
		return nil, fmt.Errorf("read issue store: %w", err)
	}
	issues := make([]Issue, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		issue, err := store.Show(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	sort.Slice(issues, func(left, right int) bool { return issues[left].CreatedAt.After(issues[right].CreatedAt) })
	return issues, nil
}

func (store *Store) Show(identifier string) (Issue, error) {
	path, err := store.issuePath(identifier)
	if err != nil {
		return Issue{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Issue{}, fmt.Errorf("issue %q not found", identifier)
		}
		return Issue{}, fmt.Errorf("read issue %q: %w", identifier, err)
	}
	var issue Issue
	if err := json.Unmarshal(content, &issue); err != nil {
		return Issue{}, fmt.Errorf("decode issue %q: %w", identifier, err)
	}
	return issue, nil
}

func (store *Store) Transition(identifier, status string) (Issue, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if !validStatuses[status] {
		return Issue{}, fmt.Errorf("invalid issue status %q", status)
	}
	issue, err := store.Show(identifier)
	if err != nil {
		return Issue{}, err
	}
	issue.Status = status
	issue.UpdatedAt = store.clock().UTC()
	if err := store.write(issue); err != nil {
		return Issue{}, err
	}
	return issue, nil
}

func (store *Store) write(issue Issue) error {
	path, err := store.issuePath(issue.ID)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(issue, "", "  ")
	if err != nil {
		return fmt.Errorf("encode issue: %w", err)
	}
	temporaryPath := path + ".tmp"
	if err := os.WriteFile(temporaryPath, append(content, '\n'), 0o600); err != nil {
		return fmt.Errorf("write issue: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace issue: %w", err)
	}
	return nil
}

func (store *Store) issuePath(identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || strings.ContainsAny(identifier, `/\\`) || strings.Contains(identifier, "..") {
		return "", errors.New("invalid issue identifier")
	}
	return filepath.Join(store.directory, identifier+".json"), nil
}

func newIdentifier(now time.Time) (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate issue identifier: %w", err)
	}
	return fmt.Sprintf("CECLI-%s-%s", now.Format("20060102T150405Z"), hex.EncodeToString(randomBytes)), nil
}
