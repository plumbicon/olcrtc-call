package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/openlibrecommunity/olcrtc/internal/app/session"
	"github.com/openlibrecommunity/olcrtc/internal/logger"
)

var errBoom = errors.New("boom")

//nolint:cyclop // table-driven test naturally has many branches
func TestToSessionConfig(t *testing.T) {
	cfg := config{
		mode:            "cnc",
		link:            "direct", //nolint:goconst // test literal, repetition is intentional
		transport:       "vp8channel",
		carrier:         "jazz", //nolint:goconst // test literal, repetition is intentional
		roomID:          "room", //nolint:goconst // test literal, repetition is intentional
		clientID:        "client", //nolint:goconst // test literal, repetition is intentional
		keyHex:          "key", //nolint:goconst // test literal, repetition is intentional
		socksHost:       "127.0.0.1",
		socksPort:       1080,
		dnsServer:       "1.1.1.1:53", //nolint:goconst // test literal, repetition is intentional
		socksProxyAddr:  "proxy",
		socksProxyPort:  1081,
		videoWidth:      640,
		videoHeight:     480,
		videoFPS:        30,
		videoBitrate:    "1M",
		videoHW:         "none",
		videoQRSize:     4,
		videoQRRecovery: "low",
		videoCodec:      "qrcode",
		videoTileModule: 4,
		videoTileRS:     20,
		vp8FPS:          25,
		vp8BatchSize:    8,
		seiFPS:          40,
		seiBatchSize:    3,
		seiFragmentSize: 512,
		seiAckTimeoutMS: 1500,
		amount:          5,
	}

	got := toSessionConfig(cfg)
	if got.Mode != cfg.mode || got.Carrier != "jazz" || got.SOCKSPort != cfg.socksPort ||
		got.VideoTileRS != cfg.videoTileRS || got.VP8BatchSize != cfg.vp8BatchSize ||
		got.SEIFPS != cfg.seiFPS || got.SEIBatchSize != cfg.seiBatchSize ||
		got.SEIFragmentSize != cfg.seiFragmentSize || got.SEIAckTimeoutMS != cfg.seiAckTimeoutMS ||
		got.Amount != cfg.amount {
		t.Fatalf("toSessionConfig() = %+v", got)
	}
}

//nolint:cyclop // table-driven test naturally has many branches
func TestParseFlagsFrom(t *testing.T) {
	cfg, err := parseFlagsFrom([]string{
		"-mode", "srv", //nolint:goconst // test literal, repetition is intentional
		"-link", "direct",
		"-transport", "vp8channel",
		"-carrier", "telemost",
		"-id", "room",
		"-client-id", "client",
		"-socks-port", "1080",
		"-socks-host", "127.0.0.1",
		"-key", "key",
		"-debug",
		"-data", "data",
		"-dns", "9.9.9.9:53",
		"-socks-proxy", "proxy",
		"-socks-proxy-port", "1081",
		"-video-w", "640",
		"-video-h", "480",
		"-video-fps", "30",
		"-video-bitrate", "1M",
		"-video-hw", "none",
		"-video-qr-size", "128",
		"-video-qr-recovery", "high",
		"-video-codec", "tile",
		"-video-tile-module", "6",
		"-video-tile-rs", "40",
		"-vp8-fps", "24",
		"-vp8-batch", "3",
		"-fps", "40",
		"-batch", "4",
		"-frag", "512",
		"-ack-ms", "1500",
		"-amount", "7",
	}, flag.ContinueOnError)
	if err != nil {
		t.Fatalf("parseFlagsFrom() error = %v", err)
	}
	if cfg.mode != "srv" || cfg.carrier != "telemost" || cfg.roomID != "room" ||
		cfg.debug != true || cfg.videoCodec != "tile" || cfg.videoTileRS != 40 ||
		cfg.vp8FPS != 24 || cfg.vp8BatchSize != 3 || cfg.seiFPS != 40 ||
		cfg.seiBatchSize != 4 || cfg.seiFragmentSize != 512 || cfg.seiAckTimeoutMS != 1500 ||
		cfg.amount != 7 {
		t.Fatalf("parseFlagsFrom() = %+v", cfg)
	}

	_, err = parseFlagsFrom([]string{"-bad"}, flag.ContinueOnError)
	if err == nil {
		t.Fatal("parseFlagsFrom(bad flag) error = nil")
	}
}

func TestRunGenModeValidationErrors(t *testing.T) {
	session.RegisterDefaults()

	if err := runWithConfig(config{mode: "gen"}); err == nil { //nolint:goconst // test literal, repetition is intentional
		t.Fatal("runWithConfig(gen, no carrier) error = nil")
	}

	if err := runWithConfig(config{mode: "gen", carrier: "wbstream", dnsServer: "1.1.1.1:53"}); err == nil { //nolint:goconst,lll // test literal, repetition is intentional
		t.Fatal("runWithConfig(gen, amount=0) error = nil")
	}
}

