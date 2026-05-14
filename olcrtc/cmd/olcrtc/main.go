// Package main provides the olcrtc CLI entrypoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	protoLogger "github.com/livekit/protocol/logger"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/openlibrecommunity/olcrtc/internal/app/session"
	"github.com/openlibrecommunity/olcrtc/internal/logger"
	"github.com/openlibrecommunity/olcrtc/internal/names"
	"github.com/openlibrecommunity/olcrtc/internal/transport/videochannel"
)

const modeGen = "gen"

// ErrDataDirRequired is returned when no data directory is specified.
var ErrDataDirRequired = errors.New("data directory required (use -data data)")

//nolint:gochecknoglobals // Tests replace the long-running session runner with a bounded function.
var runSession = session.Run

//nolint:gochecknoglobals // Tests replace gen runner with a stub.
var runGen = execGen

type config struct {
	mode            string
	link            string
	transport       string
	carrier         string
	roomID          string
	clientID        string
	socksPort       int
	socksHost       string
	socksUser       string
	socksPass       string
	keyHex          string
	debug           bool
	dataDir         string
	dnsServer       string
	socksProxyAddr  string
	socksProxyPort  int
	videoWidth      int
	videoHeight     int
	videoFPS        int
	videoBitrate    string
	videoHW         string
	videoQRSize     int
	videoQRRecovery string
	videoCodec      string
	videoTileModule int
	videoTileRS     int
	vp8FPS          int
	vp8BatchSize    int
	seiFPS          int
	seiBatchSize    int
	seiFragmentSize int
	seiAckTimeoutMS int
	amount          int
	ffmpegPath      string
}

func main() {
	if err := run(); err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}

func run() error {
	return runWithArgs(os.Args[1:])
}

func runWithArgs(args []string) error {
	session.RegisterDefaults()

	cfg, err := parseFlagsFrom(args, flag.ExitOnError)
	if err != nil {
		return err
	}
	return runWithConfig(cfg)
}

func runWithConfig(cfg config) error {
	configureLogging(cfg.debug)

	if cfg.ffmpegPath != "ffmpeg" && cfg.ffmpegPath != "" {
		videochannel.FFmpegPath = cfg.ffmpegPath
	}

	if cfg.mode == modeGen {
		return runGen(cfg)
	}

	if err := session.Validate(toSessionConfig(cfg)); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	if cfg.dataDir == "" {
		return ErrDataDirRequired
	}

	dataDir, err := resolveDataDir(cfg.dataDir)
	if err != nil {
		return err
	}

	if err := loadNames(dataDir); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- runSession(ctx, toSessionConfig(cfg))
	}()

	select {
	case <-sigCh:
		logger.Info("Shutting down gracefully...")
		cancel()
		return waitForShutdown(errCh)
	case err := <-errCh:
		return err
	}
}

