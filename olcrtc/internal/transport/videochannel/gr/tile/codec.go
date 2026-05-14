package tile

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/klauspost/reedsolomon"
)

const (
	FrameW     = 1080
	FrameH     = 1080
	Black      = byte(64)
	White      = byte(192)
	headerSize = 20
	magic      = uint32(0x544C4532)
)

// Config configures Codec instance.
type Config struct {
	// Module is the pixel side of each bit-cell (1..270).
	// 1 = max density (145KB/frame), 4 = VP8-safe default.
	Module int

	// RSPercent is Reed-Solomon parity overhead in percent of data shards (0..200).
	// 0 = no ECC, 20 = 20% parity overhead (recovers up to 10% shard loss), 100 = 2× redundancy.
	RSPercent int
}

// DefaultConfig is the VP8/WebRTC-tuned default: 4×4 modules, 20% RS parity.
var DefaultConfig = Config{Module: 4, RSPercent: 20}

// Codec encodes/decodes tile frames.
type Codec struct {
	cfg        Config
	cols, rows int
	syncRows   int // rows reserved top+bottom for sync band
	dataRows   int
	frameBytes int // total bytes in data area
	dataShards int // RS data shards
	parShards  int // RS parity shards
	shardSz    int // bytes per shard, sized so wire (data+parity) fits in frame
	maxPayload int // bytes of user data per frame after header+ECC overhead
	enc        reedsolomon.Encoder
}

// New creates a Codec from config.
func New(cfg Config) (*Codec, error) {
	if cfg.Module < 1 || cfg.Module > 270 {
		return nil, fmt.Errorf("tile: module must be 1..270, got %d", cfg.Module)
	}
	if cfg.RSPercent < 0 || cfg.RSPercent > 200 {
		return nil, fmt.Errorf("tile: RSPercent must be 0..200, got %d", cfg.RSPercent)
	}

	cols := FrameW / cfg.Module
	rows := FrameH / cfg.Module

	syncRows := 0
	if rows > 4 {
		syncRows = 2
	}
	dataRows := rows - syncRows*2
	frameBytes := (cols * dataRows) / 8

	if frameBytes <= headerSize {
		return nil, fmt.Errorf("tile: module %d too large - frame holds only %d bytes", cfg.Module, frameBytes)
	}

	dataShards, parShards := rsShards(frameBytes-headerSize, cfg.RSPercent)

	totalShards := dataShards + parShards
	shardSz := (frameBytes - headerSize) / totalShards
	if shardSz < 1 {
		return nil, fmt.Errorf("tile: module too large for RS config")
	}

	maxPayload := dataShards*shardSz - headerSize
	if maxPayload <= 0 {
		return nil, fmt.Errorf("tile: module too large for RS config")
	}

	var enc reedsolomon.Encoder
	if parShards > 0 {
		var err error
		enc, err = reedsolomon.New(dataShards, parShards)
		if err != nil {
			return nil, fmt.Errorf("tile: reedsolomon.New(%d,%d): %w", dataShards, parShards, err)
		}
	}

	return &Codec{
		cfg:        cfg,
		cols:       cols,
		rows:       rows,
		syncRows:   syncRows,
		dataRows:   dataRows,
		frameBytes: frameBytes,
		dataShards: dataShards,
		parShards:  parShards,
		shardSz:    shardSz,
		maxPayload: maxPayload,
		enc:        enc,
	}, nil
}

// MaxPayload returns the max user bytes per frame.
func (c *Codec) MaxPayload() int { return c.maxPayload }

// Info prints codec parameters.
func (c *Codec) Info() string {
	fps60 := float64(c.maxPayload) * 60 / 1024 / 1024
	return fmt.Sprintf("tile: module=%dpx  grid=%dx%d  payload=%d B/frame  %.2f MB/s @60fps  RS=%d+%d",
		c.cfg.Module, c.cols, c.rows, c.maxPayload, fps60, c.dataShards, c.parShards)
}

