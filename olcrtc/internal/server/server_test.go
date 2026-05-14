package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	cryptopkg "github.com/openlibrecommunity/olcrtc/internal/crypto"
	"github.com/openlibrecommunity/olcrtc/internal/muxconn"
	"github.com/xtaci/smux"
)

func TestSetupCipher(t *testing.T) {
	keyHex := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	cipher, err := setupCipher(keyHex)
	if err != nil {
		t.Fatalf("setupCipher() error = %v", err)
	}
	if cipher == nil {
		t.Fatal("setupCipher() returned nil cipher")
	}
}

func TestSetupCipherRejectsBadInput(t *testing.T) {
	if _, err := setupCipher(""); !errors.Is(err, ErrKeyRequired) {
		t.Fatalf("setupCipher() error = %v, want %v", err, ErrKeyRequired)
	}
	if _, err := setupCipher("zz"); err == nil {
		t.Fatal("setupCipher() unexpectedly succeeded for bad hex")
	}
	if _, err := setupCipher("00"); !errors.Is(err, ErrKeySize) {
		t.Fatalf("setupCipher() error = %v, want ErrKeySize", err)
	}
}

func TestSmuxConfig(t *testing.T) {
	cfg := smuxConfig()
	if cfg.Version != 2 || !cfg.KeepAliveDisabled || cfg.MaxFrameSize != 32768 || cfg.MaxReceiveBuffer != 16*1024*1024 {
		t.Fatalf("smuxConfig() = %+v", cfg)
	}
}

func TestParseConnectRequest(t *testing.T) {
	buf, err := json.Marshal(ConnectRequest{
		Cmd:      "connect",
		ClientID: "client-1",    //nolint:goconst // test literal, repetition is intentional
		Addr:     "example.com", //nolint:goconst // test literal, repetition is intentional
		Port:     443,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req, ok := parseConnectRequest(buf)
	if !ok {
		t.Fatal("parseConnectRequest() returned ok=false")
	}
	if req.ClientID != "client-1" || req.Addr != "example.com" || req.Port != 443 {
		t.Fatalf("parseConnectRequest() = %+v", req)
	}

	if _, ok := parseConnectRequest([]byte("not-json")); ok {
		t.Fatal("parseConnectRequest() unexpectedly accepted invalid json")
	}
	if _, ok := parseConnectRequest([]byte(`{"cmd":"other"}`)); ok {
		t.Fatal("parseConnectRequest() unexpectedly accepted wrong command")
	}
}

func TestAuthorizeRequest(t *testing.T) {
	s := &Server{clientID: "client-1"}
	if !s.authorizeRequest(ConnectRequest{ClientID: "client-1"}) {
		t.Fatal("authorizeRequest() rejected valid client")
	}
	if s.authorizeRequest(ConnectRequest{ClientID: "client-2"}) {
		t.Fatal("authorizeRequest() accepted wrong client")
	}
}

//nolint:cyclop // table-driven test naturally has many branches
func TestSocks5ConnectSuccess(t *testing.T) {
	s := &Server{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- s.socks5Connect(server, "example.com", 443)
	}()

	auth := make([]byte, 3)
	if _, err := io.ReadFull(client, auth); err != nil {
		t.Fatalf("ReadFull(auth) error = %v", err)
	}
	if !bytes.Equal(auth, []byte{5, 1, 0}) {
		t.Fatalf("auth request = %v", auth)
	}
	if _, err := client.Write([]byte{5, 0}); err != nil {
		t.Fatalf("Write(auth resp) error = %v", err)
	}

	req := make([]byte, 18)
	if _, err := io.ReadFull(client, req); err != nil {
		t.Fatalf("ReadFull(connect req) error = %v", err)
	}
	if req[0] != 5 || req[1] != 1 || req[3] != 3 || req[4] != byte(len("example.com")) {
		t.Fatalf("connect request header = %v", req[:5])
	}
	if string(req[5:16]) != "example.com" {
		t.Fatalf("connect request addr = %q", req[5:16])
	}
	if req[16] != 0x01 || req[17] != 0xbb {
		t.Fatalf("connect request port bytes = %v", req[16:18])
	}
	if _, err := client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("Write(connect resp) error = %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("socks5Connect() error = %v", err)
	}
}

func TestSocks5AddrBytes(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		wantType byte
		want     []byte
	}{
		{name: "ipv4", addr: "192.0.2.1", wantType: 1, want: []byte{192, 0, 2, 1}},
		{name: "ipv6", addr: "2001:db8::1", wantType: 4, want: net.ParseIP("2001:db8::1").To16()},
		{name: "domain", addr: "example.com", wantType: 3, want: []byte("example.com")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, got := socks5AddrBytes(tt.addr)
			if gotType != tt.wantType || !bytes.Equal(got, tt.want) {
				t.Fatalf("socks5AddrBytes() = (%d, %v), want (%d, %v)", gotType, got, tt.wantType, tt.want)
			}
		})
	}
}

