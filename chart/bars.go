// Package chart renders simple in-memory PNG charts for Telegram.
package chart

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed fonts/NotoSans-Regular.ttf
var notoSansTTF []byte

// Slice is one bar in the chart.
type Slice struct {
	Label string // full label shown on the chart (emoji + name ok)
	Value int64
	Pct   int64
}

var palette = []color.RGBA{
	{66, 133, 244, 255},
	{219, 68, 55, 255},
	{244, 180, 0, 255},
	{15, 157, 88, 255},
	{171, 71, 188, 255},
	{0, 172, 193, 255},
	{255, 112, 67, 255},
	{124, 179, 66, 255},
	{63, 81, 181, 255},
	{0, 150, 136, 255},
}

var (
	faceTitle    font.Face
	faceSubtitle font.Face
	faceLabel    font.Face
	facePct      font.Face
)

func init() {
	ft, err := opentype.Parse(notoSansTTF)
	if err != nil {
		panic("chart: parse font: " + err.Error())
	}
	faceTitle, err = opentype.NewFace(ft, &opentype.FaceOptions{Size: 22, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	faceSubtitle, err = opentype.NewFace(ft, &opentype.FaceOptions{Size: 15, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	faceLabel, err = opentype.NewFace(ft, &opentype.FaceOptions{Size: 16, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	facePct, err = opentype.NewFace(ft, &opentype.FaceOptions{Size: 15, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
}

// HorizontalBars draws a horizontal bar chart as PNG bytes.
// subtitle is optional (e.g. period "01.09.2026 – 07.09.2026") and is drawn under the title.
func HorizontalBars(title, subtitle string, slices []Slice) ([]byte, error) {
	if len(slices) == 0 {
		return nil, fmt.Errorf("no slices")
	}
	const (
		width     = 920
		leftPad   = 24
		rightPad  = 24
		rowH      = 56
		bottomPad = 28
		gapLabel  = 16
		pctColW   = 64
	)
	topPad := 56
	if strings.TrimSpace(subtitle) != "" {
		topPad = 78
	}

	// Label column width from longest truncated label.
	labelColW := 200
	for _, s := range slices {
		w := font.MeasureString(faceLabel, truncate(s.Label, 32)).Ceil()
		if w+8 > labelColW {
			labelColW = w + 8
		}
	}
	if labelColW > 360 {
		labelColW = 360
	}

	barMaxW := width - leftPad - rightPad - labelColW - gapLabel - pctColW
	if barMaxW < 120 {
		barMaxW = 120
	}
	height := topPad + len(slices)*rowH + bottomPad
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	bg := color.RGBA{248, 249, 251, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	drawText(img, faceTitle, leftPad, 32, title, color.RGBA{32, 32, 32, 255})
	if s := strings.TrimSpace(subtitle); s != "" {
		drawText(img, faceSubtitle, leftPad, 54, s, color.RGBA{100, 100, 110, 255})
	}

	var maxV int64
	for _, s := range slices {
		if s.Value > maxV {
			maxV = s.Value
		}
	}
	if maxV == 0 {
		maxV = 1
	}

	for i, s := range slices {
		y := topPad + i*rowH
		c := palette[i%len(palette)]
		label := truncate(s.Label, 32)
		drawText(img, faceLabel, leftPad, y+34, label, color.RGBA{40, 40, 40, 255})

		barW := int(math.Round(float64(s.Value) / float64(maxV) * float64(barMaxW)))
		if barW < 4 && s.Value > 0 {
			barW = 4
		}
		x0 := leftPad + labelColW + gapLabel
		barRect := image.Rect(x0, y+14, x0+barW, y+42)
		draw.Draw(img, barRect, &image.Uniform{C: c}, image.Point{}, draw.Src)
		draw.Draw(img, image.Rect(x0+barW-3, y+14, x0+barW, y+42), &image.Uniform{C: darker(c)}, image.Point{}, draw.Src)

		drawText(img, facePct, x0+barW+10, y+34, fmt.Sprintf("%d%%", s.Pct), color.RGBA{70, 70, 70, 255})
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func darker(c color.RGBA) color.RGBA {
	return color.RGBA{c.R * 8 / 10, c.G * 8 / 10, c.B * 8 / 10, 255}
}

func truncate(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes-1]) + "…"
}

func drawText(img *image.RGBA, face font.Face, x, y int, s string, col color.RGBA) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}
