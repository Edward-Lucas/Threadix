package ui

import (
	"image/color"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// KoreanTheme is a custom theme that uses Malgun Gothic for Korean text support
// and provides a modern dark palette.
type KoreanTheme struct {
	variant fyne.ThemeVariant
	base    fyne.Theme
	font    fyne.Resource
}

// NewKoreanTheme creates a new theme with Korean font support.
func NewKoreanTheme() *KoreanTheme {
	t := &KoreanTheme{
		variant: theme.VariantDark,
		base:    theme.DefaultTheme(),
	}

	// Try to load Malgun Gothic from various paths
	fontPaths := []string{
		"resources/malgun.ttf",
		filepath.Join(getExeDir(), "resources", "malgun.ttf"),
		"C:\\Windows\\Fonts\\malgun.ttf",
	}

	for _, fp := range fontPaths {
		if data, err := os.ReadFile(fp); err == nil {
			t.font = fyne.NewStaticResource("malgun.ttf", data)
			break
		}
	}

	return t
}

func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func (t *KoreanTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 14, G: 18, B: 30, A: 255}
	case theme.ColorNameButton:
		return color.NRGBA{R: 52, G: 74, B: 120, A: 255}
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 88, G: 96, B: 110, A: 255}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{R: 54, G: 62, B: 80, A: 255}
	case theme.ColorNameError:
		return color.NRGBA{R: 220, G: 96, B: 97, A: 255}
	case theme.ColorNameFocus:
		return color.NRGBA{R: 110, G: 168, B: 218, A: 255}
	case theme.ColorNameHover:
		return color.NRGBA{R: 70, G: 98, B: 142, A: 255}
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 18, G: 24, B: 38, A: 255}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 224, G: 230, B: 239, A: 255}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 92, G: 140, B: 206, A: 255}
	case theme.ColorNamePressed:
		return color.NRGBA{R: 28, G: 58, B: 98, A: 255}
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 58, G: 70, B: 96, A: 255}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 28, G: 36, B: 56, A: 255}
	case theme.ColorNameShadow:
		return color.NRGBA{R: 0, G: 0, B: 0, A: 88}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 120, G: 136, B: 156, A: 255}
	case theme.ColorNameOverlayBackground:
		return color.NRGBA{R: 10, G: 14, B: 26, A: 220}
	default:
		return t.base.Color(name, variant)
	}
}

func (t *KoreanTheme) Font(style fyne.TextStyle) fyne.Resource {
	if t.font != nil {
		return t.font
	}
	return t.base.Font(style)
}

func (t *KoreanTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *KoreanTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
