// Package provider defines the interface and registry for different WebRTC providers.
package provider

import (
	"context"
	"errors"

	"github.com/pion/webrtc/v4"
)

var (
	// ErrDataChannelTimeout is returned when the DataChannel fails to open within the timeout period.
	ErrDataChannelTimeout = errors.New("datachannel timeout")
	// ErrDataChannelNotReady is returned when attempting to send data before the DataChannel is open.
	ErrDataChannelNotReady = errors.New("datachannel not ready")
	// ErrSendQueueClosed is returned when attempting to send data after the send queue has been closed.
	ErrSendQueueClosed = errors.New("send queue closed")
	// ErrSendQueueTimeout is returned when the send queue is full and the timeout is reached.
	ErrSendQueueTimeout = errors.New("send queue timeout")
)

// Provider defines the standard interface for WebRTC connection handlers.
type Provider interface {
	Connect(ctx context.Context) error
	Send(data []byte) error
	Close() error
	SetReconnectCallback(cb func(*webrtc.DataChannel))
	SetShouldReconnect(fn func() bool)
	SetEndedCallback(cb func(string))
	WatchConnection(ctx context.Context)
	CanSend() bool
	GetSendQueue() chan []byte
	GetBufferedAmount() uint64
}

// VideoTrackCapable is implemented by providers that can exchange video tracks.
type VideoTrackCapable interface {
	AddVideoTrack(track webrtc.TrackLocal) error
	SetVideoTrackHandler(cb func(*webrtc.TrackRemote, *webrtc.RTPReceiver))
}

// Config holds common configuration for all providers.
type Config struct {
	RoomURL   string
	Name      string
	OnData    func([]byte)
	DNSServer string
	ProxyAddr string
	ProxyPort int
}
