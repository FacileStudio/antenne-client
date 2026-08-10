package antenne

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

// A reconnect that the client will retry must be distinguishable from one it
// has given up on, because the two deserve different log levels and a consumer
// can only tell them apart if the library says so.
func TestTransientErrorIsIdentifiableAndUnwraps(t *testing.T) {
	cause := errors.New("registration failed: 404")
	var err error = &TransientError{Attempt: 2, Err: cause}

	var transient *TransientError
	if !errors.As(err, &transient) {
		t.Fatal("a retried failure is not identifiable as transient")
	}
	if transient.Attempt != 2 {
		t.Fatalf("Attempt = %d, want 2", transient.Attempt)
	}
	if !errors.Is(err, cause) {
		t.Fatal("the cause is not reachable through the wrapper")
	}
	if !strings.Contains(err.Error(), "retrying") {
		t.Fatalf("message %q does not say it is retrying", err.Error())
	}

	// The terminal failure must NOT be transient: it is the one worth waking
	// somebody for.
	var terminal error = errors.New("max reconnection attempts reached")
	if errors.As(terminal, &transient) {
		t.Fatal("the terminal error is marked transient")
	}
}
