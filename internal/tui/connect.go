package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"sshtui/internal/sshconfig"
)

type connectModel struct {
	path     string
	cfg      *sshconfig.Config
	entries  []sshconfig.Entry
	filtered []int
	selected int
	search   string
	status   string
	chosen   string
}

func RunConnectSelector(path string) (string, bool, error) {
	cfg, err := sshconfig.Load(path)
	if err != nil {
		return "", false, err
	}
	m := newConnectModel(path, cfg)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", false, err
	}
	result := final.(connectModel)
	return result.chosen, result.chosen != "", nil
}

func newConnectModel(path string, cfg *sshconfig.Config) connectModel {
	m := connectModel{path: path, cfg: cfg}
	m.refresh()
	return m
}

func (m connectModel) Init() tea.Cmd {
	return nil
}

func (m connectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			m.move(-1)
		case "down", "j":
			m.move(1)
		case "enter":
			if entry, ok := m.selectedEntry(); ok {
				m.chosen = entry.Alias
				return m, tea.Quit
			}
		case "backspace":
			if len(m.search) > 0 {
				m.search = m.search[:len(m.search)-1]
				m.applyFilter()
			}
		case "r":
			if m.search == "" {
				m.reload()
				break
			}
			m.search += msg.String()
			m.applyFilter()
		default:
			if len(msg.String()) == 1 {
				m.search += msg.String()
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m connectModel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("sshtui"))
	if m.search != "" {
		b.WriteString("  " + selectedStyle.Render("/"+m.search))
	}
	b.WriteString("\n\n")
	b.WriteString(sectionStyle.Render("Connections"))
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(mutedStyle.Render("No connections"))
		b.WriteString("\n")
	} else {
		for i, idx := range m.filtered {
			entry := m.entries[idx]
			b.WriteString(renderConnectionRow(entry, i == m.selected))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Enter connect  type search  r reload  q quit"))
	if m.status != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(m.status))
	}
	return b.String()
}

func (m *connectModel) refresh() {
	m.entries = m.cfg.Entries()
	m.applyFilter()
}

func (m *connectModel) applyFilter() {
	m.filtered = m.filtered[:0]
	query := strings.ToLower(strings.TrimSpace(m.search))
	for i, entry := range m.entries {
		haystack := strings.ToLower(entry.Alias + " " + entry.User + " " + entry.HostName)
		if query == "" || strings.Contains(haystack, query) {
			m.filtered = append(m.filtered, i)
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = len(m.filtered) - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *connectModel) move(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.selected = (m.selected + delta + len(m.filtered)) % len(m.filtered)
}

func (m connectModel) selectedEntry() (sshconfig.Entry, bool) {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return sshconfig.Entry{}, false
	}
	return m.entries[m.filtered[m.selected]], true
}

func (m *connectModel) reload() {
	cfg, err := sshconfig.Load(m.path)
	if err != nil {
		m.status = "Error: " + err.Error()
		return
	}
	m.cfg = cfg
	m.refresh()
	m.status = ""
}