func TestRunGenModeCallsGen(t *testing.T) {
	session.RegisterDefaults()

	var collected []string
	oldRunGen := runGen
	t.Cleanup(func() { runGen = oldRunGen })
	runGen = func(cfg config) error {
		if cfg.carrier != "wbstream" || cfg.dnsServer != "1.1.1.1:53" || cfg.amount != 3 {
			t.Fatalf("runGen cfg = %+v", cfg)
		}
		collected = append(collected, "ok")
		return nil
	}

	err := runWithConfig(config{mode: "gen", carrier: "wbstream", dnsServer: "1.1.1.1:53", amount: 3})
	if err != nil {
		t.Fatalf("runWithConfig(gen) error = %v", err)
	}
	if len(collected) != 1 {
		t.Fatalf("runGen called %d times, want 1", len(collected))
	}
}

func TestRunWithConfigValidationAndDataDirErrors(t *testing.T) {
	session.RegisterDefaults()
	cfg := config{
		mode:       "srv",
		link:       "direct",
		transport:  "datachannel",
		carrier:    "jazz",
		clientID:   "client",
		keyHex:     "key",
		dnsServer:  "1.1.1.1:53",
		videoCodec: "qrcode",
	}
	if err := runWithConfig(cfg); !errors.Is(err, ErrDataDirRequired) {
		t.Fatalf("runWithConfig(no data dir) = %v, want %v", err, ErrDataDirRequired)
	}

	cfg.mode = ""
	if err := runWithConfig(cfg); err == nil {
		t.Fatal("runWithConfig(invalid config) error = nil")
	}
}

func TestRunWithArgsSuccessfulSessionReturn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "names"), []byte("A\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(names) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "surnames"), []byte("B\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(surnames) error = %v", err)
	}

	oldRunSession := runSession
	t.Cleanup(func() {
		runSession = oldRunSession
	})
	called := false
	runSession = func(ctx context.Context, cfg session.Config) error {
		called = true
		if cfg.Mode != "srv" || cfg.Carrier != "jazz" || cfg.ClientID != "client" {
			t.Fatalf("session config = %+v", cfg)
		}
		select {
		case <-ctx.Done():
			t.Fatal("context canceled before session returned")
		default:
		}
		return nil
	}

	err := runWithArgs([]string{
		"-mode", "srv",
		"-link", "direct",
		"-transport", "datachannel",
		"-carrier", "jazz",
		"-client-id", "client",
		"-key", "key",
		"-dns", "1.1.1.1:53",
		"-data", dir,
	})
	if err != nil {
		t.Fatalf("runWithArgs() error = %v", err)
	}
	if !called {
		t.Fatal("runWithArgs() did not call session runner")
	}
}

func TestConfigureLogging(t *testing.T) {
	t.Setenv("PION_LOG_DISABLE", "")
	logger.SetVerbose(false)
	configureLogging(true)
	if !logger.IsVerbose() {
		t.Fatal("configureLogging(true) did not enable verbose logging")
	}
	if got := os.Getenv("PION_LOG_DISABLE"); got != "" {
		t.Fatalf("configureLogging(true) PION_LOG_DISABLE = %q, want empty", got)
	}

	logger.SetVerbose(false)
	configureLogging(false)
	if logger.IsVerbose() {
		t.Fatal("configureLogging(false) enabled verbose logging")
	}
	if got := os.Getenv("PION_LOG_DISABLE"); got != "all" {
		t.Fatalf("configureLogging(false) PION_LOG_DISABLE = %q, want all", got)
	}
}

func TestResolveDataDir(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "data")
	got, err := resolveDataDir(abs)
	if err != nil {
		t.Fatalf("resolveDataDir(abs) error = %v", err)
	}
	if got != abs {
		t.Fatalf("resolveDataDir(abs) = %q, want %q", got, abs)
	}

	got, err = resolveDataDir("data")
	if err != nil {
		t.Fatalf("resolveDataDir(rel) error = %v", err)
	}
	if filepath.Base(got) != "data" || !filepath.IsAbs(got) {
		t.Fatalf("resolveDataDir(rel) = %q, want absolute path ending in data", got)
	}
}

func TestLoadNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "names"), []byte("A\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(names) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "surnames"), []byte("B\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(surnames) error = %v", err)
	}
	if err := loadNames(dir); err != nil {
		t.Fatalf("loadNames() error = %v", err)
	}
}

func TestWaitForShutdown(t *testing.T) {
	errCh := make(chan error, 1)
	errCh <- nil
	if err := waitForShutdown(errCh); err != nil {
		t.Fatalf("waitForShutdown(nil) error = %v", err)
	}

	want := errBoom
	errCh = make(chan error, 1)
	errCh <- want
	if err := waitForShutdown(errCh); !errors.Is(err, want) {
		t.Fatalf("waitForShutdown(error) = %v, want %v", err, want)
	}
}