func execGen(cfg config) error {
	scfg := toSessionConfig(cfg)
	if err := session.ValidateGen(scfg); err != nil {
		return fmt.Errorf("validate gen config: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		errCh <- session.Gen(ctx, scfg, func(id string) { _, _ = fmt.Fprintln(os.Stdout, id) })
	}()

	select {
	case <-sigCh:
		cancel()
		return waitForShutdown(errCh)
	case err := <-errCh:
		return err
	}
}

func parseFlagsFrom(args []string, errorHandling flag.ErrorHandling) (config, error) {
	cfg := config{}
	fs := flag.NewFlagSet("olcrtc", errorHandling)
	if errorHandling == flag.ContinueOnError {
		fs.SetOutput(io.Discard)
	}

	fs.StringVar(&cfg.mode, "mode", "", "Mode: srv or cnc")
	fs.StringVar(&cfg.link, "link", "", "Link: direct (p2p connection type)")
	fs.StringVar(&cfg.transport, "transport", "", "Transport: datachannel, videochannel, seichannel")
	fs.StringVar(&cfg.carrier, "carrier", "", "Carrier: telemost, jazz, wbstream")
	fs.StringVar(&cfg.roomID, "id", "", "Room ID")
	fs.StringVar(&cfg.clientID, "client-id", "", "Client ID: binds one srv to one cnc (required)")
	fs.IntVar(&cfg.socksPort, "socks-port", 0, "SOCKS5 port (client only)")
	fs.StringVar(&cfg.socksHost, "socks-host", "", "SOCKS5 listen host (client only)")
	fs.StringVar(&cfg.socksUser, "socks-user", "", "SOCKS5 username for incoming connections (client only, optional)")
	fs.StringVar(&cfg.socksPass, "socks-pass", "", "SOCKS5 password for incoming connections (client only, optional)")
	fs.StringVar(&cfg.keyHex, "key", "", "Shared encryption key (hex)")
	fs.BoolVar(&cfg.debug, "debug", false, "Enable verbose logging")
	fs.StringVar(&cfg.dataDir, "data", "", "Path to data directory")
	fs.StringVar(&cfg.dnsServer, "dns", "", "DNS server (e.g. 1.1.1.1:53)")
	fs.StringVar(&cfg.socksProxyAddr, "socks-proxy", "", "SOCKS5 proxy address (server only)")
	fs.IntVar(&cfg.socksProxyPort, "socks-proxy-port", 0, "SOCKS5 proxy port (server only)")
	fs.IntVar(&cfg.videoWidth, "video-w", 0, "Video logical width (videochannel only)")
	fs.IntVar(&cfg.videoHeight, "video-h", 0, "Video logical height (videochannel only)")
	fs.IntVar(&cfg.videoFPS, "video-fps", 0, "Video frames per second (videochannel only)")
	fs.StringVar(&cfg.videoBitrate, "video-bitrate", "", "Video bitrate (videochannel only)")
	fs.StringVar(&cfg.videoHW, "video-hw", "", "Hardware acceleration (none, nvenc)")
	fs.IntVar(&cfg.videoQRSize, "video-qr-size", 0, "Video QR code fragment size (videochannel only)")
	fs.StringVar(&cfg.videoQRRecovery, "video-qr-recovery", "low",
		"QR error correction: low (7%), medium (15%), high (25%), highest (30%)")
	fs.StringVar(&cfg.videoCodec, "video-codec", "qrcode", "Visual codec: qrcode or tile")
	fs.IntVar(&cfg.videoTileModule, "video-tile-module", 0,
		"Tile module size in pixels 1..270 (videochannel tile only, default 4)")
	fs.IntVar(&cfg.videoTileRS, "video-tile-rs", 0,
		"Tile Reed-Solomon parity percent 0..200 (videochannel tile only, default 20)")
	fs.IntVar(&cfg.vp8FPS, "vp8-fps", 0, "VP8 frames per second (vp8channel only, default 25)")
	fs.IntVar(&cfg.vp8BatchSize, "vp8-batch", 0, "VP8 frames per tick (vp8channel only, default 1)")
	fs.IntVar(&cfg.seiFPS, "fps", 0, "Frames per second for transports that use video timing (seichannel)")
	fs.IntVar(&cfg.seiBatchSize, "batch", 0, "Transport frames per tick for batched transports (seichannel)")
	fs.IntVar(&cfg.seiFragmentSize, "frag", 0, "Fragment size in bytes for fragmented transports (seichannel)")
	fs.IntVar(&cfg.seiAckTimeoutMS, "ack-ms", 0, "ACK timeout in milliseconds for reliable visual transports (seichannel)")
	fs.IntVar(&cfg.amount, "amount", 0, "Number of rooms to generate (gen mode only)")
	fs.StringVar(&cfg.ffmpegPath, "ffmpeg", "ffmpeg", "Path to ffmpeg executable")

	if err := fs.Parse(args); err != nil {
		return cfg, fmt.Errorf("parse flags: %w", err)
	}

	return cfg, nil
}

func configureLogging(debug bool) {
	if debug {
		logger.SetVerbose(true)
		return
	}
	// Suppress noisy LiveKit/pion logs unless debug is enabled.
	_ = os.Setenv("PION_LOG_DISABLE", "all")
	lksdk.SetLogger(protoLogger.GetDiscardLogger())
}

func resolveDataDir(dataDir string) (string, error) {
	if filepath.IsAbs(dataDir) {
		return dataDir, nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), dataDir), nil
}

func loadNames(dataDir string) error {
	namesPath := filepath.Join(dataDir, "names")
	surnamesPath := filepath.Join(dataDir, "surnames")
	if err := names.LoadNameFiles(namesPath, surnamesPath); err != nil {
		return fmt.Errorf("load embedded names override: %w", err)
	}

	return nil
}

func toSessionConfig(cfg config) session.Config {
	return session.Config{
		Mode:            cfg.mode,
		Link:            cfg.link,
		Transport:       cfg.transport,
		Carrier:         cfg.carrier,
		RoomID:          cfg.roomID,
		ClientID:        cfg.clientID,
		KeyHex:          cfg.keyHex,
		SOCKSHost:       cfg.socksHost,
		SOCKSPort:       cfg.socksPort,
		SOCKSUser:       cfg.socksUser,
		SOCKSPass:       cfg.socksPass,
		DNSServer:       cfg.dnsServer,
		SOCKSProxyAddr:  cfg.socksProxyAddr,
		SOCKSProxyPort:  cfg.socksProxyPort,
		VideoWidth:      cfg.videoWidth,
		VideoHeight:     cfg.videoHeight,
		VideoFPS:        cfg.videoFPS,
		VideoBitrate:    cfg.videoBitrate,
		VideoHW:         cfg.videoHW,
		VideoQRSize:     cfg.videoQRSize,
		VideoQRRecovery: cfg.videoQRRecovery,
		VideoCodec:      cfg.videoCodec,
		VideoTileModule: cfg.videoTileModule,
		VideoTileRS:     cfg.videoTileRS,
		VP8FPS:          cfg.vp8FPS,
		VP8BatchSize:    cfg.vp8BatchSize,
		SEIFPS:          cfg.seiFPS,
		SEIBatchSize:    cfg.seiBatchSize,
		SEIFragmentSize: cfg.seiFragmentSize,
		SEIAckTimeoutMS: cfg.seiAckTimeoutMS,
		Amount:          cfg.amount,
	}
}

func waitForShutdown(errCh <-chan error) error {
	done := make(chan error, 1)
	go func() {
		if err := <-errCh; err != nil {
			done <- err
		} else {
			done <- nil
		}
	}()

	select {
	case err := <-done:
		if err == nil {
			logger.Info("Shutdown complete")
		}
		return err
	case <-time.After(5 * time.Second):
		logger.Warn("Shutdown timeout, forcing exit")
		return nil
	}
}
