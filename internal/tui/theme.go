package tui

import (
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type themePalette struct {
	accent     lipgloss.Color
	secondary  lipgloss.Color
	success    lipgloss.Color
	warning    lipgloss.Color
	foreground lipgloss.Color
	muted      lipgloss.Color
	surface    lipgloss.Color
	border     lipgloss.Color
	canvas     lipgloss.Color
	gradientA  lipgloss.Color
	gradientB  lipgloss.Color
	gradientC  lipgloss.Color
}

func loadOmarchyPalette() themePalette {
	palette, _ := loadOmarchyPaletteSnapshot()
	return palette
}

func loadOmarchyPaletteSnapshot() (themePalette, string) {
	fallback := fallbackPalette()
	output, err := exec.Command("omarchy-theme-color", "--all").Output()
	if err != nil {
		return fallback, "fallback"
	}
	text := string(output)
	return paletteFromColors(parseThemeColors(text), fallback), text
}

func parseThemeColors(output string) map[string]string {
	colors := make(map[string]string)
	for line := range strings.Lines(output) {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasPrefix(fields[1], "#") {
			colors[fields[0]] = fields[1]
		}
	}
	return colors
}

func paletteFromColors(colors map[string]string, fallback themePalette) themePalette {
	return themePalette{
		accent:     themeColor(colors, "accent", fallback.accent),
		secondary:  themeColor(colors, "magenta", fallback.secondary),
		success:    themeColor(colors, "green", fallback.success),
		warning:    themeColor(colors, "yellow", fallback.warning),
		foreground: themeColor(colors, "foreground", fallback.foreground),
		muted:      themeColor(colors, "muted", fallback.muted),
		surface:    themeColor(colors, "lighter_background", fallback.surface),
		border:     themeColor(colors, "blue", fallback.border),
		canvas:     themeColor(colors, "background", fallback.canvas),
		gradientA:  themeColor(colors, "blue", fallback.gradientA),
		gradientB:  themeColor(colors, "magenta", fallback.gradientB),
		gradientC:  themeColor(colors, "yellow", fallback.gradientC),
	}
}

func themeColor(colors map[string]string, key string, fallback lipgloss.Color) lipgloss.Color {
	if value := strings.TrimSpace(colors[key]); value != "" {
		return lipgloss.Color(value)
	}
	return fallback
}

func fallbackPalette() themePalette {
	return themePalette{
		accent:     lipgloss.Color("4"),
		secondary:  lipgloss.Color("5"),
		success:    lipgloss.Color("2"),
		warning:    lipgloss.Color("3"),
		foreground: lipgloss.Color("7"),
		muted:      lipgloss.Color("8"),
		surface:    lipgloss.Color("0"),
		border:     lipgloss.Color("4"),
		canvas:     lipgloss.Color("0"),
		gradientA:  lipgloss.Color("4"),
		gradientB:  lipgloss.Color("5"),
		gradientC:  lipgloss.Color("3"),
	}
}
