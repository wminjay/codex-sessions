package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func loadRollouts(store Store, options LoadOptions) ([]Session, error) {
	root := filepath.Join(store.CodexHome, "sessions")
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", root, err)
	}
	names := loadSessionNames(store.CodexHome)
	items := make([]Session, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return nil
		}
		if !strings.HasPrefix(entry.Name(), "rollout-") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			return nil
		}
		item, ok := readRollout(path, store, names)
		if !ok {
			return nil
		}
		if !options.AllSources && !isInteractiveSource(item.Source) {
			return nil
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	return items, nil
}

func readRollout(path string, store Store, names map[string]string) (Session, bool) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)
	if !scanner.Scan() {
		return Session{}, false
	}
	var first map[string]any
	if json.Unmarshal(scanner.Bytes(), &first) != nil {
		return Session{}, false
	}
	payload, _ := first["payload"].(map[string]any)
	id := stringValue(payload["id"])
	if id == "" {
		return Session{}, false
	}
	title := names[id]
	for title == "" && scanner.Scan() {
		var event map[string]any
		if json.Unmarshal(scanner.Bytes(), &event) != nil {
			continue
		}
		title = userMessage(event)
	}
	if title == "" {
		return Session{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return Session{}, false
	}
	source := sourceValue(payload["source"])
	if source == "" {
		source = stringValue(payload["originator"])
	}
	if source == "" {
		source = "cli"
	}
	provider := fallback(cleanText(stringValue(payload["model_provider"])), "unknown")
	return Session{
		ID: id, Title: cleanText(title), UpdatedMS: info.ModTime().UnixMilli(),
		Provider: provider, Source: cleanText(source), CWD: cleanText(stringValue(payload["cwd"])),
		Model: cleanText(stringValue(payload["model"])), CodexHome: store.CodexHome,
		SQLiteHome: store.SQLiteHome,
	}, true
}

func loadSessionNames(codexHome string) map[string]string {
	file, err := os.Open(filepath.Join(codexHome, "session_index.jsonl"))
	if err != nil {
		return nil
	}
	defer file.Close()
	names := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var item map[string]any
		if json.Unmarshal(scanner.Bytes(), &item) != nil {
			continue
		}
		id := stringValue(item["id"])
		name := cleanText(stringValue(item["thread_name"]))
		if id != "" && name != "" {
			names[id] = name
		}
	}
	return names
}

func userMessage(event map[string]any) string {
	payload, _ := event["payload"].(map[string]any)
	typeName := stringValue(event["type"])
	var texts []string
	if typeName == "response_item" && stringValue(payload["type"]) == "message" {
		if stringValue(payload["role"]) != "user" {
			return ""
		}
		content, _ := payload["content"].([]any)
		for _, raw := range content {
			item, _ := raw.(map[string]any)
			if stringValue(item["type"]) == "input_text" {
				texts = append(texts, stringValue(item["text"]))
			}
		}
	} else if typeName == "event_msg" && stringValue(payload["type"]) == "user_message" {
		texts = append(texts, stringValue(payload["message"]))
	}
	message := cleanText(strings.Join(texts, " "))
	if strings.HasPrefix(message, "<environment_context>") {
		return ""
	}
	return message
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func sourceValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}
