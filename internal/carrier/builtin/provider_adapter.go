package builtin

import (
	"context"
	"fmt"

	"github.com/openlibrecommunity/olcrtc/internal/carrier"
	"github.com/openlibrecommunity/olcrtc/internal/provider"
	"github.com/pion/webrtc/v4"
)

type providerSession struct {
	provider provider.Provider
}

func (s *providerSession) Capabilities() carrier.Capabilities {
	caps := carrier.Capabilities{ByteStream: true}
	_, caps.VideoTrack = s.provider.(videoTrackProvider)
	return caps
}

func (s *providerSession) OpenByteStream() (carrier.ByteStream, error) {
	return &providerByteStream{provider: s.provider}, nil
}

func (s *providerSession) OpenVideoTrack() (carrier.VideoTrack, error) {
	vtp, ok := s.provider.(videoTrackProvider)
	if !ok {
		return nil, carrier.ErrVideoTrackUnsupported
	}
	return &providerVideoTrack{provider: vtp}, nil
}

type videoTrackProvider interface {
	provider.Provider
	provider.VideoTrackCapable
}

type providerByteStream struct {
	provider provider.Provider
}

func (p *providerByteStream) Connect(ctx context.Context) error {
	if err := p.provider.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

func (p *providerByteStream) Send(data []byte) error {
	if err := p.provider.Send(data); err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}

func (p *providerByteStream) Close() error {
	if err := p.provider.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

func (p *providerByteStream) SetReconnectCallback(cb func()) {
	p.provider.SetReconnectCallback(func(_ *webrtc.DataChannel) {
		if cb != nil {
			cb()
		}
	})
}

func (p *providerByteStream) SetShouldReconnect(fn func() bool) { p.provider.SetShouldReconnect(fn) }
func (p *providerByteStream) SetEndedCallback(cb func(string))  { p.provider.SetEndedCallback(cb) }
func (p *providerByteStream) WatchConnection(ctx context.Context) {
	p.provider.WatchConnection(ctx)
}
func (p *providerByteStream) CanSend() bool { return p.provider.CanSend() }

type providerVideoTrack struct {
	provider videoTrackProvider
}

func (v *providerVideoTrack) Connect(ctx context.Context) error {
	if err := v.provider.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

func (v *providerVideoTrack) Close() error {
	if err := v.provider.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

func (v *providerVideoTrack) SetReconnectCallback(cb func()) {
	v.provider.SetReconnectCallback(func(_ *webrtc.DataChannel) {
		if cb != nil {
			cb()
		}
	})
}

func (v *providerVideoTrack) SetShouldReconnect(fn func() bool) { v.provider.SetShouldReconnect(fn) }
func (v *providerVideoTrack) SetEndedCallback(cb func(string))  { v.provider.SetEndedCallback(cb) }
func (v *providerVideoTrack) WatchConnection(ctx context.Context) {
	v.provider.WatchConnection(ctx)
}
func (v *providerVideoTrack) CanSend() bool { return v.provider.CanSend() }

func (v *providerVideoTrack) AddTrack(track webrtc.TrackLocal) error {
	if err := v.provider.AddVideoTrack(track); err != nil {
		return fmt.Errorf("add track: %w", err)
	}
	return nil
}

func (v *providerVideoTrack) SetTrackHandler(cb func(*webrtc.TrackRemote, *webrtc.RTPReceiver)) {
	v.provider.SetVideoTrackHandler(cb)
}
