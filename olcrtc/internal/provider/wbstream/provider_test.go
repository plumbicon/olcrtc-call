package wbstream

import (
	"context"
	"errors"
	"testing"

	"github.com/pion/webrtc/v4"
)

//nolint:cyclop // table-driven test naturally has many branches
func TestWBStreamProviderForwardsPeerMethods(t *testing.T) {
	peer, err := NewPeer(context.Background(), "room", "name", nil)
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}
	p := &wbStreamProvider{peer: peer}

	p.SetReconnectCallback(func(*webrtc.DataChannel) {})
	p.SetShouldReconnect(func() bool { return true })
	p.SetEndedCallback(func(string) {})
	p.SetVideoTrackHandler(func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {})
	if peer.onReconnect == nil || peer.shouldReconnect == nil || peer.onEnded == nil || peer.onVideoTrack == nil {
		t.Fatal("callbacks were not forwarded")
	}

	if p.GetSendQueue() != peer.sendQueue {
		t.Fatal("GetSendQueue() did not forward")
	}
	if p.GetBufferedAmount() != 0 {
		t.Fatal("GetBufferedAmount() != 0")
	}
	if err := p.AddVideoTrack(nil); err != nil {
		t.Fatalf("AddVideoTrack(nil) error = %v", err)
	}
	if p.CanSend() {
		t.Fatal("CanSend() = true without LiveKit room")
	}
	p.WatchConnection(context.Background())

	if err := p.Send([]byte("x")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := p.Send([]byte("x")); !errors.Is(err, ErrPeerClosed) {
		t.Fatalf("Send() error = %v, want peer closed", err)
	}
}
