package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/wminjay/codex-sessions/internal/launcher"
	"github.com/wminjay/codex-sessions/internal/session"
)

type viewMode int

const (
	directoryView viewMode = iota
	sessionView
)

type savedState struct {
	query    string
	selected int
	offset   int
}

type Options struct {
	InitialQuery string
	Yolo         bool
	Inline       bool
}

type Model struct {
	sessions         []session.Session
	directories      []session.Directory
	mode             viewMode
	currentDirectory *session.Directory
	search           textinput.Model
	searching        bool
	selected         int
	offset           int
	directoryState   savedState
	width            int
	height           int
	yolo             bool
	request          *launcher.Request
}

var (
	primary = lipgloss.AdaptiveColor{Light: "#6D4AFF", Dark: "#B59CFF"}
	cyan    = lipgloss.AdaptiveColor{Light: "#007F8B", Dark: "#56D4DD"}
	green   = lipgloss.AdaptiveColor{Light: "#087A45", Dark: "#50D890"}
	yellow  = lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#FFD166"}
	red     = lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"}
	muted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#7D8590"}
	soft    = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#30363D"}

	brandStyle    = lipgloss.NewStyle().Bold(true).Foreground(primary)
	breadcrumb    = lipgloss.NewStyle().Foreground(muted)
	mutedStyle    = lipgloss.NewStyle().Foreground(muted)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(primary)
	pathStyle     = lipgloss.NewStyle().Foreground(muted)
	countStyle    = lipgloss.NewStyle().Foreground(cyan)
	helpKeyStyle  = lipgloss.NewStyle().Bold(true).Foreground(primary)
	helpTextStyle = lipgloss.NewStyle().Foreground(muted)
	warningStyle  = lipgloss.NewStyle().Bold(true).Foreground(red)
)

func New(items []session.Session, options Options) Model {
	search := textinput.New()
	search.Prompt = "  / "
	search.Placeholder = "Search projects, titles, providers, models, or IDs"
	search.SetValue(options.InitialQuery)
	search.PromptStyle = lipgloss.NewStyle().Foreground(primary).Bold(true)
	search.TextStyle = lipgloss.NewStyle().Foreground(cyan)
	search.PlaceholderStyle = lipgloss.NewStyle().Foreground(muted)
	search.Cursor.Style = lipgloss.NewStyle().Foreground(primary)
	return Model{
		sessions:       items,
		directories:    session.Group(items),
		mode:           directoryView,
		search:         search,
		directoryState: savedState{query: options.InitialQuery},
		yolo:           options.Yolo,
	}
}

func Run(items []session.Session, options Options) (*launcher.Request, error) {
	programOptions := []tea.ProgramOption{tea.WithMouseCellMotion()}
	if !options.Inline {
		programOptions = append(programOptions, tea.WithAltScreen())
	}
	final, err := tea.NewProgram(New(items, options), programOptions...).Run()
	if err != nil {
		return nil, err
	}
	model, ok := final.(Model)
	if !ok {
		return nil, fmt.Errorf("unexpected UI state %T", final)
	}
	return model.request, nil
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := message.(type) {
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
		m.search.Width = max(20, m.contentWidth()-5)
		m.normalizeSelection()
		return m, nil
	case tea.KeyMsg:
		key := typed.String()
		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if m.searching {
			return m.updateSearch(typed)
		}
		return m.updateNavigation(key)
	case tea.MouseMsg:
		return m.updateMouse(typed)
	}
	return m, nil
}

func (m Model) updateSearch(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "enter":
		m.searching = false
		m.search.Blur()
		return m, nil
	case "esc":
		m.search.SetValue("")
		m.searching = false
		m.search.Blur()
		m.selected = 0
		m.offset = 0
		return m, nil
	}
	before := m.search.Value()
	var command tea.Cmd
	m.search, command = m.search.Update(message)
	if m.search.Value() != before {
		m.selected = 0
		m.offset = 0
		m.normalizeSelection()
	}
	return m, command
}