func TestReadSocks5ConnectResponseConsumesVariableAddress(t *testing.T) {
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- readSocks5ConnectResponse(server)
	}()

	response := []byte{5, 0, 0, 3, byte(len("proxy.local"))}
	response = append(response, []byte("proxy.local")...)
	response = append(response, 0x1f, 0x90)
	response = append(response, []byte("next")...)
	writeDone := make(chan error, 1)
	go func() {
		_, err := client.Write(response)
		writeDone <- err
	}()

	if err := <-done; err != nil {
		t.Fatalf("readSocks5ConnectResponse() error = %v", err)
	}

	leftover := make([]byte, 4)
	if _, err := io.ReadFull(server, leftover); err != nil {
		t.Fatalf("ReadFull(leftover) error = %v", err)
	}
	if string(leftover) != "next" {
		t.Fatalf("leftover = %q, want next", leftover)
	}
	if err := <-writeDone; err != nil {
		t.Fatalf("Write() error = %v", err)
	}
}

func TestSocks5ConnectErrors(t *testing.T) {
	s := &Server{}

	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- s.socks5Connect(server, "example.com", 443)
	}()

	auth := make([]byte, 3)
	if _, err := io.ReadFull(client, auth); err != nil {
		t.Fatalf("ReadFull(auth) error = %v", err)
	}
	if _, err := client.Write([]byte{5, 1}); err != nil {
		t.Fatalf("Write(auth resp) error = %v", err)
	}
	if err := <-done; !errors.Is(err, ErrSocks5AuthFailed) {
		t.Fatalf("socks5Connect() error = %v, want %v", err, ErrSocks5AuthFailed)
	}

	server2, client2 := net.Pipe()
	defer func() {
		_ = server2.Close()
		_ = client2.Close()
	}()

	done = make(chan error, 1)
	go func() {
		done <- s.socks5Connect(server2, "example.com", 443)
	}()

	if _, err := io.ReadFull(client2, auth); err != nil {
		t.Fatalf("ReadFull(auth2) error = %v", err)
	}
	if _, err := client2.Write([]byte{5, 0}); err != nil {
		t.Fatalf("Write(auth2 resp) error = %v", err)
	}

	req := make([]byte, 18)
	if _, err := io.ReadFull(client2, req); err != nil {
		t.Fatalf("ReadFull(req2) error = %v", err)
	}
	if _, err := client2.Write([]byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("Write(connect2 resp) error = %v", err)
	}
	if err := <-done; !errors.Is(err, ErrSocks5ConnectFailed) {
		t.Fatalf("socks5Connect() error = %v, want %v", err, ErrSocks5ConnectFailed)
	}
}

func TestSetupResolver(t *testing.T) {
	s := &Server{dnsServer: "127.0.0.1:53"}
	s.setupResolver()
	if s.resolver == nil || !s.resolver.PreferGo || s.resolver.Dial == nil {
		t.Fatalf("setupResolver() = %+v", s.resolver)
	}
}

func TestOnDataWithNilConn(_ *testing.T) {
	s := &Server{}
	s.onData([]byte("ignored"))
}

type serverLinkStub struct {
	closed bool
}

