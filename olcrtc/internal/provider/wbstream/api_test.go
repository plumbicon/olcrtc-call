package wbstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withWBAPIServer(t *testing.T, h http.Handler) {
	t.Helper()
	old := apiBase
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		apiBase = old
		srv.Close()
	})
	apiBase = srv.URL
}

func TestWBStreamAPIHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/api/v1/auth/user/guest-register", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(guestRegisterResponse{AccessToken: "access"}) //nolint:goconst,gosec,lll // test literal; G117 is a false positive for test fixtures
	})
	mux.HandleFunc("POST /api-room/api/v2/room", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer access" {
			t.Fatalf("room auth = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createRoomResponse{RoomID: "room"}) //nolint:goconst,lll // test literal, repetition is intentional
	})
	mux.HandleFunc("POST /api-room/api/v1/room/room/join", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api-room-manager/v2/room/room/connection-details", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("displayName") != "peer" {
			t.Fatalf("displayName query = %q", r.URL.Query().Get("displayName"))
		}
		_ = json.NewEncoder(w).Encode(tokenResponse{RoomToken: "token"}) //nolint:goconst,lll // test literal, repetition is intentional
	})

	withWBAPIServer(t, mux)

	access, err := registerGuest(context.Background(), "peer")
	if err != nil {
		t.Fatalf("registerGuest() error = %v", err)
	}
	if access != "access" {
		t.Fatalf("registerGuest() = %q", access)
	}

	room, err := createRoom(context.Background(), access)
	if err != nil {
		t.Fatalf("createRoom() error = %v", err)
	}
	if room != "room" {
		t.Fatalf("createRoom() = %q", room)
	}

	if err := joinRoom(context.Background(), access, room); err != nil {
		t.Fatalf("joinRoom() error = %v", err)
	}
	token, err := getToken(context.Background(), access, room, "peer")
	if err != nil {
		t.Fatalf("getToken() error = %v", err)
	}
	if token != "token" {
		t.Fatalf("getToken() = %q", token)
	}
}

func TestWBStreamAPIErrors(t *testing.T) {
	withWBAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadGateway)
	}))

	if _, err := registerGuest(context.Background(), "peer"); !errors.Is(err, errGuestRegister) {
		t.Fatalf("registerGuest() error = %v, want %v", err, errGuestRegister)
	}
	if _, err := createRoom(context.Background(), "access"); !errors.Is(err, errCreateRoom) {
		t.Fatalf("createRoom() error = %v, want %v", err, errCreateRoom)
	}
	if err := joinRoom(context.Background(), "access", "room"); !errors.Is(err, errJoinRoom) {
		t.Fatalf("joinRoom() error = %v, want %v", err, errJoinRoom)
	}
	if _, err := getToken(context.Background(), "access", "room", "peer"); !errors.Is(err, errGetToken) {
		t.Fatalf("getToken() error = %v, want %v", err, errGetToken)
	}
}

func TestWBStreamGetRoomToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/api/v1/auth/user/guest-register", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(guestRegisterResponse{AccessToken: "access"}) //nolint:gosec,lll // G117: test-only struct mirroring upstream API shape
	})
	mux.HandleFunc("POST /api-room/api/v2/room", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(createRoomResponse{RoomID: "created"})
	})
	mux.HandleFunc("POST /api-room/api/v1/room/{id}/join", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api-room-manager/v2/room/{id}/connection-details", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResponse{RoomToken: "token"})
	})

	withWBAPIServer(t, mux)

	p, err := NewPeer(context.Background(), "any", "peer", nil)
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}
	token, err := p.getRoomToken(context.Background())
	if err != nil {
		t.Fatalf("getRoomToken() error = %v", err)
	}
	if token != "token" {
		t.Fatalf("getRoomToken() = %q", token)
	}
}