func (m Model) updateNavigation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q":
		return m, tea.Quit
	case "/":
		m.searching = true
		return m, m.search.Focus()
	}
	if m.mode == sessionView {
		switch key {
		case "esc", "b", "left", "backspace":
			m.mode = directoryView
			m.currentDirectory = nil
			m.search.SetValue(m.directoryState.query)
			m.selected = m.directoryState.selected
			m.offset = m.directoryState.offset
			m.normalizeSelection()
			return m, nil
		}
	} else if key == "esc" {
		return m, tea.Quit
	}

	count := m.itemCount()
	page := m.rowsAvailable()
	switch key {
	case "up", "k":
		if count > 0 {
			m.selected = max(0, m.selected-1)
		}
	case "down", "j":
		if count > 0 {
			m.selected = min(count-1, m.selected+1)
		}
	case "pgup", "ctrl+u":
		m.selected = max(0, m.selected-page)
	case "pgdown", "ctrl+d":
		m.selected = min(max(0, count-1), m.selected+page)
	case "home", "g":
		m.selected = 0
	case "end", "G":
		m.selected = max(0, count-1)
	case "enter":
		if count == 0 {
			break
		}
		if m.mode == directoryView {
			directories := m.filteredDirectories()
			selected := directories[m.selected]
			m.directoryState = savedState{
				query: m.search.Value(), selected: m.selected, offset: m.offset,
			}
			m.currentDirectory = &selected
			m.mode = sessionView
			m.search.SetValue("")
			m.selected = 0
			m.offset = 0
		} else {
			return m.choose(launcher.ActionResume, launcher.ProviderOriginal)
		}
	case "c":
		if m.mode == sessionView && count > 0 {
			return m.choose(launcher.ActionResume, launcher.ProviderCustom)
		}
	case "a":
		if m.mode == sessionView && count > 0 {
			return m.choose(launcher.ActionResume, launcher.ProviderOpenAI)
		}
	case "f":
		if m.mode == sessionView && count > 0 {
			return m.choose(launcher.ActionFork, launcher.ProviderOriginal)
		}
	case "C":
		if m.mode == sessionView && count > 0 {
			return m.choose(launcher.ActionFork, launcher.ProviderCustom)
		}
	case "A":
		if m.mode == sessionView && count > 0 {
			return m.choose(launcher.ActionFork, launcher.ProviderOpenAI)
		}
	}
	m.normalizeSelection()
	return m, nil
}

func (m Model) updateMouse(message tea.MouseMsg) (tea.Model, tea.Cmd) {
	if message.Button != tea.MouseButtonWheelUp && message.Button != tea.MouseButtonWheelDown {
		return m, nil
	}
	if message.Button == tea.MouseButtonWheelUp {
		m.selected = max(0, m.selected-3)
	} else {
		m.selected = min(max(0, m.itemCount()-1), m.selected+3)
	}
	m.normalizeSelection()
	return m, nil
}

func (m Model) choose(action, provider string) (tea.Model, tea.Cmd) {
	items := m.filteredSessions()
	if len(items) == 0 {
		return m, nil
	}
	m.request = &launcher.Request{Session: items[m.selected], Action: action, Provider: provider}
	return m, tea.Quit
}

func (m *Model) normalizeSelection() {
	count := m.itemCount()
	if count == 0 {
		m.selected = 0
		m.offset = 0
		return
	}
	m.selected = min(max(0, m.selected), count-1)
	rows := m.rowsAvailable()
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+rows {
		m.offset = m.selected - rows + 1
	}
	m.offset = max(0, min(m.offset, max(0, count-rows)))
}

func (m Model) filteredDirectories() []session.Directory {
	return session.FilterDirectories(m.directories, m.search.Value())
}

func (m Model) filteredSessions() []session.Session {
	if m.currentDirectory == nil {
		return nil
	}
	return session.Filter(m.currentDirectory.Sessions, m.search.Value())
}

func (m Model) itemCount() int {
	if m.mode == directoryView {
		return len(m.filteredDirectories())
	}
	return len(m.filteredSessions())
}

func (m Model) rowsAvailable() int { return max(1, m.height-10) }

func (m Model) contentWidth() int {
	padding := 4
	if m.width < 70 {
		padding = 2
	}
	return max(30, m.width-padding)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading Codex Sessions…"
	}
	contentWidth := m.contentWidth()
	padding := 2
	if m.width < 70 {
		padding = 1
	}
	lines := []string{
		m.headerView(contentWidth),
		m.searchView(contentWidth),
		lipgloss.NewStyle().Foreground(soft).Render(strings.Repeat("─", contentWidth)),
	}

	rows := m.rowsAvailable()
	if m.mode == directoryView {
		items := m.filteredDirectories()
		if len(items) == 0 {
			lines = append(lines, mutedStyle.Render("  No matching directories"))
		} else {
			end := min(len(items), m.offset+rows)
			for index := m.offset; index < end; index++ {
				lines = append(lines, directoryRow(items[index], index == m.selected, contentWidth))
			}
		}
	} else {
		items := m.filteredSessions()
		if len(items) == 0 {
			lines = append(lines, mutedStyle.Render("  No matching sessions"))
		} else {
			end := min(len(items), m.offset+rows)
			for index := m.offset; index < end; index++ {
				lines = append(lines, sessionRow(items[index], index == m.selected, contentWidth))
			}
		}
	}
	for len(lines) < rows+3 {
		lines = append(lines, "")
	}
	lines = append(lines,
		lipgloss.NewStyle().Foreground(soft).Render(strings.Repeat("─", contentWidth)),
		m.detailView(contentWidth),
		m.helpView(contentWidth),
	)
	return lipgloss.NewStyle().Padding(1, 0, 0, padding).Render(strings.Join(lines, "\n"))
}

func (m Model) headerView(width int) string {
	brand := brandStyle.Render("◆ CODEX SESSIONS")
	crumb := "  /  Directories"
	if m.mode == sessionView && m.currentDirectory != nil {
		crumb += "  /  " + m.currentDirectory.Project()
	}
	count := fmt.Sprintf("%d projects  ·  %d sessions", len(m.directories), len(m.sessions))
	right := mutedStyle.Render(count)
	if m.yolo {
		right += "  " + warningStyle.Render("⚠ YOLO")
	}
	left := brand + breadcrumb.Render(crumb)
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", space) + right
}

