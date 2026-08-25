package session

import (
	"path/filepath"
	"sort"
	"strings"
)

// Store points to one Codex configuration and SQLite state pair.
type Store struct {
	CodexHome  string
	SQLiteHome string
}

// Session is the stable subset of a Codex thread needed by the launcher.
type Session struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	UpdatedMS  int64  `json:"updated_ms"`
	Provider   string `json:"provider"`
	Source     string `json:"source"`
	CWD        string `json:"cwd"`
	Model      string `json:"model,omitempty"`
	Archived   bool   `json:"archived"`
	CodexHome  string `json:"codex_home"`
	SQLiteHome string `json:"sqlite_home"`
}

func (s Session) Project() string {
	if s.CWD == "" {
		return "Unknown directory"
	}
	project := filepath.Base(filepath.Clean(s.CWD))
	if project == "." || project == string(filepath.Separator) {
		return s.CWD
	}
	return project
}

// Directory groups sessions by their recorded working directory.
type Directory struct {
	CWD       string
	Sessions  []Session
	UpdatedMS int64
}

func (d Directory) Project() string {
	if d.CWD == "" {
		return "Unknown directory"
	}
	project := filepath.Base(filepath.Clean(d.CWD))
	if project == "." || project == string(filepath.Separator) {
		return d.CWD
	}
	return project
}

func (d Directory) ProviderSummary() string {
	counts := make(map[string]int)
	for _, item := range d.Sessions {
		counts[item.Provider]++
	}
	providers := make([]string, 0, len(counts))
	for provider := range counts {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	parts := make([]string, 0, len(providers))
	for _, provider := range providers {
		parts = append(parts, provider+":"+itoa(counts[provider]))
	}
	return strings.Join(parts, "  ")
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	for value > 0 {
		digits = append(digits, byte('0'+value%10))
		value /= 10
	}
	for left, right := 0, len(digits)-1; left < right; left, right = left+1, right-1 {
		digits[left], digits[right] = digits[right], digits[left]
	}
	return string(digits)
}
