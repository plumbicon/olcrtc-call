package telemost

import (
	"testing"
	"time"

	"github.com/pion/webrtc/v4"
)

func TestCloseSignal(t *testing.T) {
	closeSignal(nil)

	ch := make(chan struct{})
	closeSignal(ch)
	select {
	case <-ch:
	default:
		t.Fatal("closeSignal() did not close channel")
	}
	closeSignal(ch)
}

func TestTrafficShapeAndDelay(t *testing.T) {
	p := &Peer{}
	p.SetTrafficShape(TrafficShape{MaxMessageSize: -1, MinDelay: 5 * time.Millisecond, MaxDelay: 2 * time.Millisecond})
	if p.trafficShape.MaxMessageSize != realDataChannelMessageLimit {
		t.Fatalf("MaxMessageSize = %d, want default", p.trafficShape.MaxMessageSize)
	}
	if p.trafficShape.MaxDelay != p.trafficShape.MinDelay {
		t.Fatalf("MaxDelay = %v, want %v", p.trafficShape.MaxDelay, p.trafficShape.MinDelay)
	}
	if got := p.calculateDelay(); got != 5*time.Millisecond {
		t.Fatalf("calculateDelay() = %v, want 5ms", got)
	}

	p.SetTrafficShape(TrafficShape{MaxMessageSize: 10, MinDelay: time.Millisecond, MaxDelay: 4 * time.Millisecond})
	for range 20 {
		got := p.calculateDelay()
		if got < time.Millisecond || got >= 4*time.Millisecond {
			t.Fatalf("calculateDelay() = %v, out of range", got)
		}
	}
}

func TestICEParsingFiltersTURN(t *testing.T) {
	if isNonTURNURL("") || isNonTURNURL("turn:host") || isNonTURNURL("turns:host") {
		t.Fatal("isNonTURNURL accepted empty or TURN URL")
	}
	if !isNonTURNURL("stun:host") {
		t.Fatal("isNonTURNURL rejected STUN URL")
	}

	urls := parseICEURLs(map[string]interface{}{"urls": []interface{}{"turn:x", "stun:a", 123, "turns:y"}}) //nolint:goconst,lll // test literal, repetition is intentional
	if len(urls) != 1 || urls[0] != "stun:a" {
		t.Fatalf("parseICEURLs(interface) = %v, want [stun:a]", urls)
	}

	urls = parseICEURLs(map[string]interface{}{"urls": []string{"stun:a", "turn:b"}})
	if len(urls) != 1 || urls[0] != "stun:a" {
		t.Fatalf("parseICEURLs(strings) = %v, want [stun:a]", urls)
	}
}

func TestParseICEServer(t *testing.T) {
	if _, ok := parseICEServer("bad"); ok {
		t.Fatal("parseICEServer() accepted non-map")
	}
	if _, ok := parseICEServer(map[string]interface{}{"urls": []interface{}{"turn:x"}}); ok {
		t.Fatal("parseICEServer() accepted TURN-only server")
	}

	ice, ok := parseICEServer(map[string]interface{}{
		"urls":       []interface{}{"stun:a", "turn:b"},
		"username":   "user",
		"credential": "pass",
	})
	if !ok {
		t.Fatal("parseICEServer() ok = false")
	}
	if len(ice.URLs) != 1 || ice.URLs[0] != "stun:a" || ice.Username != "user" || ice.Credential != "pass" {
		t.Fatalf("parseICEServer() = %+v", ice)
	}
}

func TestConferenceEndParsing(t *testing.T) {
	for _, msg := range []map[string]interface{}{
		{"conferenceClosed": true},
		{"conference": map[string]interface{}{"state": "ENDED"}}, //nolint:goconst // test literal, repetition is intentional
		{"conferenceState": map[string]interface{}{"state": "terminated"}},
	} {
		if !isConferenceEndMessage(msg) {
			t.Fatalf("isConferenceEndMessage(%v) = false", msg)
		}
	}
	if isConferenceEndMessage(map[string]interface{}{"conference": map[string]interface{}{"state": "open"}}) {
		t.Fatal("isConferenceEndMessage() accepted active conference")
	}

	for _, state := range []string{"closed", "ended", "finished", "terminated"} {
		if !isEndedState(state) {
			t.Fatalf("isEndedState(%q) = false", state)
		}
	}
	if isEndedState("active") {
		t.Fatal("isEndedState(active) = true")
	}
}

//nolint:cyclop // table-driven test naturally has many branches
func TestPeerSmallStateHelpers(t *testing.T) {
	p := &Peer{
		reconnectCh: make(chan struct{}, 1),
		closeCh:     make(chan struct{}),
		sendQueue:   make(chan []byte, 2),
		ackWaiters:  make(map[string]chan struct{}),
	}
	p.SetEndedCallback(func(string) {})
	if p.onEnded == nil {
		t.Fatal("SetEndedCallback() did not store callback")
	}
	p.SetReconnectCallback(func(*webrtc.DataChannel) {})
	if p.onReconnect == nil {
		t.Fatal("SetReconnectCallback() did not store callback")
	}
	p.SetShouldReconnect(func() bool { return true })
	if p.shouldReconnect == nil || !p.shouldReconnect() {
		t.Fatal("SetShouldReconnect() did not store callback")
	}

	p.subscriberReady.Store(true)
	if !p.CanSend() {
		t.Fatal("CanSend() = false for subscriber-only ready peer")
	}
	p.closed.Store(true)
	if p.CanSend() {
		t.Fatal("CanSend() = true for closed peer")
	}

	ch := p.registerAckWaiter("uid-1")
	p.resolveAck("uid-1")
	select {
	case <-ch:
	default:
		t.Fatal("resolveAck() did not close waiter")
	}
	if p.waitForAck("", make(chan struct{}), time.Millisecond) {
		t.Fatal("waitForAck(empty uid) = true")
	}

	ch = p.registerAckWaiter("uid-2")
	go p.resolveAck("uid-2")
	if !p.waitForAck("uid-2", ch, time.Second) {
		t.Fatal("waitForAck() = false after resolveAck")
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
}

func TestTelemetryCfgParsing(t *testing.T) {
	if _, _, ok := parseTelemetryCfg(map[string]interface{}{}); ok {
		t.Fatal("parseTelemetryCfg() accepted missing config")
	}
	if _, _, ok := parseTelemetryCfg(map[string]interface{}{
		"telemetryConfiguration": map[string]interface{}{}, //nolint:goconst // test literal, repetition is intentional
	}); ok {
		t.Fatal("parseTelemetryCfg() accepted missing endpoint")
	}

	endpoint, interval, ok := parseTelemetryCfg(map[string]interface{}{
		"telemetryConfiguration": map[string]interface{}{
			"endpoint":        "https://example.test/log",
			"sendingInterval": float64(250),
		},
	})
	if !ok || endpoint != "https://example.test/log" || interval != 250*time.Millisecond {
		t.Fatalf("parseTelemetryCfg() = (%q, %v, %v)", endpoint, interval, ok)
	}

	endpoint, interval, ok = parseTelemetryCfg(map[string]interface{}{
		"telemetryConfiguration": map[string]interface{}{
			"url": "https://example.test/url",
		},
	})
	if !ok || endpoint != "https://example.test/url" || interval != defaultTelemetryInterval {
		t.Fatalf("parseTelemetryCfg(default) = (%q, %v, %v)", endpoint, interval, ok)
	}
}
