package jazz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withJazzAPIServer(t *testing.T, h http.Handler) {
	t.Helper()
	old := apiBase
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		apiBase = old
		srv.Close()
	})
	apiBase = srv.URL
}

func TestCreateMeetingAndPreconnect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /room/create-meeting", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerAuthType) != authTypeAnonymous {
			t.Fatalf("missing auth header: %v", r.Header)
		}
		_ = json.NewEncoder(w).Encode(createResponse{RoomID: "room-1", Password: "pass"}) //nolint:gosec,lll // G117: test-only struct mirroring upstream API shape
	})
	mux.HandleFunc("POST /room/room-1/preconnect", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"connectorUrl": "wss://connector"}) //nolint:goconst,lll // test literal, repetition is intentional
	})

	withJazzAPIServer(t, mux)

	headers := map[string]string{
		headerAuthType: authTypeAnonymous,
		"Content-Type": "application/json",
	}
	created, err := createMeeting(context.Background(), headers)
	if err != nil {
		t.Fatalf("createMeeting() error = %v", err)
	}
	if created.RoomID != "room-1" || created.Password != "pass" {
		t.Fatalf("createMeeting() = %+v", created)
	}

	connector, err := preconnect(context.Background(), "room-1", "pass", headers)
	if err != nil {
		t.Fatalf("preconnect() error = %v", err)
	}
	if connector != "wss://connector" {
		t.Fatalf("preconnect() = %q", connector)
	}
}

func TestCreateRoomAndJoinRoom(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /room/create-meeting", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(createResponse{RoomID: "new-room", Password: "new-pass"}) //nolint:goconst,gosec,lll // test literal; G117 is a false positive for test fixtures
	})
	mux.HandleFunc("POST /room/{id}/preconnect", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"connectorUrl": "wss://connector"})
	})

	withJazzAPIServer(t, mux)

	room, err := createRoom(context.Background())
	if err != nil {
		t.Fatalf("createRoom() error = %v", err)
	}
	if room.RoomID != "new-room" || room.Password != "new-pass" || room.ConnectorURL != "wss://connector" {
		t.Fatalf("createRoom() = %+v", room)
	}

	room, err = joinRoom(context.Background(), "existing", "secret")
	if err != nil {
		t.Fatalf("joinRoom() error = %v", err)
	}
	if room.RoomID != "existing" || room.Password != "secret" || room.ConnectorURL != "wss://connector" {
		t.Fatalf("joinRoom() = %+v", room)
	}
}

func TestJazzAPIErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/room/create-meeting", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusTeapot)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusInternalServerError)
	})

	withJazzAPIServer(t, mux)

	if _, err := createMeeting(context.Background(), nil); !errors.Is(err, errCreateRoomFailed) {
		t.Fatalf("createMeeting() error = %v, want %v", err, errCreateRoomFailed)
	}
	if _, err := preconnect(context.Background(), "room", "pass", nil); !errors.Is(err, errPreconnectFailed) {
		t.Fatalf("preconnect() error = %v, want %v", err, errPreconnectFailed)
	}
}

func TestNewPeerUsesRoomAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /room/create-meeting", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(createResponse{RoomID: "new-room", Password: "new-pass"}) //nolint:gosec,lll // G117: test-only struct mirroring upstream API shape
	})
	mux.HandleFunc("POST /room/{id}/preconnect", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"connectorUrl": "wss://connector"})
	})

	withJazzAPIServer(t, mux)

	created, err := NewPeer(context.Background(), "any", "peer", nil)
	if err != nil {
		t.Fatalf("NewPeer(create) error = %v", err)
	}
	if created.roomInfo.RoomID != "new-room" {
		t.Fatalf("created room = %+v", created.roomInfo)
	}

	joined, err := NewPeer(context.Background(), "existing:secret", "peer", nil)
	if err != nil {
		t.Fatalf("NewPeer(join) error = %v", err)
	}
	if joined.roomInfo.RoomID != "existing" || joined.roomInfo.Password != "secret" {
		t.Fatalf("joined room = %+v", joined.roomInfo)
	}
}
