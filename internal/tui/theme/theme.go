package theme

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// ── Surface colors ──────────────────────────────────────────────────────────

var (
	colSurface          = lipgloss.Color("#0d0d1c")
	colSurfaceContainer = lipgloss.Color("#18182a")
	colSurfaceBright    = lipgloss.Color("#2a2a42")
)

// ── Content colors ──────────────────────────────────────────────────────────

var (
	colOnSurface        = lipgloss.Color("#e6e3fa")
	colOnSurfaceVariant = lipgloss.Color("#c9c5dd")
	colOutlineVariant   = lipgloss.Color("#474658")
)

// ── Accent colors ───────────────────────────────────────────────────────────

var (
	colPrimary   = lipgloss.Color("#d1abfd")
	colSecondary = lipgloss.Color("#8cb7fe")
	colGreen     = lipgloss.Color("#a6e3a1")
	colYellow    = lipgloss.Color("#f9e2af")
	colPeach     = lipgloss.Color("#fab387")
	colRed       = lipgloss.Color("#f38ba8")
)

// ── Dim accent colors (for pulsing) ────────────────────────────────────────

var (
	colPeachDim = lipgloss.Color("#7d5944")
	colRedDim   = lipgloss.Color("#7a4654")
)

// ── Gradient stops for progress bars ────────────────────────────────────────

var (
	gradGreen  = mustParseHex("#a6e3a1")
	gradYellow = mustParseHex("#f9e2af")
	gradPeach  = mustParseHex("#fab387")
	gradRed    = mustParseHex("#f38ba8")
)

func mustParseHex(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		panic("invalid color: " + hex)
	}
	return c
}

// GradientColor returns the interpolated color for a percentage value (0-100).
func GradientColor(pct float64) lipgloss.Color {
	var c colorful.Color

	switch {
	case pct <= 50:
		t := pct / 50.0
		c = gradGreen.BlendRgb(gradYellow, t)
	case pct <= 70:
		t := (pct - 50) / 20.0
		c = gradYellow.BlendRgb(gradPeach, t)
	case pct <= 90:
		t := (pct - 70) / 20.0
		c = gradPeach.BlendRgb(gradRed, t)
	default:
		c = gradRed
	}

	return lipgloss.Color(c.Hex())
}

// ── Label style helpers ─────────────────────────────────────────────────────

// LetterSpace inserts spaces between characters for the "editorial" label look.
func LetterSpace(s string) string {
	runes := []rune(s)
	result := make([]rune, 0, len(runes)*2)
	for i, r := range runes {
		result = append(result, r)
		if i < len(runes)-1 {
			result = append(result, ' ')
		}
	}
	return string(result)
}

// ── Base styles ─────────────────────────────────────────────────────────────

var (
	// SurfaceBase is the root background.
	SurfaceBase = lipgloss.NewStyle().Background(colSurface)

	// SurfaceContainer is for major section backgrounds.
	SurfaceContainer = lipgloss.NewStyle().Background(colSurfaceContainer)

	// SurfaceBright is for elevated panels (burn rate).
	SurfaceBright = lipgloss.NewStyle().Background(colSurfaceBright)
)

// ── Typography styles ───────────────────────────────────────────────────────

var (
	// LabelMD is the small uppercase label style.
	LabelMD = lipgloss.NewStyle().
		Foreground(colOnSurfaceVariant)

	// HeadlineMD is the section headline style.
	HeadlineMD = lipgloss.NewStyle().
			Foreground(colOnSurface).
			Bold(true)

	// BodyLG is the primary text style.
	BodyLG = lipgloss.NewStyle().
		Foreground(colOnSurface)

	// ValueBold is for large data values.
	ValueBold = lipgloss.NewStyle().
			Foreground(colOnSurface).
			Bold(true)

	// ValueGreen is for token velocity.
	ValueGreen = lipgloss.NewStyle().
			Foreground(colGreen).
			Bold(true)

	// PrimaryText is for accented headers.
	PrimaryText = lipgloss.NewStyle().
			Foreground(colPrimary)

	// DimText is for metadata and low-importance text.
	DimText = lipgloss.NewStyle().
		Foreground(colOutlineVariant)
)

// ── Status chip styles ──────────────────────────────────────────────────────

var (
	StatusWorkingStyle = lipgloss.NewStyle().Foreground(colSecondary)
	StatusWaitingStyle = lipgloss.NewStyle().Foreground(colPeach)
	StatusBlockedStyle = lipgloss.NewStyle().Foreground(colRed)
	StatusZombieStyle  = lipgloss.NewStyle().Foreground(colOutlineVariant)
	StatusIdleStyle    = lipgloss.NewStyle().Foreground(colOnSurfaceVariant)

	StatusWaitingDimStyle = lipgloss.NewStyle().Foreground(colPeachDim)
	StatusBlockedDimStyle = lipgloss.NewStyle().Foreground(colRedDim)
)

// ── Spinner frames ──────────────────────────────────────────────────────────

var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ── Size warning style ──────────────────────────────────────────────────────

var SizeWarning = lipgloss.NewStyle().
	Foreground(colOnSurfaceVariant).
	Bold(true).
	Align(lipgloss.Center)

// ── Color accessors for use in components that need raw lipgloss.Color values ─

func ColPrimary() lipgloss.Color          { return colPrimary }
func ColSecondary() lipgloss.Color        { return colSecondary }
func ColGreen() lipgloss.Color            { return colGreen }
func ColYellow() lipgloss.Color           { return colYellow }
func ColPeach() lipgloss.Color            { return colPeach }
func ColRed() lipgloss.Color              { return colRed }
func ColOnSurface() lipgloss.Color        { return colOnSurface }
func ColOnSurfaceVariant() lipgloss.Color { return colOnSurfaceVariant }
func ColSurfaceContainer() lipgloss.Color { return colSurfaceContainer }
func ColSurface() lipgloss.Color          { return colSurface }
func ColSurfaceBright() lipgloss.Color    { return colSurfaceBright }
func ColOutlineVariant() lipgloss.Color   { return colOutlineVariant }