// DecodeResult holds decoded frame data.
type DecodeResult struct {
	FrameID     uint32
	TotalFrames uint32
	Payload     []byte
}

// Encode packs payload into a 1080×1080 grayscale frame.
func (c *Codec) Encode(payload []byte, frameID, totalFrames uint32) ([]byte, error) {
	if len(payload) > c.maxPayload {
		return nil, fmt.Errorf("tile: payload %d > maxPayload %d", len(payload), c.maxPayload)
	}

	totalData := c.dataShards * c.shardSz

	raw := make([]byte, totalData+headerSize)
	binary.BigEndian.PutUint32(raw[0:], magic)
	binary.BigEndian.PutUint32(raw[4:], frameID)
	binary.BigEndian.PutUint32(raw[8:], totalFrames)
	binary.BigEndian.PutUint32(raw[12:], uint32(len(payload)))
	copy(raw[headerSize:], payload)
	crcBuf := make([]byte, 16+len(payload))
	copy(crcBuf, raw[:16])
	copy(crcBuf[16:], payload)
	binary.BigEndian.PutUint32(raw[16:], crc32.ChecksumIEEE(crcBuf))

	var wire []byte
	if c.parShards > 0 {
		shards := splitShards(raw, c.dataShards, c.shardSz)
		parity := make([][]byte, c.parShards)
		for i := range parity {
			parity[i] = make([]byte, c.shardSz)
		}
		all := append(shards, parity...)
		if err := c.enc.Encode(all); err != nil {
			return nil, fmt.Errorf("tile: rs encode: %w", err)
		}
		wire = joinShards(all)
	} else {
		wire = raw
	}

	return c.renderFrame(wire), nil
}

// Decode extracts payload from a 1080×1080 grayscale frame.
func (c *Codec) Decode(frame []byte) (*DecodeResult, error) {
	if len(frame) != FrameW*FrameH {
		return nil, fmt.Errorf("tile: expected %d bytes frame, got %d", FrameW*FrameH, len(frame))
	}

	wire := c.readFrame(frame)
	shardSz := shardBytes(c.frameBytes-headerSize, c.dataShards)

	var raw []byte
	if c.parShards > 0 {
		totalShards := c.dataShards + c.parShards
		all := splitShards(wire, totalShards, shardSz)
		ok, err := c.enc.Verify(all)
		if err != nil || !ok {
			if err := c.enc.Reconstruct(all); err != nil {
				return nil, fmt.Errorf("tile: rs reconstruct: %w", err)
			}
		}
		raw = joinShards(all[:c.dataShards])
	} else {
		raw = wire
	}

	if binary.BigEndian.Uint32(raw[0:]) != magic {
		return nil, fmt.Errorf("tile: bad magic 0x%08X", binary.BigEndian.Uint32(raw[0:]))
	}
	frameID := binary.BigEndian.Uint32(raw[4:])
	totalFrames := binary.BigEndian.Uint32(raw[8:])
	payloadLen := binary.BigEndian.Uint32(raw[12:])
	if int(payloadLen) > c.maxPayload {
		return nil, fmt.Errorf("tile: bad payloadLen %d", payloadLen)
	}
	gotCRC := binary.BigEndian.Uint32(raw[16:])
	crcBuf := make([]byte, 16+payloadLen)
	copy(crcBuf, raw[:16])
	copy(crcBuf[16:], raw[headerSize:headerSize+payloadLen])
	if wantCRC := crc32.ChecksumIEEE(crcBuf); gotCRC != wantCRC {
		return nil, fmt.Errorf("tile: CRC mismatch got=%08X want=%08X", gotCRC, wantCRC)
	}

	out := make([]byte, payloadLen)
	copy(out, raw[headerSize:])
	return &DecodeResult{
		FrameID:     frameID,
		TotalFrames: totalFrames,
		Payload:     out,
	}, nil
}
