// Package server implements the olcrtc tunnel server logic.
package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/openlibrecommunity/olcrtc/internal/crypto"
	"github.com/openlibrecommunity/olcrtc/internal/link"
	"github.com/openlibrecommunity/olcrtc/internal/logger"
	"github.com/openlibrecommunity/olcrtc/internal/muxconn"
	"github.com/openlibrecommunity/olcrtc/internal/names"
	"github.com/xtaci/smux"
)

const connectCommand = "connect"

var (
	// ErrKeyRequired is returned when no encryption key is provided.
	ErrKeyRequired = errors.New("key required (use -key <hex>)")
	// ErrKeySize is returned when the encryption key is not 32 bytes.
	ErrKeySize = errors.New("key must be 32 bytes")
	// ErrSocks5AuthFailed is returned when SOCKS5 authentication fails.
	ErrSocks5AuthFailed = errors.New("SOCKS5 auth failed")
	// ErrSocks5ConnectFailed is returned when SOCKS5 connection fails.
	ErrSocks5ConnectFailed = errors.New("SOCKS5 connect failed")
)

// Server handles incoming tunnel connections and proxies their traffic.
type Server struct {
	ln             link.Link
	cipher         *crypto.Cipher
	conn           *muxconn.Conn
	session        *smux.Session
	sessMu         sync.RWMutex
	reinstallMu    sync.Mutex
	wg             sync.WaitGroup
	clientID       string
	dnsServer      string
	resolver       *net.Resolver
	socksProxyAddr string
	socksProxyPort int
}

// ConnectRequest is a message from the client to establish a new connection.
type ConnectRequest struct {
	Cmd      string `json:"cmd"`
	ClientID string `json:"clientId"`
	Addr     string `json:"addr"`
	Port     int    `json:"port"`
}

// Run starts the server with the specified parameters.
func Run(
	ctx context.Context,
	linkName,
	transportName,
	carrierName,
	roomURL,
	keyHex,
	clientID string,
	dnsServer,
	socksProxyAddr string,
	socksProxyPort int,
	videoWidth int,
	videoHeight int,
	videoFPS int,
	videoBitrate string,
	videoHW string,
	videoQRSize int,
	videoQRRecovery string,
	videoCodec string,
	videoTileModule int,
	videoTileRS int,
	vp8FPS int,
	vp8BatchSize int,
	seiFPS int,
	seiBatchSize int,
	seiFragmentSize int,
	seiAckTimeoutMS int,
) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cipher, err := setupCipher(keyHex)
	if err != nil {
		return fmt.Errorf("setupCipher failed: %w", err)
	}

	s := &Server{
		cipher:         cipher,
		clientID:       clientID,
		dnsServer:      dnsServer,
		socksProxyAddr: socksProxyAddr,
		socksProxyPort: socksProxyPort,
	}
	s.setupResolver()

	if err := s.bringUpLink(
		runCtx, linkName, transportName, carrierName, roomURL, cancel,
		videoWidth, videoHeight, videoFPS, videoBitrate, videoHW,
		videoQRSize, videoQRRecovery, videoCodec, videoTileModule, videoTileRS,
		vp8FPS, vp8BatchSize,
		seiFPS, seiBatchSize, seiFragmentSize, seiAckTimeoutMS,
	); err != nil {
		return err
	}

	go func() {
		<-runCtx.Done()
		s.closeSession()
	}()

	s.serve(runCtx)

	s.shutdown()
	s.wg.Wait()

	return nil
}

func setupCipher(keyHex string) (*crypto.Cipher, error) {
	if keyHex == "" {
		return nil, ErrKeyRequired
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("%w, got %d", ErrKeySize, len(key))
	}

	cipher, err := crypto.NewCipher(string(key))
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	return cipher, nil
}

func (s *Server) setupResolver() {
	s.resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 3 * time.Second}
			return d.DialContext(ctx, network, s.dnsServer)
		},
	}
}

// smuxConfig mirrors the client side. Both peers must agree on Version and
// MaxFrameSize.
func smuxConfig() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2
	cfg.KeepAliveDisabled = true
	cfg.MaxFrameSize = 32768
	cfg.MaxReceiveBuffer = 16 * 1024 * 1024
	cfg.MaxStreamBuffer = 1024 * 1024
	cfg.KeepAliveInterval = 10 * time.Second
	cfg.KeepAliveTimeout = 60 * time.Second
	return cfg
}

