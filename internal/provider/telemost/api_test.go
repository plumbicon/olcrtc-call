//nolint:all // Test file keeps scenario setup inline.
package telemost

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withTelemostAPIServer(t *testing.T, h http.Handler) {
	t.Helper()
	old := apiBase
	srv := httptest.NewServer(h)
	t.Cleanup(func() {
		apiBase = old
		srv.Close()
	})
	apiBase = srv.URL
}

func TestGetConnectionInfo(t *testing.T) {
	withTelemostAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if !strings.Contains(r.URL.EscapedPath(), "/conferences/room%2Fid/connection") {
			t.Fatalf("path = %q escaped=%q", r.URL.Path, r.URL.EscapedPath())
		}
		if r.URL.Query().Get("display_name") != "peer" {
			t.Fatalf("display_name query = %q", r.URL.Query().Get("display_name"))
		}
		_ = json.NewEncoder(w).Encode(ConnectionInfo{
			RoomID:      "room",
			PeerID:      "peer-id",
			Credentials: "creds",
		})
	}))

	info, err := GetConnectionInfo(context.Background(), "room/id", "peer")
	if err != nil {
		t.Fatalf("GetConnectionInfo() error = %v", err)
	}
	if info.RoomID != "room" || info.PeerID != "peer-id" || info.Credentials != "creds" {
		t.Fatalf("GetConnectionInfo() = %+v", info)
	}
}

func TestGetConnectionInfoErrors(t *testing.T) {
	withTelemostAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad", http.StatusForbidden)
	}))
	if _, err := GetConnectionInfo(context.Background(), "room", "peer"); !errors.Is(err, ErrAPI) {
		t.Fatalf("GetConnectionInfo() error = %v, want %v", err, ErrAPI)
	}

	withTelemostAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("{"))
	}))
	if _, err := GetConnectionInfo(context.Background(), "room", "peer"); err == nil {
		t.Fatal("GetConnectionInfo() unexpectedly accepted bad json")
	}
}

func TestTelemostNewPeerUsesConnectionInfo(t *testing.T) {
	withTelemostAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ConnectionInfo{
			RoomID:      "room",
			PeerID:      "peer-id",
			Credentials: "creds",
		})
	}))

	p, err := NewPeer(context.Background(), "room", "name", nil)
	if err != nil {
		t.Fatalf("NewPeer() error = %v", err)
	}
	if p.roomURL != "room" || p.name != "name" || p.conn.PeerID != "peer-id" || p.sendQueue == nil {
		t.Fatalf("NewPeer() = %+v", p)
	}
}
