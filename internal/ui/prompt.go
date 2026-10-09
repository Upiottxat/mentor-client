package ui

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
)

// ErrNotInteractive is returned by prompt helpers when the client is running
// without a terminal. Callers should fall back to static rendering.
var ErrNotInteractive = errors.New("interactive terminal unavailable")

func runForm(fields ...huh.Field) error {
	if !Opts.Interactive || len(fields) == 0 {
		return ErrNotInteractive
	}
	form := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCharm())
	return form.Run()
}

// Select presents a single-choice list and returns the chosen index.
func Select(title string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, ErrNotInteractive
	}

	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(fmt.Sprintf("%d) %s", i+1, o), i)
	}

	choice := -1
	field := huh.NewSelect[int]().
		Title(title).
		Options(opts...).
		Value(&choice)

	if err := runForm(field); err != nil {
		return -1, err
	}
	return choice, nil
}

// Confirm asks a yes/no question.
func Confirm(title, affirmative, negative string) (bool, error) {
	value := true
	field := huh.NewConfirm().
		Title(title).
		Affirmative(affirmative).
		Negative(negative).
		Value(&value)

	if err := runForm(field); err != nil {
		return false, err
	}
	return value, nil
}

// Input prompts for a single line of text.
func Input(title, placeholder string) (string, error) {
	value := ""
	field := huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Value(&value)

	if err := runForm(field); err != nil {
		return "", err
	}
	return value, nil
}
