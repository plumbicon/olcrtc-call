package qr

import (
	"github.com/makiuchi-d/gozxing"
)

type fixedThresholdBinarizer struct {
	src       gozxing.LuminanceSource
	threshold int
}

func newFixedThresholdBinarizer(src gozxing.LuminanceSource, threshold int) gozxing.Binarizer {
	return &fixedThresholdBinarizer{src: src, threshold: threshold}
}

func (b *fixedThresholdBinarizer) GetLuminanceSource() gozxing.LuminanceSource { return b.src }
func (b *fixedThresholdBinarizer) GetWidth() int                               { return b.src.GetWidth() }
func (b *fixedThresholdBinarizer) GetHeight() int                              { return b.src.GetHeight() }
func (b *fixedThresholdBinarizer) CreateBinarizer(src gozxing.LuminanceSource) gozxing.Binarizer {
	return newFixedThresholdBinarizer(src, b.threshold)
}

func (b *fixedThresholdBinarizer) GetBlackRow(y int, row *gozxing.BitArray) (*gozxing.BitArray, error) {
	w := b.src.GetWidth()
	if row == nil || row.GetSize() < w {
		row = gozxing.NewBitArray(w)
	} else {
		row.Clear()
	}
	lum, err := b.src.GetRow(y, nil)
	if err != nil {
		return nil, err
	}
	for x := 0; x < w; x++ {
		if int(lum[x]&0xff) < b.threshold {
			row.Set(x)
		}
	}
	return row, nil
}

func (b *fixedThresholdBinarizer) GetBlackMatrix() (*gozxing.BitMatrix, error) {
	w, h := b.src.GetWidth(), b.src.GetHeight()
	matrix, err := gozxing.NewBitMatrix(w, h)
	if err != nil {
		return nil, err
	}
	lum := b.src.GetMatrix()
	for y := 0; y < h; y++ {
		base := y * w
		for x := 0; x < w; x++ {
			if int(lum[base+x]&0xff) < b.threshold {
				matrix.Set(x, y)
			}
		}
	}
	return matrix, nil
}
