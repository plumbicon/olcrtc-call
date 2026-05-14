package jazz

import (
	"context"
	"errors"
	"testing"

	"github.com/openlibrecommunity/olcrtc/internal/provider"
	"github.com/pion/webrtc/v4"
)

//nolint:cyclop // table-driven test naturally has many branches
func TestPeerStateHelpers(t *testing.T) {
	p := &Peer{
		reconnectCh:    make(chan struct{}, 1),
		closeCh:        make(chan struct{}),
		sessionCloseCh: make(chan struct{}),
		sendQueue:      make(chan []byte, 1),
		subscriberConn: make(chan struct{}),
		publisherConn:  make(chan struct{}),
	}

	p.resetMediaState()
	if p.subscriberReady.Load() || p.publisherReady.Load() || p.subscriberConn == nil || p.publisherConn == nil {
		t.Fatal("resetMediaState() did not reset readiness")
	}
	if p.hasLocalVideoTracks() {
		t.Fatal("hasLocalVideoTracks() = true without tracks")
	}
	if err := p.AddVideoTrack(nil); err != nil {
		t.Fatalf("AddVideoTrack(nil) error = %v", err)
	}
	if !p.hasLocalVideoTracks() {
		t.Fatal("hasLocalVideoTracks() = false after AddVideoTrack")
	}

	p.SetVideoTrackHandler(func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {})
	if p.videoTrackHandler() == nil {
		t.Fatal("videoTrackHandler() = nil")
	}

	cfg := defaultWebRTCConfig()
	if cfg.SDPSemantics != webrtc.SDPSemanticsUnifiedPlan || cfg.BundlePolicy != webrtc.BundlePolicyMaxBundle {
		t.Fatalf("defaultWebRTCConfig() = %+v", cfg)
	}
	if p.buildAPI() == nil {
		t.Fatal("buildAPI() returned nil")
	}
}

func TestPeerCallbacksQueueReconnectAndClose(t *testing.T) {
	p := &Peer{
		reconnectCh:    make(chan struct{}, 1),
		closeCh:        make(chan struct{}),
		sessionCloseCh: make(chan struct{}),
		sendQueue:      make(chan []byte, 1),
	}

	p.SetReconnectCallback(func(*webrtc.DataChannel) {})
	p.SetShouldReconnect(func() bool { return true })
	p.SetEndedCallback(func(string) {})
	if p.onReconnect == nil || p.shouldReconnect == nil || p.onEnded == nil {
		t.Fatal("callbacks were not stored")
	}

	p.queueReconnect()
	select {
	case <-p.reconnectCh:
	default:
		t.Fatal("queueReconnect() did not enqueue")
	}

	p.SetShouldReconnect(func() bool { return false })
	p.queueReconnect()
	select {
	case <-p.reconnectCh:
		t.Fatal("queueReconnect() enqueued despite policy=false")
	default:
	}

	done := make(chan struct{})
	go func() {
		p.WatchConnection(context.Background())
		close(done)
	}()
	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	<-done
	if err := p.Send([]byte("closed")); !errors.Is(err, provider.ErrDataChannelNotReady) {
		t.Fatalf("Send() error = %v, want datachannel not ready", err)
	}
}

func TestPeerCanSendVideoOnlyModes(t *testing.T) {
	p := &Peer{sendQueue: make(chan []byte, 1)}
	p.subscriberReady.Store(true)
	if !p.CanSend() {
		t.Fatal("CanSend() = false for subscriber-ready peer without local video")
	}
	_ = p.AddVideoTrack(nil)
	if p.CanSend() {
		t.Fatal("CanSend() = true with local video but publisher not ready")
	}
	p.publisherReady.Store(true)
	if !p.CanSend() {
		t.Fatal("CanSend() = false with subscriber and publisher ready")
	}
	p.closed.Store(true)
	if p.CanSend() {
		t.Fatal("CanSend() = true for closed peer")
	}
}
