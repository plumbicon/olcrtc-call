package qr

import (
	"fmt"

	"github.com/makiuchi-d/gozxing"
	gzqr "github.com/makiuchi-d/gozxing/qrcode"
	rsciiqr "rsc.io/qr"
)

// ECCLevel controls QR error correction level (capacity vs resilience trade-off).
type ECCLevel int

const (
	ECCLow      ECCLevel = iota // L - max capacity (~2953 bytes)
	ECCMedium                   // M - ~25% recovery
	ECCQuartile                 // Q - ~50% recovery
	ECCHigh                     // H - ~65% recovery, min capacity (~1273 bytes)
)

// Config configures a QR Codec.
type Config struct {
	// FrameW, FrameH - output frame dimensions in pixels. Default: 1080×1080.
	FrameW, FrameH int

	// Margin - pixel margin around the QR on each side. Default: 20.
	Margin int

	// ECC - error correction level. Default: ECCLow (max payload).
	ECC ECCLevel
}

// DefaultConfig is the fastest, highest-capacity config for screen-to-screen transfer.
var DefaultConfig = Config{
	FrameW: 1080,
	FrameH: 1080,
	Margin: 20,
	ECC:    ECCLow,
}

// Codec encodes and decodes QR frames with a fixed config.
type Codec struct {
	cfg Config
}

// New creates a Codec. Zero-value fields in cfg fall back to DefaultConfig values.
func New(cfg Config) (*Codec, error) {
	if cfg.FrameW <= 0 {
		cfg.FrameW = DefaultConfig.FrameW
	}
	if cfg.FrameH <= 0 {
		cfg.FrameH = DefaultConfig.FrameH
	}
	if cfg.Margin < 0 {
		cfg.Margin = DefaultConfig.Margin
	}
	return &Codec{cfg: cfg}, nil
}

// MaxPayload returns the approximate maximum payload in bytes for this config.
func (c *Codec) MaxPayload() int {
	switch c.cfg.ECC {
	case ECCHigh:
		return 1273
	case ECCQuartile:
		return 1663
	case ECCMedium:
		return 2331
	default:
		return 2953
	}
}

// Info returns a human-readable summary of the codec config.
func (c *Codec) Info() string {
	eccName := [...]string{"Low", "Medium", "Quartile", "High"}[c.cfg.ECC]
	return fmt.Sprintf("qr: frame=%dx%d  margin=%d  ECC=%s  maxPayload=%d bytes",
		c.cfg.FrameW, c.cfg.FrameH, c.cfg.Margin, eccName, c.MaxPayload())
}

// Encode encodes payload into a grayscale frame of FrameW×FrameH pixels (1 byte/pixel).
// 0 = black, 255 = white. QR is centered with quiet zone.
func (c *Codec) Encode(payload []byte) ([]byte, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("qr: empty payload")
	}
	level := toRscLevel(c.cfg.ECC)
	code, err := rsciiqr.Encode(string(payload), level)
	if err != nil {
		return nil, fmt.Errorf("qr: encode: %w", err)
	}

	// rsc.io/qr: Black(x,y) valid for x,y in [-4, Size+4)
	// modules including quiet zone = Size + 8
	modules := code.Size + 8

	w, h := c.cfg.FrameW, c.cfg.FrameH
	margin := c.cfg.Margin

	scaleX := (w - margin*2) / modules
	scaleY := (h - margin*2) / modules
	scale := min(scaleX, scaleY)
	if scale < 1 {
		scale = 1
	}

	qrW := modules * scale
	qrH := modules * scale
	offsetX := (w - qrW) / 2
	offsetY := (h - qrH) / 2

	frame := make([]byte, w*h)
	for i := range frame {
		frame[i] = 255
	}

	for row := 0; row < modules; row++ {
		y0 := offsetY + row*scale
		for col := 0; col < modules; col++ {
			if !code.Black(col-4, row-4) {
				continue
			}
			x0 := offsetX + col*scale
			for dy := 0; dy < scale; dy++ {
				base := (y0+dy)*w + x0
				for dx := 0; dx < scale; dx++ {
					frame[base+dx] = 0
				}
			}
		}
	}
	return frame, nil
}

// EncodeBitmap returns the raw [][]bool module grid (with quiet zone) for custom rendering.
func (c *Codec) EncodeBitmap(payload []byte) ([][]bool, error) {
	code, err := rsciiqr.Encode(string(payload), toRscLevel(c.cfg.ECC))
	if err != nil {
		return nil, err
	}
	size := code.Size + 8
	bmp := make([][]bool, size)
	for row := range bmp {
		bmp[row] = make([]bool, size)
		for col := range bmp[row] {
			bmp[row][col] = code.Black(col-4, row-4)
		}
	}
	return bmp, nil
}

// Decode decodes a QR code from a FrameW×FrameH grayscale frame.
// Uses PURE_BARCODE fast path; falls back to GlobalHistogram on failure.
func (c *Codec) Decode(frame []byte) ([]byte, error) {
	w, h := c.cfg.FrameW, c.cfg.FrameH
	if len(frame) != w*h {
		return nil, fmt.Errorf("qr: decode: expected %d bytes, got %d", w*h, len(frame))
	}

	src := &grayLuminance{pix: frame, w: w, h: h}
	bmp, err := gozxing.NewBinaryBitmap(newFixedThresholdBinarizer(src, 128))
	if err != nil {
		return nil, fmt.Errorf("qr: decode: %w", err)
	}

	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_PURE_BARCODE: true,
	}
	reader := gzqr.NewQRCodeReader()
	result, err := reader.Decode(bmp, hints)
	if err != nil {
		bmp2, _ := gozxing.NewBinaryBitmap(gozxing.NewGlobalHistgramBinarizer(src))
		result, err = reader.Decode(bmp2, nil)
		if err != nil {
			return nil, fmt.Errorf("qr: decode: %w", err)
		}
	}
	return extractBytes(result), nil
}

// --- package-level convenience wrappers using DefaultConfig ---

var defaultCodec, _ = New(DefaultConfig)

// Encode encodes payload into a 1080×1080 frame using default config (ECC Low).
func Encode(payload []byte) ([]byte, error) { return defaultCodec.Encode(payload) }

// Decode decodes a 1080×1080 frame using default config.
func Decode(frame []byte) ([]byte, error) { return defaultCodec.Decode(frame) }

// EncodeBitmap returns a raw bitmap using default config.
func EncodeBitmap(payload []byte) ([][]bool, error) { return defaultCodec.EncodeBitmap(payload) }

// --- helpers ---

func toRscLevel(ecc ECCLevel) rsciiqr.Level {
	switch ecc {
	case ECCHigh:
		return rsciiqr.H
	case ECCQuartile:
		return rsciiqr.Q
	case ECCMedium:
		return rsciiqr.M
	default:
		return rsciiqr.L
	}
}

func extractBytes(result interface {
	GetResultMetadata() map[gozxing.ResultMetadataType]interface{}
	GetText() string
}) []byte {
	meta := result.GetResultMetadata()
	if segs, ok := meta[gozxing.ResultMetadataType_BYTE_SEGMENTS]; ok {
		if parts, ok := segs.([][]byte); ok && len(parts) > 0 {
			if len(parts) == 1 {
				return parts[0]
			}
			total := 0
			for _, p := range parts {
				total += len(p)
			}
			out := make([]byte, 0, total)
			for _, p := range parts {
				out = append(out, p...)
			}
			return out
		}
	}
	return []byte(result.GetText())
}
