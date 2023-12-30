/*
Package jzazbz implements color math in the JzAzBz color space.

JzAzBz was designed to provide a color space capable of predicting both small
and large color differences and accurate lightness values in high dynamic range
environments with minimum computational cost. It is perceptually uniform over a
wide gamut, and linear in iso-hue directions, allowing for more natural color
gradients. Colors in this space are calculated relative to standard illuminant
D65, and are comprised of three components:

	+-----------+-------------+---------+
	| Component | Description |  Range  |
	+-----------+-------------+---------+
	| j         | lightness   | [ 0, 1] |
	| a         | green-red   | [-1, 1] |
	| b         | blue-yellow | [-1, 1] |
	+-----------+-------------+---------+

JzAzBz reference paper:
https://opg.optica.org/oe/fulltext.cfm?uri=oe-25-13-15131&id=368272

JzAzBz implementation based on the Javascript at
https://observablehq.com/@jrus/jzazbz

Supplemental sRGB -> lRGB transformation math from:
https://entropymine.com/imageworsener/srgbformula/

Precomputed lRGB -> CIE XYZ matrices from:
http://www.brucelindbloom.com/index.html?Eqn_RGB_to_XYZ.html
*/
package jzazbz

import (
	"fmt"
	"math"
	"sort"
)

// BadGradient is returned if the provided gradient configuration is incorrect.
var BadGradient = &Gradient{}

func black() *Color {
	return &Color{0, 0, 0}
}

// Gradient is a multi-stop color gradient in the JzAzBz color space. Colors are
// interpolated using a basic linear interpolation along each component axis.
type Gradient struct {
	stops []stop
}

// NewGradient creates a new color gradient from one or more color stops in sRGB
// hex format (#RRGGBB), along with an optional list of color stop offsets
// between 0 and 1. If provided, offsets must be a sorted list of float64
// offsets between 0 and 1, and must be the same length as stops. If offsets are
// not provided, colors will be spread evenly across the gradient with stops[0]
// at offset 0 and stops[len(stops)-1] at offset 1.
// Invalid color stops will be replaced with black. Invalid offsets will return
// [BadGradient]
func NewGradient(stops []string, offsets []float64) *Gradient {
	if len(stops) == 0 {
		return BadGradient
	}
	if len(offsets) > 0 {
		switch {
		case len(offsets) != len(stops):
			fallthrough
		// TODO: slices.IsSorted @ go1.20
		case !sort.IsSorted(sort.Float64Slice(offsets)):
			return BadGradient
		}
	} else {
		offsets = make([]float64, len(stops))
		offsets[len(offsets)-1] = 1.0
		for i := 1; i < len(offsets)-1; i++ {
			offsets[i] = 1.0 / float64(len(offsets)) * float64(i)
		}
	}
	g := &Gradient{
		stops: make([]stop, len(stops)),
	}
	for i := range stops {
		g.stops[i] = stop{
			Color:  FromHex(stops[i]),
			Offset: offsets[i],
		}
	}
	return g
}

func (g *Gradient) Color(idx int) *Color {
	if len(g.stops) > idx {
		return g.stops[idx].Color
	}
	return black()
}

func (g *Gradient) ColorAt(pos, max int) *Color {
	switch len(g.stops) {
	case 0:
		return black()
	case 1:
		return g.stops[0].Color
	}

	switch pos {
	case 0:
		return g.stops[0].Color
	case max:
		return g.stops[len(g.stops)-1].Color
	}
	f := float64(pos) / float64(max)
	var s int
	for s = 0; s < len(g.stops); s++ {
		if f < g.stops[s].Offset {
			break
		}
	}
	switch s {
	case 0:
		return g.stops[0].Color
	case len(g.stops):
		return g.stops[len(g.stops)-1].Color
	}
	// normalize 0.0-1.0 between stops
	f = (f - g.stops[s-1].Offset) / (g.stops[s].Offset - g.stops[s-1].Offset)
	return (g.stops[s-1].Color.blend(g.stops[s].Color, f))
}

// stop is a gradient stop. Offset is [0,1]
type stop struct {
	Color  *Color
	Offset float64
}

// Color is a color value in the JzAzBz color space.
type Color struct {
	j float64
	a float64
	b float64
}

func (c *Color) Hex() string {
	r, g, b := c.sRGB()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r), uint8(g), uint8(b))
}

// RGBA returns the RGBA value of this color.
func (c *Color) RGBA() (r, g, b, a uint32) {
	sR, sG, sB := c.sRGB()
	return uint32(sR * 65535.0),
		uint32(sG * 65535.0),
		uint32(sB * 65535.0),
		0xFFFF
}

