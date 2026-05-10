package jazz

import (
	"context"
	"errors"
	"testing"

	"github.com/openlibrecommunity/olcrtc/internal/provider"
	"github.com/pion/webrtc/v4"
)

func TestJazzProviderForwardsPeerMethods(t *testing.T) {
	peer := &Peer{
		reconnectCh:    make(chan struct{}, 1),
		closeCh:        make(chan struct{}),
		sessionCloseCh: make(chan struct{}),
		sendQueue:      make(chan []byte, 1),
	}
	p := &jazzProvider{peer: peer}

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
		t.Fatal("GetBufferedAmount() != 0 with nil datachannel")
	}
	if err := p.AddVideoTrack(nil); err != nil {
		t.Fatalf("AddVideoTrack(nil) error = %v", err)
	}
	if err := p.Send([]byte("x")); !errors.Is(err, provider.ErrDataChannelNotReady) {
		t.Fatalf("Send() error = %v, want datachannel not ready", err)
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
}