func (s *Server) bringUpLink(
	ctx context.Context,
	linkName, transportName, carrierName, roomURL string,
	cancel context.CancelFunc,
	videoWidth, videoHeight, videoFPS int,
	videoBitrate, videoHW string,
	videoQRSize int,
	videoQRRecovery string,
	videoCodec string,
	videoTileModule, videoTileRS int,
	vp8FPS, vp8BatchSize int,
	seiFPS, seiBatchSize, seiFragmentSize, seiAckTimeoutMS int,
) error {
	ln, err := link.New(ctx, linkName, link.Config{
		Transport:       transportName,
		Carrier:         carrierName,
		RoomURL:         roomURL,
		ClientID:        s.clientID,
		Name:            names.Generate(),
		OnData:          s.onData,
		DNSServer:       s.dnsServer,
		ProxyAddr:       s.socksProxyAddr,
		ProxyPort:       s.socksProxyPort,
		VideoWidth:      videoWidth,
		VideoHeight:     videoHeight,
		VideoFPS:        videoFPS,
		VideoBitrate:    videoBitrate,
		VideoHW:         videoHW,
		VideoQRSize:     videoQRSize,
		VideoQRRecovery: videoQRRecovery,
		VideoCodec:      videoCodec,
		VideoTileModule: videoTileModule,
		VideoTileRS:     videoTileRS,
		VP8FPS:          vp8FPS,
		VP8BatchSize:    vp8BatchSize,
		SEIFPS:          seiFPS,
		SEIBatchSize:    seiBatchSize,
		SEIFragmentSize: seiFragmentSize,
		SEIAckTimeoutMS: seiAckTimeoutMS,
	})
	if err != nil {
		return fmt.Errorf("failed to create link: %w", err)
	}
	s.ln = ln

	ln.SetEndedCallback(func(reason string) {
		logger.Infof("Server link reported conference end: %s", reason)
		cancel()
	})
	ln.SetReconnectCallback(func() { s.handleReconnect() })

	logger.Infof("Connecting link via %s/%s/%s...", linkName, transportName, carrierName)
	if err := ln.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect link: %w", err)
	}
	logger.Infof("Link connected")

	s.installSession()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ln.WatchConnection(ctx)
	}()
	return nil
}

func (s *Server) installSession() {
	conn := muxconn.New(s.ln, s.cipher)
	sess, err := smux.Server(conn, smuxConfig())
	if err != nil {
		logger.Warnf("smux server init failed: %v", err)
		return
	}
	s.sessMu.Lock()
	s.conn = conn
	s.session = sess
	s.sessMu.Unlock()
}

func (s *Server) handleReconnect() {
	logger.Infof("server link reconnect - tearing down smux session")
	s.sessMu.RLock()
	current := s.session
	s.sessMu.RUnlock()
	s.reinstallSession(current)
}

func (s *Server) reinstallSession(dead *smux.Session) {
	s.reinstallMu.Lock()
	defer s.reinstallMu.Unlock()

	s.sessMu.Lock()
	if s.session != dead {
		s.sessMu.Unlock()
		return
	}
	if s.session != nil {
		_ = s.session.Close()
		s.session = nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.sessMu.Unlock()
	s.installSession()
}

func (s *Server) closeSession() {
	s.sessMu.Lock()
	if s.session != nil {
		_ = s.session.Close()
		s.session = nil
	}
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.sessMu.Unlock()
}

func (s *Server) onData(data []byte) {
	s.sessMu.RLock()
	conn := s.conn
	s.sessMu.RUnlock()
	if conn != nil {
		conn.Push(data)
	}
}

// serve drives the smux Accept loop, spawning a tunnel per inbound stream.
// The loop tolerates session bounces (reconnects) by waiting until a fresh
// session is installed instead of terminating the server.
func (s *Server) serve(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		s.sessMu.RLock()
		sess := s.session
		s.sessMu.RUnlock()
		if sess == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(50 * time.Millisecond):
				continue
			}
		}

		stream, err := sess.AcceptStream()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			logger.Debugf("AcceptStream returned %v - reinstalling session", err)
			s.reinstallSession(sess)
			continue
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleStream(ctx, stream)
		}()
	}
}

func (s *Server) shutdown() {
	s.closeSession()
	if s.ln != nil {
		_ = s.ln.Close()
	}
}