func (s *serverLinkStub) Connect(context.Context) error   { return nil }
func (s *serverLinkStub) Send([]byte) error               { return nil }
func (s *serverLinkStub) Close() error                    { s.closed = true; return nil }
func (s *serverLinkStub) SetReconnectCallback(func())     {}
func (s *serverLinkStub) SetShouldReconnect(func() bool)  {}
func (s *serverLinkStub) SetEndedCallback(func(string))   {}
func (s *serverLinkStub) WatchConnection(context.Context) {}
func (s *serverLinkStub) CanSend() bool                   { return true }

func TestShutdownClosesLinkAndConn(t *testing.T) {
	cipher, err := cryptopkg.NewCipher("01234567890123456789012345678901")
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	ln := &serverLinkStub{}
	s := &Server{
		ln:     ln,
		cipher: cipher,
		conn:   muxconn.New(ln, cipher),
	}
	s.shutdown()
	if !ln.closed {
		t.Fatal("shutdown() did not close link")
	}
}

func TestDialWithoutProxy(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
			close(done)
		}
	}()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr type = %T, want *net.TCPAddr", ln.Addr())
	}
	s := &Server{resolver: net.DefaultResolver}
	conn, err := s.dial(ConnectRequest{Addr: "127.0.0.1", Port: tcpAddr.Port})
	if err != nil {
		t.Fatalf("dial() error = %v", err)
	}
	_ = conn.Close()
	<-done
}

func TestDialWithoutProxyIPv6(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
			close(done)
		}
	}()

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener addr type = %T, want *net.TCPAddr", ln.Addr())
	}
	s := &Server{resolver: net.DefaultResolver}
	conn, err := s.dial(ConnectRequest{Addr: "::1", Port: tcpAddr.Port})
	if err != nil {
		t.Fatalf("dial() error = %v", err)
	}
	_ = conn.Close()
	<-done
}

func TestDialProxyError(t *testing.T) {
	s := &Server{socksProxyAddr: "127.0.0.1", socksProxyPort: 1}
	if _, err := s.dial(ConnectRequest{Addr: "example.com", Port: 443}); err == nil || !strings.Contains(err.Error(), "failed to dial proxy") { //nolint:lll // long test description
		t.Fatalf("dial() error = %v", err)
	}
}

func TestSocks5ConnectTruncatesLongDomain(t *testing.T) {
	s := &Server{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	longHost := strings.Repeat("a", 300)
	done := make(chan error, 1)
	go func() {
		done <- s.socks5Connect(server, longHost, 443)
	}()

	auth := make([]byte, 3)
	if _, err := io.ReadFull(client, auth); err != nil {
		t.Fatalf("ReadFull(auth) error = %v", err)
	}
	if _, err := client.Write([]byte{5, 0}); err != nil {
		t.Fatalf("Write(auth resp) error = %v", err)
	}

	req := make([]byte, 262)
	if _, err := io.ReadFull(client, req); err != nil {
		t.Fatalf("ReadFull(connect req) error = %v", err)
	}
	if req[4] != 255 {
		t.Fatalf("domain len byte = %d, want 255", req[4])
	}
	if _, err := client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("Write(connect resp) error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("socks5Connect() error = %v", err)
	}
}

func TestHandleStreamRejectsWrongClientID(t *testing.T) {
	a, b := net.Pipe()
	defer func() {
		_ = a.Close()
		_ = b.Close()
	}()

	serverSess, err := smux.Server(a, smuxConfig())
	if err != nil {
		t.Fatalf("smux.Server() error = %v", err)
	}
	defer func() { _ = serverSess.Close() }()
	clientSess, err := smux.Client(b, smuxConfig())
	if err != nil {
		t.Fatalf("smux.Client() error = %v", err)
	}
	defer func() { _ = clientSess.Close() }()

	done := make(chan struct{})
	go func() {
		stream, err := serverSess.AcceptStream()
		if err == nil {
			(&Server{clientID: "expected"}).handleStream(context.Background(), stream)
		}
		close(done)
	}()

	stream, err := clientSess.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	req, err := json.Marshal(ConnectRequest{
		Cmd:      "connect",
		ClientID: "wrong",
		Addr:     "example.com",
		Port:     443,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := stream.Write(req); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	<-done
}
