package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sshtui/internal/sshconfig"
)

func TestConnectSearchAllowsRWhenQueryActive(t *testing.T) {
	cfg, err := sshconfig.Parse("", []byte(`Host prod
  HostName prod.example

Host staging
  HostName staging.example
`))
	if err != nil {
		t.Fatal(err)
	}

	model := newConnectModel("", cfg)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	model = updated.(connectModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	model = updated.(connectModel)

	if model.search != "pr" {
		t.Fatalf("search = %q, want %q", model.search, "pr")
	}
	if len(model.filtered) != 1 {
		t.Fatalf("filtered count = %d, want 1", len(model.filtered))
	}
	entry, ok := model.selectedEntry()
	if !ok || entry.Alias != "prod" {
		t.Fatalf("selected entry = %+v, ok = %v; want prod", entry, ok)
	}
}