func (c *Color) sRGB() (r, g, b float64) {
	// JzAzBz -> L'M'S'
	jz := c.j + 1.6295499532821566e-11
	iz := jz / (0.44 + 0.56*jz)
	l := pqInv(iz + 1.386050432715393e-1*c.a + 5.804731615611869e-2*c.b)
	m := pqInv(iz - 1.386050432715393e-1*c.a - 5.804731615611891e-2*c.b)
	s := pqInv(iz - 9.601924202631895e-2*c.a - 8.118918960560390e-1*c.b)
	// L'M'S' -> CIE XYZ
	x := 1.661373055774069e+00*l - 9.145230923250668e-01*m + 2.313620767186147e-01*s
	y := -3.250758740427037e-01*l + 1.571847038366936e+00*m - 2.182538318672940e-01*s
	z := -9.098281098284756e-02*l - 3.127282905230740e-01*m + 1.522766561305260e+00*s
	// CIE XYZ -> lRGB -> sRGB
	r = math.Round(255 * rgbLinearToStandard(3.2404542*x-1.5371385*y-0.4985314*z))
	g = math.Round(255 * rgbLinearToStandard(-0.9692660*x+1.8760108*y+0.0415560*z))
	b = math.Round(255 * rgbLinearToStandard(0.0556434*x-0.2040259*y+1.0572252*z))
	return r, g, b
}

// FromHex converts a hex color in the form of #RRGGBB to JzAzBz
// invalid colors will result in black.
func FromHex(hexStr string) *Color {
	if hexStr[0] == '#' {
		hexStr = hexStr[1:]
	}
	if len(hexStr) != 6 {
		return black()
	}
	// hex string -> sRGB -> lRGB
	r := rgbStandardToLinear(float64(hexByte(hexStr[0])<<4+hexByte(hexStr[1])) / 0xFF)
	g := rgbStandardToLinear(float64(hexByte(hexStr[2])<<4+hexByte(hexStr[3])) / 0xFF)
	b := rgbStandardToLinear(float64(hexByte(hexStr[4])<<4+hexByte(hexStr[5])) / 0xFF)

	// lRGB -> CIE XYZ
	x := 0.4124564*r + 0.3575761*g + 0.1804375*b
	y := 0.2126729*r + 0.7151522*g + 0.0721750*b
	z := 0.0193339*r + 0.1191920*g + 0.9503041*b

	// CIE XYZ -> L'M'S' (with "perceptual quantizer" gamma)
	// https://observablehq.com/@jrus/jzazbz
	lP := pq(0.674207838*x + 0.382799340*y - 0.047570458*z)
	mP := pq(0.149284160*x + 0.739628340*y + 0.083327300*z)
	sP := pq(0.070941080*x + 0.174768000*y + 0.670970020*z)
	iZ := 0.5 * (lP + mP)

	// L'M'S' -> JzAzBz
	return &Color{
		j: (0.44*iZ)/(1-0.56*iZ) - 1.6295499532821566e-11,
		a: 3.524000*lP - 4.066708*mP + 0.542708*sP,
		b: 0.199076*lP + 1.096799*mP - 1.295875*sP,
	}
}

func (c *Color) blend(c2 *Color, frac float64) *Color {
	return &Color{
		j: lerp(c.j, c2.j, frac),
		a: lerp(c.a, c2.a, frac),
		b: lerp(c.b, c2.b, frac),
	}
}

func hexByte(b byte) byte {
	switch {
	case b >= '0' && b <= '9':
		return b - '0'
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10
	}
	return 0
}

func rgbLinearToStandard(l float64) float64 {
	if l <= 0.0031308 {
		return 12.92 * l
	}
	return 1.055*math.Pow(l, 0.4166666666666667) - 0.055
}

func rgbStandardToLinear(s float64) float64 {
	if s <= 0.04045 {
		// return s / 12.92
		return 0.07739938080495357 * s
	}
	// return math.Pow((s+0.55)/1.055), 2.4)
	return math.Pow((s+0.055)*0.9478672985781991, 2.4)
}

func pq(x float64) float64 {
	xP := math.Pow(x*1e-4, 0.1593017578125)
	return math.Pow((0.8359375+18.8515625*xP)/(1+18.6875*xP),
		134.034375)
}

func pqInv(x float64) float64 {
	xP := math.Pow(x, 7.460772656268214e-03)
	v := 1e4 * math.Pow(
		(0.8359375-xP)/(18.6875*xP-18.8515625),
		6.277394636015326,
	)
	if math.IsNaN(v) {
		return 0
	}
	return v
}

func lerp(a, b, f float64) float64 {
	if a == b {
		return a
	}
	return a*(1.0-f) + (b * f)
}
