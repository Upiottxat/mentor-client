// Package ui provides the shared visual language for mentor-client: adaptive
// colour styles, gradient banners, responsive text layout, and interactive
// primitives (typewriter narration, pagination, and huh-backed prompts).
//
// Every interactive primitive degrades gracefully to plain, uncoloured output
// when stdout/stdin are not terminals or when interactivity is disabled, so the
// client stays safe to pipe and script.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/term"
)

// Options controls the interactive capabilities of the client.
type Options struct {
	Interactive bool
	Animation   bool
	Pagination  bool
}

// Opts is the process-wide UI option set. It is configured once at startup.
var Opts = Options{Interactive: true, Animation: true, Pagination: true}

// Configure resolves the requested flags against real terminal capabilities.
// Any capability requiring a TTY is automatically disabled when the client is
// not attached to one, guaranteeing clean piped output.
func Configure(interactive, animation, pagination bool) {
	outTTY := term.IsTerminal(int(os.Stdout.Fd()))
	inTTY := term.IsTerminal(int(os.Stdin.Fd()))

	Opts.Interactive = interactive && outTTY && inTTY
	Opts.Animation = animation && Opts.Interactive
	Opts.Pagination = pagination && Opts.Interactive
}

// Interactive reports whether rich, TTY-bound rendering is available.
func Interactive() bool { return Opts.Interactive }

// Adaptive palette shared across every surface.
var (
	colPrimary = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"}
	colAccent  = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"}
	colSuccess = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	colWarning = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	colDanger  = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	colMuted   = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	colInfo    = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
)

// Base text styles.
var (
	TitleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colPrimary)
	AccentStyle  = lipgloss.NewStyle().Foreground(colAccent)
	MutedStyle   = lipgloss.NewStyle().Foreground(colMuted)
	InfoStyle    = lipgloss.NewStyle().Foreground(colInfo)
	SuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(colSuccess)
	WarnStyle    = lipgloss.NewStyle().Bold(true).Foreground(colWarning)
	DangerStyle  = lipgloss.NewStyle().Bold(true).Foreground(colDanger)
	BoldStyle    = lipgloss.NewStyle().Bold(true)
	CodeStyle    = lipgloss.NewStyle().Foreground(colSuccess)
)

// Container styles.
var (
	NarratorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 2).
			MarginTop(1).
			MarginBottom(1)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(colMuted).
			Padding(0, 2).
			MarginBottom(1)

	BossPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(colDanger).
			Padding(1, 3).
			MarginTop(1).
			MarginBottom(1)

	BadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#111111")).
			Background(colWarning).
			Padding(0, 1)

	SuccessBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04120A")).
			Background(colSuccess).
			Padding(0, 1)

	DangerBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1A0505")).
			Background(colDanger).
			Padding(0, 1)

	HintStyle = lipgloss.NewStyle().
			Foreground(colWarning).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colWarning).
			PaddingLeft(1)
)

// Width returns a comfortable rendering width clamped to the terminal size.
func Width() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	if w > 100 {
		return 100
	}
	if w < 40 {
		return 40
	}
	return w
}

// Gradient renders s with a smooth horizontal colour ramp from one hex colour
// to another. When colours are stripped (non-TTY) the text is returned as-is.
func Gradient(s, from, to string) string {
	c1, err1 := colorful.Hex(from)
	c2, err2 := colorful.Hex(to)
	if err1 != nil || err2 != nil {
		return s
	}

	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return ""
	}

	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if n > 1 {
			t = float64(i) / float64(n-1)
		}
		c := c1.BlendLuv(c2, t).Clamped()
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex())).Render(string(r)))
	}
	return b.String()
}

// Banner renders a gradient headline in a double-lined frame with an optional
// muted subtitle.
func Banner(title, subtitle string) string {
	width := Width() - 2
	if width < 20 {
		width = 20
	}

	head := Gradient(title, "#22D3EE", "#A78BFA")
	content := lipgloss.NewStyle().Bold(true).Render(head)
	if subtitle != "" {
		content += "\n" + MutedStyle.Render(subtitle)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colPrimary).
		Padding(0, 3).
		Width(width).
		MarginBottom(1).
		Render(content)
}

// Rule renders a full-width divider, optionally centred around a label.
func Rule(label string) string {
	width := Width()
	if label == "" {
		return MutedStyle.Render(strings.Repeat("─", width))
	}

	lbl := " " + label + " "
	pad := width - lipgloss.Width(lbl)
	if pad < 2 {
		return TitleStyle.Render(label)
	}
	left := pad / 2
	right := pad - left
	return MutedStyle.Render(strings.Repeat("─", left)) +
		TitleStyle.Render(lbl) +
		MutedStyle.Render(strings.Repeat("─", right))
}

// Section renders a titled section header with a short underline.
func Section(title string) string {
	under := strings.Repeat("─", min(Width(), lipgloss.Width(title)+4))
	return "\n" + TitleStyle.Render(title) + "\n" + MutedStyle.Render(under)
}

// Wrap word-wraps text to the render width (emoji/ANSI aware).
func Wrap(s string) string {
	return wordwrap.String(s, Width())
}

// Bullet renders a single bullet list row.
func Bullet(text string) string {
	return "  " + AccentStyle.Render("•") + " " + Wrap(text)
}

// KeyVal renders an aligned "Label: value" row.
func KeyVal(label, value string) string {
	pad := 14
	l := label + ":"
	if len(l) < pad {
		l += strings.Repeat(" ", pad-len(l))
	}
	return MutedStyle.Render(l) + value
}

// Badge wraps text in the warning badge style.
func Badge(text string) string { return BadgeStyle.Render(text) }

// Success wraps text in the success badge style.
func Success(text string) string { return SuccessBadge.Render(text) }

// Danger wraps text in the danger badge style.
func Danger(text string) string { return DangerBadge.Render(text) }

// Println writes a line to stdout.
func Println(args ...any) { fmt.Fprintln(os.Stdout, args...) }

// Printf writes formatted output to stdout.
func Printf(format string, args ...any) { fmt.Fprintf(os.Stdout, format, args...) }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
