package agentdocs

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed agent/*.md agent/rules/*.md agent/skills/*.md
var documents embed.FS

type Document struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content,omitempty"`
	Command     string `json:"command,omitempty"`
}

type IssueGuide struct {
	Command    string   `json:"command"`
	Categories []string `json:"categories"`
	Message    string   `json:"message"`
}

func Brief() string {
	content, err := documents.ReadFile("agent/brief.md")
	if err != nil {
		return "Cheat Engine CLI exposes ceserver process and memory inspection as a structured command-line API."
	}
	_, _, body := parseDocument(string(content))
	return body
}

func Rules() []Document {
	return loadDirectory("agent/rules", false)
}

func Skills() []Document {
	return loadDirectory("agent/skills", true)
}

func Skill(name string) (Document, error) {
	for _, skill := range Skills() {
		if skill.Name == name {
			return skill, nil
		}
	}
	return Document{}, fmt.Errorf("unknown skill %q", name)
}

func Issue() IssueGuide {
	return IssueGuide{
		Command:    "cecli issue create --type <category> --message <text>",
		Categories: []string{"bug", "requirement", "suggestion", "bad-output"},
		Message:    "Store structured feedback locally; no external service is contacted.",
	}
}

func loadDirectory(directory string, includeCommand bool) []Document {
	entries, err := fs.ReadDir(documents, directory)
	if err != nil {
		return nil
	}
	loaded := make([]Document, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := documents.ReadFile(directory + "/" + entry.Name())
		if err != nil {
			continue
		}
		name, description, body := parseDocument(string(content))
		document := Document{Name: name, Description: description, Content: body}
		if includeCommand {
			document.Command = "cecli skills " + name
		}
		loaded = append(loaded, document)
	}
	sort.Slice(loaded, func(left, right int) bool { return loaded[left].Name < loaded[right].Name })
	return loaded
}

func parseDocument(content string) (string, string, string) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---\n") {
		return "", "", trimmed
	}
	parts := strings.SplitN(strings.TrimPrefix(trimmed, "---\n"), "\n---\n", 2)
	if len(parts) != 2 {
		return "", "", trimmed
	}
	var name string
	var description string
	for _, line := range strings.Split(parts[0], "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = strings.Trim(strings.TrimSpace(value), `"'`)
		case "description":
			description = strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return name, description, strings.TrimSpace(parts[1])
}
