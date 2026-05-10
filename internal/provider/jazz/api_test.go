//nolint:all // Test file keeps scenario setup inline.
package jazz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withJazzAPIServer(t *testing.T, h http.Handler) string {
	t.Helper()
	old := apiBase
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		apiBase = old
		srv.Close()
	})
	apiBase = srv.URL
	return srv.URL
}

func TestCreateMeetingAndPreconnect(t *testing.T) {
	withJazzAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerAuthType) != authTypeAnonymous {
			t.Fatalf("missing auth header: %v", r.Header)
		}
		switch r.URL.Path {
		case "/room/create-meeting":
			if r.Method != http.MethodPost {
				t.Fatalf("create method = %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(createResponse{RoomID: "room-1", Password: "pass"})
		case "/room/room-1/preconnect":
			if r.Method != http.MethodPost {
				t.Fatalf("preconnect method = %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"connectorUrl": "wss://connector"})
		default:
			http.NotFound(w, r)
		}
	}))

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
	withJazzAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/room/create-meeting":
			_ = json.NewEncoder(w).Encode(createResponse{RoomID: "new-room", Password: "new-pass"})
		case "/room/new-room/preconnect", "/room/existing/preconnect":
			_ = json.NewEncoder(w).Encode(map[string]string{"connectorUrl": "wss://connector"})
		default:
			http.NotFound(w, r)
		}
	}))

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
	withJazzAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "create-meeting"):
			http.Error(w, "bad", http.StatusTeapot)
		default:
			http.Error(w, "bad", http.StatusInternalServerError)
		}
	}))

	if _, err := createMeeting(context.Background(), nil); !errors.Is(err, errCreateRoomFailed) {
		t.Fatalf("createMeeting() error = %v, want %v", err, errCreateRoomFailed)
	}
	if _, err := preconnect(context.Background(), "room", "pass", nil); !errors.Is(err, errPreconnectFailed) {
		t.Fatalf("preconnect() error = %v, want %v", err, errPreconnectFailed)
	}
}

func TestNewPeerUsesRoomAPI(t *testing.T) {
	withJazzAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/room/create-meeting":
			_ = json.NewEncoder(w).Encode(createResponse{RoomID: "new-room", Password: "new-pass"})
		case "/room/new-room/preconnect", "/room/existing/preconnect":
			_ = json.NewEncoder(w).Encode(map[string]string{"connectorUrl": "wss://connector"})
		default:
			http.NotFound(w, r)
		}
	}))

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
