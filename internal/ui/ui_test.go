package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wminjay/codex-sessions/internal/session"
)

func TestDirectoryToSessionNavigation(t *testing.T) {
	items := []session.Session{
		{ID: "one", Title: "First prompt", Provider: "openai", Source: "cli", CWD: "/work/app", UpdatedMS: 20},
		{ID: "two", Title: "Second prompt", Provider: "custom", Source: "cli", CWD: "/work/app", UpdatedMS: 10},
	}
	model := New(items, Options{Yolo: true})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	model = updated.(Model)
	view := model.View()
	if !strings.Contains(view, "CODEX SESSIONS") || !strings.Contains(view, "YOLO") || !strings.Contains(view, "app") {
		t.Fatalf("directory view missing expected content:\n%s", view)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	view = model.View()
	if !strings.Contains(view, "First prompt") || !strings.Contains(view, "Second prompt") {
		t.Fatalf("session view missing titles:\n%s", view)
	}
}

func TestCustomSelectionCreatesLaunchRequest(t *testing.T) {
	items := []session.Session{{ID: "one", Title: "Prompt", Provider: "openai", CWD: "/work/app"}}
	model := New(items, Options{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model = updated.(Model)
	if model.request == nil || model.request.Provider != "custom" || model.request.Session.ID != "one" {
		t.Fatalf("unexpected request: %#v", model.request)
	}
}