func (s *Server) handleStream(_ context.Context, stream *smux.Stream) {
	defer func() { _ = stream.Close() }()

	// Read the connect JSON. The client writes the whole JSON in one
	// stream.Write so it usually arrives intact; tolerate fragmentation
	// by reading incrementally up to a sane cap.
	const maxConnReq = 4096
	header := make([]byte, 0, 256)
	tmp := make([]byte, 256)
	_ = stream.SetReadDeadline(time.Now().Add(15 * time.Second))
	for {
		n, err := stream.Read(tmp)
		if n > 0 {
			header = append(header, tmp[:n]...)
			if req, ok := parseConnectRequest(header); ok {
				_ = stream.SetReadDeadline(time.Time{})
				if !s.authorizeRequest(req) {
					logger.Warnf("sid=%d rejected: client_id mismatch", stream.ID())
					return
				}
				s.dispatch(stream, req)
				return
			}
		}
		if err != nil {
			return
		}
		if len(header) > maxConnReq {
			return
		}
	}
}

func parseConnectRequest(buf []byte) (ConnectRequest, bool) {
	var req ConnectRequest
	if err := json.Unmarshal(buf, &req); err != nil {
		return req, false
	}
	if req.Cmd != connectCommand {
		return req, false
	}
	return req, true
}

func (s *Server) authorizeRequest(req ConnectRequest) bool {
	return req.ClientID == s.clientID
}

func (s *Server) dispatch(stream *smux.Stream, req ConnectRequest) {
	addr := net.JoinHostPort(req.Addr, strconv.Itoa(req.Port))
	logger.Infof("sid=%d connect %s", stream.ID(), addr)

	dialStart := time.Now()
	conn, err := s.dial(req)
	dialElapsed := time.Since(dialStart)

	if err != nil {
		logger.Infof("sid=%d dial %s failed (%v): %v", stream.ID(), addr, dialElapsed, err)
		return
	}
	defer func() { _ = conn.Close() }()

	logger.Infof("sid=%d connected %s in %v", stream.ID(), addr, dialElapsed)

	if _, err := stream.Write([]byte{0x00}); err != nil {
		return
	}

	go func() {
		_, _ = io.Copy(stream, conn)
		_ = stream.Close()
	}()
	_, _ = io.Copy(conn, stream)
}

func (s *Server) dial(req ConnectRequest) (net.Conn, error) {
	addr := net.JoinHostPort(req.Addr, strconv.Itoa(req.Port))
	if s.socksProxyAddr == "" {
		dialer := &net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  s.resolver,
		}
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("dial failed: %w", err)
		}
		return conn, nil
	}

	proxyAddr := net.JoinHostPort(s.socksProxyAddr, strconv.Itoa(s.socksProxyPort))
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.Dial("tcp", proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial proxy: %w", err)
	}

	if err := s.socks5Connect(conn, req.Addr, req.Port); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (s *Server) socks5Connect(conn net.Conn, targetAddr string, targetPort int) error {
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		return fmt.Errorf("failed to write socks5 auth: %w", err)
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("failed to read socks5 auth resp: %w", err)
	}
	if resp[0] != 5 || resp[1] != 0 {
		return ErrSocks5AuthFailed
	}

	addrType, addrBytes := socks5AddrBytes(targetAddr)
	req := make([]byte, 0, 6+len(addrBytes))
	req = append(req, 5, 1, 0, addrType)
	if addrType == 3 {
		req = append(req, byte(len(addrBytes)))
	}
	req = append(req, addrBytes...)
	req = append(req, byte(targetPort>>8), byte(targetPort)) //nolint:gosec // SOCKS5 port is uint16 by definition

	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("failed to write socks5 connect req: %w", err)
	}

	return readSocks5ConnectResponse(conn)
}

func socks5AddrBytes(targetAddr string) (byte, []byte) {
	if ip := net.ParseIP(targetAddr); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return 1, ipv4
		}
		return 4, ip.To16()
	}

	addrLen := len(targetAddr)
	if addrLen > 255 {
		addrLen = 255
		targetAddr = targetAddr[:255]
	}
	return 3, []byte(targetAddr)
}

func readSocks5ConnectResponse(conn net.Conn) error {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("failed to read socks5 connect resp: %w", err)
	}
	if header[0] != 5 {
		return fmt.Errorf("%w: invalid response version %d", ErrSocks5ConnectFailed, header[0])
	}

	var addrLen int
	switch header[3] {
	case 1:
		addrLen = net.IPv4len
	case 3:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return fmt.Errorf("failed to read socks5 response domain length: %w", err)
		}
		addrLen = int(lenBuf[0])
	case 4:
		addrLen = net.IPv6len
	default:
		return fmt.Errorf("unsupported socks5 response address type: %d", header[3])
	}

	if _, err := io.CopyN(io.Discard, conn, int64(addrLen+2)); err != nil {
		return fmt.Errorf("failed to read socks5 response bind address: %w", err)
	}
	if header[1] != 0 {
		return fmt.Errorf("%w: %d", ErrSocks5ConnectFailed, header[1])
	}
	return nil
}
