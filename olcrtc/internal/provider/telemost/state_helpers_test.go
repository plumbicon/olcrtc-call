package telemost

import (
	"testing"
	"time"
)

//nolint:cyclop // table-driven test naturally has many branches
func TestSessionReconnectAndEndedHelpers(t *testing.T) {
	p := &Peer{
		reconnectCh:    make(chan struct{}, 2),
		closeCh:        make(chan struct{}),
		keepAliveCh:    make(chan struct{}),
		sessionCloseCh: make(chan struct{}),
		telemetryCh:    make(chan struct{}, 1),
	}

	keepAliveCh, sessionCloseCh := p.resetSession()
	if keepAliveCh == nil || sessionCloseCh == nil || keepAliveCh != p.keepAliveCh || sessionCloseCh != p.sessionCloseCh {
		t.Fatal("resetSession() did not replace session channels")
	}

	p.subscriberReady.Store(true)
	p.publisherReady.Store(true)
	p.resetMediaState()
	if p.subscriberReady.Load() || p.publisherReady.Load() || p.subscriberConn == nil || p.publisherConn == nil {
		t.Fatal("resetMediaState() did not reset readiness")
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

	p.reconnectCh <- struct{}{}
	p.reconnectCh <- struct{}{}
	p.drainReconnectQueue()
	select {
	case <-p.reconnectCh:
		t.Fatal("drainReconnectQueue() left queued item")
	default:
	}

	p.telemetryActive.Store(true)
	p.stopTelemetry()
	select {
	case <-p.telemetryCh:
	default:
		t.Fatal("stopTelemetry() did not signal active telemetry")
	}

	ended := ""
	p.SetEndedCallback(func(reason string) { ended = reason })
	p.signalEnded("done")
	if !p.closed.Load() || ended != "done" {
		t.Fatalf("signalEnded() closed=%v reason=%q", p.closed.Load(), ended)
	}
}

func TestWaitForAckTimeoutAndClose(t *testing.T) {
	p := &Peer{
		closeCh:    make(chan struct{}),
		ackWaiters: make(map[string]chan struct{}),
	}
	ch := p.registerAckWaiter("timeout")
	if p.waitForAck("timeout", ch, time.Millisecond) {
		t.Fatal("waitForAck(timeout) = true")
	}

	ch = p.registerAckWaiter("closed")
	close(p.closeCh)
	if p.waitForAck("closed", ch, time.Second) {
		t.Fatal("waitForAck(closeCh) = true")
	}
}
