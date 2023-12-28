package lipgloss

import (
	"strconv"

	"github.com/charmbracelet/lipgloss/jzazbz"
	"github.com/muesli/termenv"
)

// DynamicColor is a color that changes depending on its position on the screen.
type DynamicColor interface {
	dColor(r *Renderer, x, y, xMax, yMax int) termenv.Color
}

// TerminalColor is a color intended to be rendered in the terminal.
type TerminalColor interface {
	color(*Renderer) termenv.Color
	RGBA() (r, g, b, a uint32)
}

func isDynamic(tc TerminalColor) bool {
	_, ok := tc.(DynamicColor)
	return ok
}

var noColor = NoColor{}

// NoColor is used to specify the absence of color styling. When this is active
// foreground colors will be rendered with the terminal's default text color,
// and background colors will not be drawn at all.
//
// Example usage:
//
//	var style = someStyle.Copy().Background(lipgloss.NoColor{})
type NoColor struct{}

func (NoColor) color(*Renderer) termenv.Color {
	return termenv.NoColor{}
}

// RGBA returns the RGBA value of this color. Because we have to return
// something, despite this color being the absence of color, we're returning
// black with 100% opacity.
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
//
// Deprecated.
func (n NoColor) RGBA() (r, g, b, a uint32) {
	return 0x0, 0x0, 0x0, 0xFFFF //nolint:gomnd
}

// Color specifies a color by hex or ANSI value. For example:
//
//	ansiColor := lipgloss.Color("21")
//	hexColor := lipgloss.Color("#0000ff")
type Color string

func (c Color) color(r *Renderer) termenv.Color {
	return r.ColorProfile().Color(string(c))
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface. Note that on error we return black with 100% opacity, or:
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
//
// Deprecated.
func (c Color) RGBA() (r, g, b, a uint32) {
	return termenv.ConvertToRGB(c.color(renderer)).RGBA()
}

// ANSIColor is a color specified by an ANSI color value. It's merely syntactic
// sugar for the more general Color function. Invalid colors will render as
// black.
//
// Example usage:
//
//	// These two statements are equivalent.
//	colorA := lipgloss.ANSIColor(21)
//	colorB := lipgloss.Color("21")
type ANSIColor uint

func (ac ANSIColor) color(r *Renderer) termenv.Color {
	return Color(strconv.FormatUint(uint64(ac), 10)).color(r)
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface. Note that on error we return black with 100% opacity, or:
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
//
// Deprecated.
func (ac ANSIColor) RGBA() (r, g, b, a uint32) {
	cf := Color(strconv.FormatUint(uint64(ac), 10))
	return cf.RGBA()
}

// AdaptiveColor provides color options for light and dark backgrounds. The
// appropriate color will be returned at runtime based on the darkness of the
// terminal background color.
//
// Example usage:
//
//	color := lipgloss.AdaptiveColor{Light: "#0000ff", Dark: "#000099"}
type AdaptiveColor struct {
	Light string
	Dark  string
}

func (ac AdaptiveColor) color(r *Renderer) termenv.Color {
	if r.HasDarkBackground() {
		return Color(ac.Dark).color(r)
	}
	return Color(ac.Light).color(r)
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface. Note that on error we return black with 100% opacity, or:
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
//
// Deprecated.
func (ac AdaptiveColor) RGBA() (r, g, b, a uint32) {
	return termenv.ConvertToRGB(ac.color(renderer)).RGBA()
}

// CompleteColor specifies exact values for truecolor, ANSI256, and ANSI color
// profiles. Automatic color degradation will not be performed.
type CompleteColor struct {
	TrueColor string
	ANSI256   string
	ANSI      string
}

func (c CompleteColor) color(r *Renderer) termenv.Color {
	p := r.ColorProfile()
	switch p { //nolint:exhaustive
	case termenv.TrueColor:
		return p.Color(c.TrueColor)
	case termenv.ANSI256:
		return p.Color(c.ANSI256)
	case termenv.ANSI:
		return p.Color(c.ANSI)
	default:
		return termenv.NoColor{}
	}
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface. Note that on error we return black with 100% opacity, or:
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
// CompleteAdaptiveColor specifies exact values for truecolor, ANSI256, and ANSI color
//
// Deprecated.
func (c CompleteColor) RGBA() (r, g, b, a uint32) {
	return termenv.ConvertToRGB(c.color(renderer)).RGBA()
}

// CompleteAdaptiveColor specifies exact values for truecolor, ANSI256, and ANSI color
// profiles, with separate options for light and dark backgrounds. Automatic
// color degradation will not be performed.
type CompleteAdaptiveColor struct {
	Light CompleteColor
	Dark  CompleteColor
}

func (cac CompleteAdaptiveColor) color(r *Renderer) termenv.Color {
	if r.HasDarkBackground() {
		return cac.Dark.color(r)
	}
	return cac.Light.color(r)
}

// RGBA returns the RGBA value of this color. This satisfies the Go Color
// interface. Note that on error we return black with 100% opacity, or:
//
// Red: 0x0, Green: 0x0, Blue: 0x0, Alpha: 0xFFFF.
//
// Deprecated.
func (cac CompleteAdaptiveColor) RGBA() (r, g, b, a uint32) {
	return termenv.ConvertToRGB(cac.color(renderer)).RGBA()
}

// GradientColor is a dynamic color that creates a perceptually-smooth
// transition between Stops, with optional Offsets between 0 and 1.
// If provided, Offsets must be sorted and must satisfy the following
// invariant: 0 <= Offsets[n] < Offsets[n+1] <=1
//
// Invalid colors or offsets will result in a black gradient.
type GradientColor struct {
	g       *jzazbz.Gradient
	Stops   []string
	Offsets []float64
}

func (gc *GradientColor) init() {
	if gc.g != nil {
		return
	}
	gc.g = jzazbz.NewGradient(gc.Stops, gc.Offsets)
	// // TODO: invariant check and clamp if Stops[0] > 0 || Stops[n] < 1.0
	// // TODO: jzazbz.NewGradient(gc.Stops, gc.Offsets)
	// g := &jzazbz.Gradient{
	// 	Stops: make([]jzazbz.Stop, len(gc.Stops)),
	// }
	// for i := range g.Stops {
	// 	g.Stops[i].Color = jzazbz.FromHex(gc.Stops[i])
	// 	// TODO: not good enough to ensure sorted invariant
	// 	if len(gc.Offsets) > i {
	// 		g.Stops[i].Offset = gc.Offsets[i]
	// 	} else {
	// 		g.Stops[i].Offset = 1.0 / float64(len(gc.Stops)) * float64(i)
	// 	}
	// }
	// gc.g = g
}

func (gc *GradientColor) RGBA() (r, g, b, a uint32) {
	gc.init()
	return gc.g.Color(0).RGBA()
}

func (gc *GradientColor) color(r *Renderer) termenv.Color {
	gc.init()
	return r.colorProfile.Color(gc.g.Color(0).Hex())
}

func (gc *GradientColor) dColor(r *Renderer, x, y, xMax, yMax int) termenv.Color {
	gc.init()
	return r.colorProfile.Color(gc.g.ColorAt(x, xMax).Hex())
}
