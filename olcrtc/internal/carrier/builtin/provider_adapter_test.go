package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/openlibrecommunity/olcrtc/internal/carrier"
	"github.com/pion/webrtc/v4"
)

var (
	errConnectBoom = errors.New("connect boom")
	errSendBoom    = errors.New("send boom")
	errCloseBoom   = errors.New("close boom")
	errTrackBoom   = errors.New("track boom")
)

type stubProvider struct {
	connectErr         error
	sendErr            error
	closeErr           error
	canSend            bool
	reconnectCallback  func(*webrtc.DataChannel)
	shouldReconnect    func() bool
	endedCallback      func(string)
	watchCalled        bool
	addTrackErr        error
	trackHandlerCalled bool
}

func (s *stubProvider) Connect(context.Context) error                     { return s.connectErr }
func (s *stubProvider) Send([]byte) error                                 { return s.sendErr }
func (s *stubProvider) Close() error                                      { return s.closeErr }
func (s *stubProvider) SetReconnectCallback(cb func(*webrtc.DataChannel)) { s.reconnectCallback = cb }
func (s *stubProvider) SetShouldReconnect(fn func() bool)                 { s.shouldReconnect = fn }
func (s *stubProvider) SetEndedCallback(cb func(string))                  { s.endedCallback = cb }
func (s *stubProvider) WatchConnection(context.Context)                   { s.watchCalled = true }
func (s *stubProvider) CanSend() bool                                     { return s.canSend }
func (s *stubProvider) GetSendQueue() chan []byte                         { return nil }
func (s *stubProvider) GetBufferedAmount() uint64                         { return 0 }
func (s *stubProvider) AddVideoTrack(webrtc.TrackLocal) error             { return s.addTrackErr }
func (s *stubProvider) SetVideoTrackHandler(func(*webrtc.TrackRemote, *webrtc.RTPReceiver)) {
	s.trackHandlerCalled = true
}

type plainProvider struct {
	connectErr        error
	sendErr           error
	closeErr          error
	canSend           bool
	reconnectCallback func(*webrtc.DataChannel)
	shouldReconnect   func() bool
	endedCallback     func(string)
	watchCalled       bool
}

func (p *plainProvider) Connect(context.Context) error                     { return p.connectErr }
func (p *plainProvider) Send([]byte) error                                 { return p.sendErr }
func (p *plainProvider) Close() error                                      { return p.closeErr }
func (p *plainProvider) SetReconnectCallback(cb func(*webrtc.DataChannel)) { p.reconnectCallback = cb }
func (p *plainProvider) SetShouldReconnect(fn func() bool)                 { p.shouldReconnect = fn }
func (p *plainProvider) SetEndedCallback(cb func(string))                  { p.endedCallback = cb }
func (p *plainProvider) WatchConnection(context.Context)                   { p.watchCalled = true }
func (p *plainProvider) CanSend() bool                                     { return p.canSend }
func (p *plainProvider) GetSendQueue() chan []byte                         { return nil }
func (p *plainProvider) GetBufferedAmount() uint64                         { return 0 }

func TestProviderSessionOpenVideoTrackUnsupported(t *testing.T) {
	sess := &providerSession{provider: &plainProvider{}}

	caps := sess.Capabilities()
	if !caps.ByteStream || caps.VideoTrack {
		t.Fatalf("Capabilities() = %+v, want byte true and video false", caps)
	}

	_, err := sess.OpenVideoTrack()
	if !errors.Is(err, carrier.ErrVideoTrackUnsupported) {
		t.Fatalf("OpenVideoTrack() error = %v, want %v", err, carrier.ErrVideoTrackUnsupported)
	}
}

