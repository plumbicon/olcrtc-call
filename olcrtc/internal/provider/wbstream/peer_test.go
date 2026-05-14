package wbstream

import (
	"context"
	"errors"
	"testing"

	"github.com/pion/webrtc/v4"
)

func TestNewPeerAndSimpleAccessors(t *testing.T) {
	p, err := NewPeer(context.Background(), "room", "name", func([]byte) {})
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}
	if p.roomURL != "room" || p.name != "name" || p.sendQueue == nil || p.done == nil { //nolint:goconst,lll // test literal, repetition is intentional
		t.Fatalf("NewPeer() = %+v", p)
	}
	if p.GetSendQueue() != p.sendQueue {
		t.Fatal("GetSendQueue() did not return sendQueue")
	}
	if p.GetBufferedAmount() != 0 {
		t.Fatal("GetBufferedAmount() != 0")
	}
	if p.CanSend() {
		t.Fatal("CanSend() = true without room")
	}
}

func TestSendQueueAndClose(t *testing.T) {
	p, err := NewPeer(context.Background(), "room", "name", nil)
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}
	p.sendQueue = make(chan []byte, 1)

	if err := p.Send([]byte("one")); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if err := p.Send([]byte("two")); !errors.Is(err, ErrSendQueueFull) {
		t.Fatalf("Send() error = %v, want %v", err, ErrSendQueueFull)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := p.Send([]byte("closed")); !errors.Is(err, ErrPeerClosed) {
		t.Fatalf("Send() error = %v, want %v", err, ErrPeerClosed)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

func TestCallbacksAndVideoTrackStorage(t *testing.T) {
	p, err := NewPeer(context.Background(), "room", "name", nil)
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}

	p.SetReconnectCallback(func(*webrtc.DataChannel) {})
	p.SetShouldReconnect(func() bool { return true })
	p.SetEndedCallback(func(string) {})
	p.SetVideoTrackHandler(func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {})
	p.WatchConnection(context.Background())

	if p.onReconnect == nil || p.shouldReconnect == nil || p.onEnded == nil || p.onVideoTrack == nil {
		t.Fatal("callbacks were not stored")
	}

	if err := p.AddVideoTrack(nil); err != nil {
		t.Fatalf("AddVideoTrack(nil) error = %v", err)
	}
	if len(p.videoTracks) != 1 {
		t.Fatalf("videoTracks len = %d, want 1", len(p.videoTracks))
	}
}
