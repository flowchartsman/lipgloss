/*
Go implementation of color math in the JzAzBz color space as described in "[Perceptually uniform color space for image signals including high dynamic range and wide gamut]"

JzAzBz was designed to provide a color space capable of predicting both small and large color differences and accurate lightness values in high dynamic range environments with minimum computational cost. It is perceptually uniform over a wide gamut, and linear in iso-hue directions, allowing for more natural color gradients. Colors in this space are comprised of three components:

	+-----------+-------------+---------+
	| Component | Description |  Range  |
	+-----------+-------------+---------+
	| j         | lightness   | [ 0, 1] |
	| a         | green-red   | [-1, 1] |
	| b         | blue-yellow | [-1, 1] |
	+-----------+-------------+---------+

JzAzBz is always calculated relative to standard illuminant D65.

Implementation based in part on Kotlin implementation at https://github.com/ajalt/colormath

[Perceptually uniform color space for image signals including high dynamic range and wide gamut]: https://opg.optica.org/oe/fulltext.cfm?uri=oe-25-13-15131&id=368272
*/
package jzazbz

import (
	"fmt"
	"math"
	"sort"
)

var BadGradient = &Gradient{}

type Gradient struct {
	Stops []Stop
}

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
		Stops: make([]Stop, len(stops)),
	}
	for i := range stops {
		g.Stops[i] = Stop{
			Color:  FromHex(stops[i]),
			Offset: offsets[i],
		}
	}
	return g
}

func (g *Gradient) Color(idx int) *Color {
	if len(g.Stops) > idx {
		return g.Stops[idx].Color
	}
	return &Color{0, 0, 0}
}

func (g *Gradient) ColorAt(pos, max int) *Color {
	switch len(g.Stops) {
	case 0:
		return &Color{0, 0, 0}
	case 1:
		return g.Stops[0].Color
	}

	switch pos {
	case 0:
		return g.Stops[0].Color
	case max:
		return g.Stops[len(g.Stops)-1].Color
	}
	f := float64(pos) / float64(max)
	var s int
	for s = 0; s < len(g.Stops); s++ {
		if f < g.Stops[s].Offset {
			break
		}
	}
	switch s {
	case 0:
		return g.Stops[0].Color
	case len(g.Stops):
		return g.Stops[len(g.Stops)-1].Color
	}
	// normalize 0.0-1.0 between stops
	f = (f - g.Stops[s-1].Offset) / (g.Stops[s].Offset - g.Stops[s-1].Offset)
	return (g.Stops[s-1].Color.blend(g.Stops[s].Color, f))
}

// Stop is a gradient stop. Pos is (0,1)
type Stop struct {
	Color  *Color
	Offset float64
}

// Color is a color value in the JzAzBz color space.
type Color struct {
	j float64
	a float64
	b float64
}

// FromHex converts a hex color to JzAzBz
// invalid colors will result in black.
func FromHex(colStr string) *Color {
	format := "#%02x%02x%02x"
	factor := 1.0 / 255.0
	if len(colStr) == 4 {
		format = "#%1x%1x%1x"
		factor = 1.0 / 15.0
	}

	var hr, hg, hb uint8
	n, err := fmt.Sscanf(colStr, format, &hr, &hg, &hb)
	if err != nil {
		return &Color{0, 0, 0}
	}
	if n != 3 {
		return &Color{0, 0, 0}
	}
	// parsed sRGB -> lRGB
	r := linearize(float64(hr) * factor)
	g := linearize(float64(hg) * factor)
	b := linearize(float64(hb) * factor)

	// lRGB -> CIE XYZ
	x := 0.41239079926595948*r + 0.35758433938387796*g + 0.18048078840183429*b
	y := 0.21263900587151036*r + 0.71516867876775593*g + 0.072192315360733715*b
	z := 0.019330818715591851*r + 0.11919477979462599*g + 0.95053215224966058*b

	// CIE XYZ -> LMS (with "perceptual quantizer" gamma)
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

// Hex returns the RGB HEX string for c.
func (c *Color) Hex() string {
	r, g, b := c.rgb()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r*255.0+0.5), uint8(g*255.0+0.5), uint8(b*255.0+0.5))
}

// RGBA returns the RGBA value of this color.
func (c *Color) RGBA() (r, g, b, a uint32) {
	r1, g1, b1 := c.lrgb()
	return uint32(r1*65535.0 + 0.5),
		uint32(g1*65535.0 + 0.5),
		uint32(b1*65535.0 + 0.5),
		0xFFFF
}

func (c *Color) lms() (l, m, s float64) {
	// Combined matrix values from https://observablehq.com/@jrus/jzazbz
	const d0 = 1.6295499532821566e-11
	jz := c.j + d0
	iz := jz / (0.44 + 0.56*jz)
	return pqInv(iz + 1.386050432715393e-1*c.a + 5.804731615611869e-2*c.b),
		pqInv(iz - 1.386050432715393e-1*c.a - 5.804731615611891e-2*c.b),
		pqInv(iz - 9.601924202631895e-2*c.a - 8.118918960560390e-1*c.b)
}

func (c *Color) xyz() (x, y, z float64) {
	l, m, s := c.lms()
	return 1.661373055774069e+00*l - 9.145230923250668e-01*m + 2.313620767186147e-01*s,
		-3.250758740427037e-01*l + 1.571847038366936e+00*m - 2.182538318672940e-01*s,
		9.098281098284756e-02*l - 3.127282905230740e-01*m + 1.522766561305260e+00*s
}

func (c *Color) rgb() (r, g, b float64) {
	lR, lG, lB := c.lrgb()
	return delinearize(lR),
		delinearize(lG),
		delinearize(lB)
}

func (c *Color) lrgb() (r, g, b float64) {
	x, y, z := c.xyz()
	return 3.2409699419045214*x - 1.5373831775700935*y - 0.49861076029300328*z,
		-0.96924363628087983*x + 1.8759675015077207*y + 0.041555057407175613*z,
		0.055630079696993609*x - 0.20397695888897657*y + 1.0569715142428786*z
}

func (c *Color) blend(c2 *Color, frac float64) *Color {
	return &Color{
		j: lerp(c.j, c2.j, frac),
		a: lerp(c.a, c2.a, frac),
		b: lerp(c.b, c2.b, frac),
	}
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

// linearization (via https://github.com/lucasb-eyer/go-colorful)
// TODO: fast [de]linearization with clamp?
func linearize(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func delinearize(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1.0/2.4) - 0.055
}

func lerp(a, b, f float64) float64 {
	if a == b {
		return a
	}
	return a*(1.0-f) + (b * f)
}
