package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"sshtui/internal/sshconfig"
)

type screen int

const (
	screenList screen = iota
	screenSearch
	screenForm
	screenDelete
	screenDiscard
)

type formMode int

const (
	modeAdd formMode = iota
	modeEdit
)

type model struct {
	path          string
	cfg           *sshconfig.Config
	entries       []sshconfig.Entry
	filtered      []int
	selected      int
	screen        screen
	status        string
	search        string
	mode          formMode
	originalAlias string
	inputs        []textinput.Model
	focus         int
	priorScreen   screen
	width         int
	height        int
}

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15"))
	sectionStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	labelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	focusedLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	selectedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	successStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func RunConfig(path string) error {
	cfg, err := sshconfig.Load(path)
	if err != nil {
		return err
	}
	m := newModel(path, cfg)
	_, err = tea.NewProgram(m).Run()
	return err
}

func newModel(path string, cfg *sshconfig.Config) model {
	m := model{path: path, cfg: cfg}
	m.refreshEntries()
	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenSearch:
		return m.viewList(true)
	case screenForm:
		return m.viewForm()
	case screenDelete:
		return m.viewDelete()
	case screenDiscard:
		return m.viewDiscard()
	default:
		return m.viewList(false)
	}
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screen == screenForm {
		return m.handleFormKey(msg)
	}

	switch m.screen {
	case screenList:
		return m.handleListKey(msg)
	case screenSearch:
		return m.handleSearchKey(msg)
	case screenDelete:
		return m.handleDeleteKey(msg)
	case screenDiscard:
		return m.handleDiscardKey(msg)
	default:
		return m, nil
	}
}

func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	case "enter":
		if entry, ok := m.selectedEntry(); ok {
			m.openForm(modeEdit, entry)
		}
	case "a":
		m.openForm(modeAdd, sshconfig.Entry{Port: "22"})
	case "d":
		if _, ok := m.selectedEntry(); ok {
			m.screen = screenDelete
		}
	case "/":
		m.search = ""
		m.screen = screenSearch
		m.applyFilter()
	case "r":
		m.reload()
	}
	return m, nil
}

func (m model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.search = ""
		m.screen = screenList
		m.applyFilter()
	case "enter":
		m.screen = screenList
	case "backspace":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
			m.applyFilter()
		}
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	default:
		if len(msg.String()) == 1 {
			m.search += msg.String()
			m.applyFilter()
		}
	}
	return m, nil
}

func (m model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		m.saveForm()
		return m, nil
	case "esc":
		if m.formChanged() {
			m.priorScreen = screenForm
			m.screen = screenDiscard
		} else {
			m.screen = screenList
		}
		return m, nil
	case "tab", "down":
		m.focusInput(1)
		return m, nil
	case "shift+tab", "up":
		m.focusInput(-1)
		return m, nil
	}

	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m model) handleDeleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entry, ok := m.selectedEntry()
	if !ok {
		m.screen = screenList
		return m, nil
	}
	switch msg.String() {
	case "y":
		if err := m.cfg.Delete(entry.Alias); err != nil {
			m.status = "Error: " + err.Error()
		} else if err := m.cfg.Save(); err != nil {
			m.status = "Error: " + err.Error()
		} else {
			m.status = fmt.Sprintf("Deleted %s.", entry.Alias)
			m.refreshEntries()
		}
		m.screen = screenList
	case "n", "esc":
		m.screen = screenList
	}
	return m, nil
}

func (m model) handleDiscardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		m.saveForm()
	case "d":
		m.screen = screenList
	case "c", "esc":
		m.screen = m.priorScreen
	}
	return m, nil
}

func (m *model) reload() {
	cfg, err := sshconfig.Load(m.path)
	if err != nil {
		m.status = "Error: " + err.Error()
		return
	}
	m.cfg = cfg
	m.refreshEntries()
	m.status = "Reloaded config."
}

func (m *model) refreshEntries() {
	m.entries = m.cfg.Entries()
	m.applyFilter()
}

func (m *model) applyFilter() {
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

func (m *model) moveSelection(delta int) {
	if len(m.filtered) == 0 {
		return
	}
	m.selected = (m.selected + delta + len(m.filtered)) % len(m.filtered)
}

func (m model) selectedEntry() (sshconfig.Entry, bool) {
	if len(m.filtered) == 0 || m.selected < 0 || m.selected >= len(m.filtered) {
		return sshconfig.Entry{}, false
	}
	return m.entries[m.filtered[m.selected]], true
}

func (m *model) openForm(mode formMode, entry sshconfig.Entry) {
	m.mode = mode
	m.originalAlias = ""
	if mode == modeEdit {
		m.originalAlias = entry.Alias
	}
	m.inputs = makeInputs(entry)
	m.focus = 0
	m.inputs[0].Focus()
	m.screen = screenForm
	m.status = ""
}

func makeInputs(entry sshconfig.Entry) []textinput.Model {
	values := []string{entry.Alias, entry.HostName, entry.User, entry.Port, entry.IdentityFile}
	placeholders := []string{"prod-api", "10.0.1.12", "ubuntu", "22", "~/.ssh/prod.pem"}
	inputs := make([]textinput.Model, len(values))
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].SetValue(values[i])
		inputs[i].Placeholder = placeholders[i]
		inputs[i].CharLimit = 256
		inputs[i].Prompt = ""
		inputs[i].TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		inputs[i].PlaceholderStyle = mutedStyle
		inputs[i].Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))
	}
	return inputs
}

