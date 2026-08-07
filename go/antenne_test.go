package antenne

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func registerCounter(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/antenne/register" {
			hits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"token":"t","app_id":"a"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func testClient(instance string) *Client {
	return NewClient(&Config{App: "Test", Instance: instance, Secret: "s"})
}

// A discarded client must not come back. Before this, Disconnect only flipped a
// flag that an already-sleeping reconnect never re-read, so the client woke up,
// re-registered under the same identity and evicted its own replacement.
func TestDisconnectStopsPendingReconnect(t *testing.T) {
	srv, hits := registerCounter(t)

	c := testClient(srv.URL)
	go c.scheduleReconnect()
	c.Disconnect()

	time.Sleep(1500 * time.Millisecond)

	if got := hits.Load(); got != 0 {
		t.Fatalf("disconnected client re-registered %d time(s), want 0", got)
	}
}

func TestPendingReconnectRegistersWhenLive(t *testing.T) {
	srv, hits := registerCounter(t)

	c := testClient(srv.URL)
	go c.scheduleReconnect()
	defer c.Disconnect()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if hits.Load() > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("live client never attempted to re-register")
}

func TestConnectAfterDisconnectIsRefused(t *testing.T) {
	srv, hits := registerCounter(t)

	c := testClient(srv.URL)
	c.Disconnect()

	if err := c.Connect(t.Context()); err != ErrClosed {
		t.Fatalf("Connect on a closed client = %v, want %v", err, ErrClosed)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("closed client registered %d time(s), want 0", got)
	}
}

func TestDisconnectIsIdempotent(t *testing.T) {
	srv, _ := registerCounter(t)

	c := testClient(srv.URL)
	c.Disconnect()
	c.Disconnect()
}
