//nolint:all // Test file keeps scenario setup inline.
package client

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"

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

func TestSocks5Handshake(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- c.socks5Handshake(server)
	}()

	if _, err := client.Write([]byte{5, 1, 0}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	resp := make([]byte, 2)
	if _, err := io.ReadFull(client, resp); err != nil {
		t.Fatalf("ReadFull() error = %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("socks5Handshake() error = %v", err)
	}
	if !bytes.Equal(resp, []byte{5, 0}) {
		t.Fatalf("handshake response = %v, want [5 0]", resp)
	}
}

func TestSocks5HandshakeRejectsVersion(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- c.socks5Handshake(server)
	}()

	if _, err := client.Write([]byte{4, 1}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := <-done; !errors.Is(err, ErrInvalidSOCKSVersion) {
		t.Fatalf("socks5Handshake() error = %v, want %v", err, ErrInvalidSOCKSVersion)
	}
}

func TestSocks5HandshakeReadMethodsError(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		done <- c.socks5Handshake(server)
	}()

	if _, err := client.Write([]byte{5, 2, 0}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	_ = client.Close()
	if err := <-done; err == nil {
		t.Fatal("socks5Handshake() unexpectedly succeeded")
	}
}

func TestSocks5RequestIPv4(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan struct {
		addr string
		port int
		err  error
	}, 1)
	go func() {
		addr, port, err := c.socks5Request(server)
		done <- struct {
			addr string
			port int
			err  error
		}{addr: addr, port: port, err: err}
	}()

	req := []byte{5, 1, 0, 1, 127, 0, 0, 1}
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, 8080)
	if _, err := client.Write(append(req, port...)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	res := <-done
	if res.err != nil {
		t.Fatalf("socks5Request() error = %v", res.err)
	}
	if res.addr != "127.0.0.1" || res.port != 8080 {
		t.Fatalf("socks5Request() = (%q, %d), want (127.0.0.1, 8080)", res.addr, res.port)
	}
}

func TestSocks5RequestIPv6(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan struct {
		addr string
		port int
		err  error
	}, 1)
	go func() {
		addr, port, err := c.socks5Request(server)
		done <- struct {
			addr string
			port int
			err  error
		}{addr: addr, port: port, err: err}
	}()

	req := []byte{5, 1, 0, 4}
	req = append(req, net.ParseIP("2001:db8::1").To16()...)
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, 443)
	if _, err := client.Write(append(req, port...)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	res := <-done
	if res.err != nil {
		t.Fatalf("socks5Request() error = %v", res.err)
	}
	if res.addr != "2001:db8::1" || res.port != 443 {
		t.Fatalf("socks5Request() = (%q, %d), want (2001:db8::1, 443)", res.addr, res.port)
	}
}

func TestSocks5RequestDomain(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan struct {
		addr string
		port int
		err  error
	}, 1)
	go func() {
		addr, port, err := c.socks5Request(server)
		done <- struct {
			addr string
			port int
			err  error
		}{addr: addr, port: port, err: err}
	}()

	req := []byte{5, 1, 0, 3, 11}
	req = append(req, []byte("example.com")...)
	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, 443)
	if _, err := client.Write(append(req, port...)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	res := <-done
	if res.err != nil {
		t.Fatalf("socks5Request() error = %v", res.err)
	}
	if res.addr != "example.com" || res.port != 443 {
		t.Fatalf("socks5Request() = (%q, %d), want (example.com, 443)", res.addr, res.port)
	}
}