func (m *model) focusInput(delta int) {
	m.inputs[m.focus].Blur()
	m.focus = (m.focus + delta + len(m.inputs)) % len(m.inputs)
	m.inputs[m.focus].Focus()
}

func (m model) formEntry() sshconfig.Entry {
	return sshconfig.Entry{
		Alias:        m.inputs[0].Value(),
		HostName:     m.inputs[1].Value(),
		User:         m.inputs[2].Value(),
		Port:         m.inputs[3].Value(),
		IdentityFile: m.inputs[4].Value(),
	}
}

func (m model) formChanged() bool {
	entry := m.formEntry()
	if m.mode == modeAdd {
		return strings.TrimSpace(entry.Alias+entry.HostName+entry.User+entry.IdentityFile) != "" || strings.TrimSpace(entry.Port) != "22"
	}
	if old, ok := m.findEntry(m.originalAlias); ok {
		return old != entry
	}
	return true
}

func (m model) findEntry(alias string) (sshconfig.Entry, bool) {
	for _, entry := range m.entries {
		if entry.Alias == alias {
			return entry, true
		}
	}
	return sshconfig.Entry{}, false
}

func (m *model) saveForm() {
	entry := m.formEntry()
	original := ""
	if m.mode == modeEdit {
		original = m.originalAlias
	}
	if err := m.cfg.Upsert(original, entry); err != nil {
		m.status = "Error: " + err.Error()
		m.screen = screenForm
		return
	}
	if err := m.cfg.Save(); err != nil {
		m.status = "Error: " + err.Error()
		m.screen = screenForm
		return
	}
	if m.mode == modeAdd {
		m.status = fmt.Sprintf("Added %s.", strings.TrimSpace(entry.Alias))
	} else {
		m.status = fmt.Sprintf("Saved %s.", strings.TrimSpace(entry.Alias))
	}
	m.refreshEntries()
	m.selectAlias(strings.TrimSpace(entry.Alias))
	m.screen = screenList
}

func (m *model) selectAlias(alias string) {
	for i, idx := range m.filtered {
		if m.entries[idx].Alias == alias {
			m.selected = i
			return
		}
	}
}

func (m model) viewList(searching bool) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("ssht config"))
	b.WriteString("\n\n")
	b.WriteString(sectionStyle.Render("Connections"))
	if searching {
		b.WriteString("  " + selectedStyle.Render("/"+m.search))
	}
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
	if searching {
		b.WriteString(mutedStyle.Render("Type search  Enter accept  Esc clear"))
	} else {
		b.WriteString(mutedStyle.Render("Enter edit  a add  d delete  / search  r reload  q quit"))
	}
	b.WriteString(m.statusLine())
	return b.String()
}

func (m model) viewForm() string {
	var b strings.Builder
	if m.mode == modeAdd {
		b.WriteString(titleStyle.Render("Add connection"))
	} else {
		b.WriteString(titleStyle.Render("Edit connection"))
	}
	b.WriteString("\n\n")
	labels := []string{"Alias", "HostName", "User", "Port", "IdentityFile"}
	for i, label := range labels {
		style := labelStyle
		cursor := "  "
		if i == m.focus {
			style = focusedLabelStyle
			cursor = "❯ "
		}
		b.WriteString(cursor)
		b.WriteString(style.Render(fmt.Sprintf("%-13s", label)))
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Ctrl+S save  Esc back  Tab next  Shift+Tab previous"))
	b.WriteString(m.statusLine())
	return b.String()
}

func (m model) viewDelete() string {
	entry, _ := m.selectedEntry()
	return fmt.Sprintf("%s\n\nThis removes it from ssht config only.\nSSH keys will not be deleted.\n\n%s%s",
		titleStyle.Render("Delete "+entry.Alias+"?"),
		mutedStyle.Render("y delete  n cancel  Esc cancel"),
		m.statusLine(),
	)
}

func (m model) viewDiscard() string {
	return fmt.Sprintf("%s\n\n%s%s",
		titleStyle.Render("Discard changes?"),
		mutedStyle.Render("s save and return  d discard and return  c cancel  Esc cancel"),
		m.statusLine(),
	)
}

func (m model) statusLine() string {
	if m.status == "" {
		return ""
	}
	style := mutedStyle
	if strings.HasPrefix(m.status, "Error:") {
		style = errorStyle
	} else if strings.HasPrefix(m.status, "Saved ") || strings.HasPrefix(m.status, "Added ") || strings.HasPrefix(m.status, "Deleted ") {
		style = successStyle
	}
	return "\n\n" + style.Render(m.status)
}

func userHost(entry sshconfig.Entry) string {
	if entry.User != "" && entry.HostName != "" {
		return entry.User + "@" + entry.HostName
	}
	if entry.HostName != "" {
		return entry.HostName
	}
	return "-"
}

func renderConnectionRow(entry sshconfig.Entry, selected bool) string {
	cursor := " "
	alias := lipgloss.NewStyle().Width(16).Render(entry.Alias)
	target := lipgloss.NewStyle().Width(24).Render(userHost(entry))
	port := entry.Port
	if port == "" {
		port = "-"
	}
	line := fmt.Sprintf("%s %s %s %s", cursor, alias, target, mutedStyle.Render(port))
	if selected {
		cursor = selectedStyle.Render("❯")
		alias = selectedStyle.Width(16).Render(entry.Alias)
		target = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Width(24).Render(userHost(entry))
		line = fmt.Sprintf("%s %s %s %s", cursor, alias, target, mutedStyle.Render(port))
	}
	return line
}
