package ui

import (
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Narrate reveals narrator text character-by-character inside a quote box. Any
// keypress skips to the end. Falls back to an immediate styled print when
// animation is unavailable.
func Narrate(text string) {
	if text == "" {
		return
	}

	render := func(partial string) string {
		return NarratorBox.
			Width(Width() - NarratorBox.GetHorizontalFrameSize()).
			Render(AccentStyle.Render("🎙️  NARRATOR") + "\n\n" +
				lipgloss.NewStyle().Italic(true).Render(partial))
	}

	if !Opts.Animation {
		Println(render(text))
		return
	}

	m := &typewriterModel{full: text, speed: 16 * time.Millisecond, render: render}
	if _, err := tea.NewProgram(m, tea.WithOutput(nil)).Run(); err != nil {
		Println(render(text))
		return
	}
	Println(render(text))
}

// Typewriter reveals an arbitrary string, character-by-character, using style.
func Typewriter(text string, style lipgloss.Style) {
	styled := style.Width(Width())
	render := func(partial string) string { return styled.Render(partial) }

	if !Opts.Animation {
		Println(render(text))
		return
	}

	m := &typewriterModel{full: text, speed: 16 * time.Millisecond, render: render}
	if _, err := tea.NewProgram(m, tea.WithOutput(nil)).Run(); err != nil {
		Println(render(text))
		return
	}
	Println(render(text))
}

// Pause waits for the user to acknowledge before continuing. It is a no-op when
// pagination is disabled, keeping piped runs non-blocking.
func Pause(label string) {
	if label == "" {
		label = "Press [enter] to continue"
	}
	if !Opts.Pagination {
		return
	}

	p := &pauseModel{prompt: MutedStyle.Render("▸ " + label + " ")}
	if _, err := tea.NewProgram(p, tea.WithOutput(nil)).Run(); err != nil {
		return
	}
}

type typewriterModel struct {
	full   string
	shown  int
	speed  time.Duration
	render func(string) string
}

type twTick struct{}

func (m *typewriterModel) Init() tea.Cmd {
	return tea.Tick(m.speed, func(time.Time) tea.Msg { return twTick{} })
}

func (m *typewriterModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		m.shown = len([]rune(m.full))
		return m, tea.Quit
	case twTick:
		if m.shown >= len([]rune(m.full)) {
			return m, tea.Quit
		}
		m.shown++
		return m, tea.Tick(m.speed, func(time.Time) tea.Msg { return twTick{} })
	}
	return m, nil
}

func (m *typewriterModel) View() string {
	runes := []rune(m.full)
	shown := m.shown
	if shown > len(runes) {
		shown = len(runes)
	}
	return m.render(string(runes[:shown]))
}

type pauseModel struct {
	prompt string
}

func (m *pauseModel) Init() tea.Cmd { return nil }

func (m *pauseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		return m, tea.Quit
	}
	return m, nil
}

func (m *pauseModel) View() string { return m.prompt }