func TestProviderByteStreamWrapsProviderAndCallbacks(t *testing.T) {
	prov := &stubProvider{canSend: true}
	stream := &providerByteStream{provider: prov}

	called := false
	stream.SetReconnectCallback(func() { called = true })
	if prov.reconnectCallback == nil {
		t.Fatal("SetReconnectCallback() did not install provider callback")
	}
	prov.reconnectCallback(nil)
	if !called {
		t.Fatal("reconnect callback was not adapted")
	}

	reconnectAllowed := false
	stream.SetShouldReconnect(func() bool { reconnectAllowed = true; return true })
	if prov.shouldReconnect == nil || !prov.shouldReconnect() || !reconnectAllowed {
		t.Fatal("SetShouldReconnect() was not forwarded")
	}

	ended := ""
	stream.SetEndedCallback(func(reason string) { ended = reason })
	if prov.endedCallback == nil {
		t.Fatal("SetEndedCallback() was not forwarded")
	}
	prov.endedCallback("bye")
	if ended != "bye" {
		t.Fatalf("ended callback reason = %q, want bye", ended)
	}

	stream.WatchConnection(context.Background())
	if !prov.watchCalled {
		t.Fatal("WatchConnection() was not forwarded")
	}
	if !stream.CanSend() {
		t.Fatal("CanSend() = false, want true")
	}
}

func TestProviderByteStreamWrapsErrors(t *testing.T) {
	prov := &stubProvider{
		connectErr: errConnectBoom,
		sendErr:    errSendBoom,
		closeErr:   errCloseBoom,
	}
	stream := &providerByteStream{provider: prov}

	if err := stream.Connect(context.Background()); err == nil || err.Error() != "connect: connect boom" {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := stream.Send([]byte("x")); err == nil || err.Error() != "send: send boom" {
		t.Fatalf("Send() error = %v", err)
	}
	if err := stream.Close(); err == nil || err.Error() != "close: close boom" {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestProviderSessionOpenByteStreamAndVideoTrack(t *testing.T) {
	prov := &stubProvider{canSend: true}
	sess := &providerSession{provider: prov}

	stream, err := sess.OpenByteStream()
	if err != nil {
		t.Fatalf("OpenByteStream() error = %v", err)
	}
	if !stream.CanSend() {
		t.Fatal("byte stream CanSend() = false, want true")
	}

	video, err := sess.OpenVideoTrack()
	if err != nil {
		t.Fatalf("OpenVideoTrack() error = %v", err)
	}
	if err := video.Connect(context.Background()); err != nil {
		t.Fatalf("video Connect() error = %v", err)
	}
	if err := video.Close(); err != nil {
		t.Fatalf("video Close() error = %v", err)
	}
	video.SetShouldReconnect(func() bool { return true })
	video.SetEndedCallback(func(string) {})
	video.WatchConnection(context.Background())
	if !video.CanSend() || prov.shouldReconnect == nil || prov.endedCallback == nil || !prov.watchCalled {
		t.Fatal("video adapter did not forward calls")
	}
}

func TestProviderVideoTrackWrapsOperations(t *testing.T) {
	prov := &stubProvider{canSend: true, addTrackErr: errTrackBoom}
	track := &providerVideoTrack{provider: prov}

	called := false
	track.SetReconnectCallback(func() { called = true })
	prov.reconnectCallback(nil)
	if !called {
		t.Fatal("reconnect callback was not adapted")
	}

	track.SetTrackHandler(func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {})
	if !prov.trackHandlerCalled {
		t.Fatal("SetTrackHandler() was not forwarded")
	}

	if err := track.AddTrack(nil); err == nil || err.Error() != "add track: track boom" {
		t.Fatalf("AddTrack() error = %v", err)
	}
}

func TestProviderVideoTrackWrapsConnectCloseErrors(t *testing.T) {
	prov := &stubProvider{
		connectErr: errConnectBoom,
		closeErr:   errCloseBoom,
	}
	track := &providerVideoTrack{provider: prov}

	if err := track.Connect(context.Background()); err == nil || err.Error() != "connect: connect boom" {
		t.Fatalf("Connect() error = %v", err)
	}
	if err := track.Close(); err == nil || err.Error() != "close: close boom" {
		t.Fatalf("Close() error = %v", err)
	}
}
