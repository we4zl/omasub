package tui

import "testing"

func TestPaletteFromOmarchyColors(t *testing.T) {
	colors := parseThemeColors("accent\t#112233\nforeground #fefefe\ngreen #00aa00\n")
	got := paletteFromColors(colors, fallbackPalette())
	if string(got.accent) != "#112233" || string(got.foreground) != "#fefefe" || string(got.success) != "#00aa00" {
		t.Fatalf("palette = %#v", got)
	}
}

func TestPaletteFallsBackForMissingColors(t *testing.T) {
	fallback := fallbackPalette()
	got := paletteFromColors(nil, fallback)
	if got != fallback {
		t.Fatalf("palette = %#v, want %#v", got, fallback)
	}
}
