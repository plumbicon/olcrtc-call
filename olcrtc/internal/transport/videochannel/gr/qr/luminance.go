package qr

import (
	"fmt"

	"github.com/makiuchi-d/gozxing"
)

type grayLuminance struct {
	pix  []byte
	w, h int
}

func (g *grayLuminance) GetWidth() int     { return g.w }
func (g *grayLuminance) GetHeight() int    { return g.h }
func (g *grayLuminance) GetMatrix() []byte { return g.pix }

func (g *grayLuminance) GetRow(y int, row []byte) ([]byte, error) {
	if y < 0 || y >= g.h {
		return nil, fmt.Errorf("qr: row %d out of bounds", y)
	}
	start := y * g.w
	if len(row) < g.w {
		row = make([]byte, g.w)
	}
	copy(row, g.pix[start:start+g.w])
	return row[:g.w], nil
}

func (g *grayLuminance) IsCropSupported() bool { return false }
func (g *grayLuminance) Crop(_, _, _, _ int) (gozxing.LuminanceSource, error) {
	return nil, fmt.Errorf("qr: crop not supported")
}
func (g *grayLuminance) IsRotateSupported() bool { return false }
func (g *grayLuminance) RotateCounterClockwise() (gozxing.LuminanceSource, error) {
	return nil, fmt.Errorf("qr: rotate not supported")
}
func (g *grayLuminance) RotateCounterClockwise45() (gozxing.LuminanceSource, error) {
	return nil, fmt.Errorf("qr: rotate not supported")
}
func (g *grayLuminance) Invert() gozxing.LuminanceSource {
	return gozxing.NewInvertedLuminanceSource(g)
}
func (g *grayLuminance) String() string { return fmt.Sprintf("GrayLuminance(%dx%d)", g.w, g.h) }