func (m Model) searchView(width int) string {
	m.search.Width = max(10, width-5)
	if m.searching {
		return m.search.View()
	}
	value := m.search.Value()
	if value == "" {
		return mutedStyle.Render("  /  Search projects, titles, providers, models, or IDs")
	}
	return helpKeyStyle.Render("  / ") + lipgloss.NewStyle().Foreground(cyan).Render(clip(value, width-5))
}

func directoryRow(item session.Directory, selected bool, width int) string {
	marker := "  "
	projectStyle := lipgloss.NewStyle()
	if selected {
		marker = selectedStyle.Render("› ")
		projectStyle = selectedStyle
	}
	if width < 78 {
		projectWidth := max(10, width-26)
		return marker + projectStyle.Render(pad(item.Project(), projectWidth)) + " " +
			countStyle.Render(pad(sessionCount(len(item.Sessions)), 12)) + " " +
			mutedStyle.Render(pad(relativeTime(item.UpdatedMS), 11))
	}
	projectWidth := min(26, max(16, width/5))
	countWidth := 12
	providerWidth := 23
	timeWidth := 11
	pathWidth := max(8, width-2-projectWidth-countWidth-providerWidth-timeWidth-4)
	return marker +
		projectStyle.Render(pad(item.Project(), projectWidth)) + " " +
		countStyle.Render(pad(sessionCount(len(item.Sessions)), countWidth)) + " " +
		lipgloss.NewStyle().Foreground(cyan).Render(pad(item.ProviderSummary(), providerWidth)) + " " +
		mutedStyle.Render(pad(relativeTime(item.UpdatedMS), timeWidth)) + " " +
		pathStyle.Render(clip(item.CWD, pathWidth))
}

func sessionRow(item session.Session, selected bool, width int) string {
	marker := "  "
	titleStyle := lipgloss.NewStyle()
	if selected {
		marker = selectedStyle.Render("› ")
		titleStyle = selectedStyle
	}
	providerWidth := 10
	timeWidth := 11
	if width < 68 {
		titleWidth := max(10, width-providerWidth-timeWidth-5)
		return marker + titleStyle.Render(pad(item.Title, titleWidth)) + " " +
			providerBadge(item.Provider, providerWidth) + " " + mutedStyle.Render(relativeTime(item.UpdatedMS))
	}
	sourceWidth := 8
	titleWidth := max(12, width-providerWidth-sourceWidth-timeWidth-7)
	return marker + titleStyle.Render(pad(item.Title, titleWidth)) + " " +
		providerBadge(item.Provider, providerWidth) + " " +
		mutedStyle.Render(pad(item.Source, sourceWidth)) + " " +
		mutedStyle.Render(pad(relativeTime(item.UpdatedMS), timeWidth))
}

func providerBadge(provider string, width int) string {
	color := yellow
	if provider == launcher.ProviderOpenAI {
		color = green
	} else if provider == launcher.ProviderCustom {
		color = primary
	}
	return lipgloss.NewStyle().Bold(true).Foreground(color).Render(pad(provider, width))
}

func (m Model) detailView(width int) string {
	if m.mode == directoryView {
		items := m.filteredDirectories()
		if len(items) == 0 {
			return ""
		}
		item := items[min(m.selected, len(items)-1)]
		return pathStyle.Render("  " + clip(item.CWD, width-2))
	}
	items := m.filteredSessions()
	if len(items) == 0 {
		return ""
	}
	item := items[min(m.selected, len(items)-1)]
	detail := item.ID + "  ·  " + item.CWD
	if item.Model != "" {
		detail += "  ·  " + item.Model
	}
	return pathStyle.Render("  " + clip(detail, width-2))
}

func (m Model) helpView(width int) string {
	var pairs [][2]string
	if m.mode == directoryView {
		pairs = [][2]string{{"↑↓", "navigate"}, {"enter", "open"}, {"/", "search"}, {"q", "quit"}}
	} else {
		pairs = [][2]string{
			{"enter", "resume"}, {"c", "custom"}, {"a", "account"}, {"f", "fork"},
			{"esc", "back"}, {"/", "search"}, {"q", "quit"},
		}
	}
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, helpKeyStyle.Render(pair[0])+" "+helpTextStyle.Render(pair[1]))
	}
	return clip("  "+strings.Join(parts, "    "), width)
}

func relativeTime(milliseconds int64) string {
	if milliseconds <= 0 {
		return "unknown"
	}
	age := time.Since(time.UnixMilli(milliseconds))
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	case age < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	default:
		return time.UnixMilli(milliseconds).Local().Format("Jan 02")
	}
}

func sessionCount(count int) string {
	if count == 1 {
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", count)
}

func clip(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) <= width {
		return value
	}
	return runewidth.Truncate(value, width, "…")
}

func pad(value string, width int) string {
	return runewidth.FillRight(clip(value, width), width)
}