func TestSocks5RequestRejectsCommandAndAddressType(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := c.socks5Request(server)
		done <- err
	}()

	if _, err := client.Write([]byte{5, 2, 0, 1}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := <-done; !errors.Is(err, ErrUnsupportedSOCKSCommand) {
		t.Fatalf("socks5Request() error = %v, want %v", err, ErrUnsupportedSOCKSCommand)
	}

	server2, client2 := net.Pipe()
	defer func() {
		_ = server2.Close()
		_ = client2.Close()
	}()

	done = make(chan error, 1)
	go func() {
		_, _, err := c.socks5Request(server2)
		done <- err
	}()

	if _, err := client2.Write([]byte{5, 1, 0, 9}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := <-done; !errors.Is(err, ErrUnsupportedAddressType) {
		t.Fatalf("socks5Request() error = %v, want %v", err, ErrUnsupportedAddressType)
	}
}

func TestSocks5RequestReadPortError(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := c.socks5Request(server)
		done <- err
	}()

	if _, err := client.Write([]byte{5, 1, 0, 1, 127, 0, 0, 1, 0}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	_ = client.Close()
	if err := <-done; err == nil {
		t.Fatal("socks5Request() unexpectedly succeeded")
	}
}

func TestReplyBuffers(t *testing.T) {
	if !bytes.Equal(replySuccess(), []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("replySuccess() = %v", replySuccess())
	}
	if !bytes.Equal(replyHostUnreachable(), []byte{5, 4, 0, 1, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("replyHostUnreachable() = %v", replyHostUnreachable())
	}
}

func TestReadSocks5AddrReadErrors(t *testing.T) {
	c := &Client{}
	server, client := net.Pipe()
	defer func() {
		_ = server.Close()
		_ = client.Close()
	}()

	done := make(chan error, 1)
	go func() {
		_, err := c.readSocks5Addr(server, 1)
		done <- err
	}()

	time.Sleep(10 * time.Millisecond)
	_ = client.Close()
	if err := <-done; err == nil {
		t.Fatal("readSocks5Addr() unexpectedly succeeded")
	}
}

func TestSendConnectRequestOverSmux(t *testing.T) {
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

	done := make(chan error, 1)
	go func() {
		stream, err := serverSess.AcceptStream()
		if err != nil {
			done <- err
			return
		}
		defer func() { _ = stream.Close() }()

		var req map[string]any
		if err := json.NewDecoder(stream).Decode(&req); err != nil {
			done <- err
			return
		}
		if req["cmd"] != "connect" || req["clientId"] != "client-1" || req["addr"] != "example.com" {
			done <- errors.New("unexpected connect request")
			return
		}
		_, err = stream.Write([]byte{0x00})
		done <- err
	}()

	stream, err := clientSess.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	c := &Client{clientID: "client-1"}
	if err := c.sendConnectRequest(stream, "example.com", 443); err != nil {
		t.Fatalf("sendConnectRequest() error = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("server side error = %v", err)
	}
}

func TestSendConnectRequestRejectsBadAck(t *testing.T) {
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

	go func() {
		stream, err := serverSess.AcceptStream()
		if err != nil {
			return
		}
		defer func() { _ = stream.Close() }()
		_, _ = io.CopyN(io.Discard, stream, 1)
		_, _ = stream.Write([]byte{0x01})
	}()

	stream, err := clientSess.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream() error = %v", err)
	}
	defer func() { _ = stream.Close() }()

	c := &Client{clientID: "client-1"}
	if err := c.sendConnectRequest(stream, "example.com", 443); !errors.Is(err, ErrRemoteNotReady) {
		t.Fatalf("sendConnectRequest() error = %v, want %v", err, ErrRemoteNotReady)
	}
}

type closerLinkStub struct {
	closed bool
}

func (s *closerLinkStub) Connect(context.Context) error   { return nil }
func (s *closerLinkStub) Send([]byte) error               { return nil }
func (s *closerLinkStub) Close() error                    { s.closed = true; return nil }
func (s *closerLinkStub) SetReconnectCallback(func())     {}
func (s *closerLinkStub) SetShouldReconnect(func() bool)  {}
func (s *closerLinkStub) SetEndedCallback(func(string))   {}
func (s *closerLinkStub) WatchConnection(context.Context) {}
func (s *closerLinkStub) CanSend() bool                   { return true }

func TestOnDataWithNilConn(t *testing.T) {
	c := &Client{}
	c.onData([]byte("ignored"))
}

func TestShutdownClosesLinkAndConn(t *testing.T) {
	cipher, err := cryptopkg.NewCipher("01234567890123456789012345678901")
	if err != nil {
		t.Fatalf("NewCipher() error = %v", err)
	}
	ln := &closerLinkStub{}
	c := &Client{
		ln:     ln,
		cipher: cipher,
		conn:   muxconn.New(ln, cipher),
	}
	c.shutdown()
	if !ln.closed {
		t.Fatal("shutdown() did not close link")
	}
}
