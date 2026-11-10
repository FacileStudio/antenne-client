package antenne

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
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

func flakyPool(t *testing.T, refusals int64) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/antenne/register":
			if hits.Add(1) <= refusals {
				http.Error(w, "deploying", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"token":"t","app_id":"a"}`))
		case "/api/antenne/ws":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

type errorLog struct {
	mu   sync.Mutex
	errs []error
}

func (l *errorLog) add(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, err)
}

func (l *errorLog) snapshot() []error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]error(nil), l.errs...)
}

func persistentClient(instance string, log *errorLog) *Client {
	c := NewClient(&Config{App: "Test", Instance: instance, Secret: "s"},
		WithPersistentConnect(), WithOnError(log.add))
	c.backoff = func(int) time.Duration { return time.Millisecond }
	return c
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// An app that boots while Antenne is down or mid-deploy must connect once
// Antenne is back, instead of staying offline until somebody restarts it.
func TestPersistentConnectRetriesTheFirstConnection(t *testing.T) {
	const refusals = 5
	srv, hits := flakyPool(t, refusals)
	log := &errorLog{}
	c := persistentClient(srv.URL, log)
	defer c.Disconnect()

	if err := c.Connect(t.Context()); err != nil {
		t.Fatalf("Connect = %v, want nil while retrying in the background", err)
	}
	eventually(t, "the connection", c.IsConnected)

	if got := hits.Load(); got != refusals+1 {
		t.Fatalf("registered %d time(s), want %d", got, refusals+1)
	}
	errs := log.snapshot()
	if len(errs) != refusals {
		t.Fatalf("reported %d error(s), want %d: %v", len(errs), refusals, errs)
	}
	for i, err := range errs {
		var transient *TransientError
		if !errors.As(err, &transient) {
			t.Fatalf("error %d is not transient: %v", i, err)
		}
		if transient.Attempt != i+1 {
			t.Fatalf("error %d has Attempt %d, want %d", i, transient.Attempt, i+1)
		}
	}
}

// Persistent mode retries past the default budget of 20 and never reports
// the terminal error.
func TestPersistentConnectIsUnlimited(t *testing.T) {
	srv, hits := flakyPool(t, 30)
	log := &errorLog{}
	c := persistentClient(srv.URL, log)
	defer c.Disconnect()

	if err := c.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the connection", c.IsConnected)

	if got := hits.Load(); got != 31 {
		t.Fatalf("registered %d time(s), want 31", got)
	}
	for _, err := range log.snapshot() {
		var transient *TransientError
		if !errors.As(err, &transient) {
			t.Fatalf("persistent client reported a non-transient error: %v", err)
		}
	}
}

// Disconnect is terminal for a client still retrying its first connection
// too: a discarded client must never resurrect.
func TestDisconnectStopsPersistentConnect(t *testing.T) {
	srv, hits := flakyPool(t, math.MaxInt64)
	log := &errorLog{}
	c := persistentClient(srv.URL, log)

	if err := c.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "a few retries", func() bool { return hits.Load() >= 3 })
	c.Disconnect()

	time.Sleep(50 * time.Millisecond)
	settled, reported := hits.Load(), len(log.snapshot())
	time.Sleep(200 * time.Millisecond)

	if got := hits.Load(); got != settled {
		t.Fatalf("disconnected client registered %d more time(s)", got-settled)
	}
	if got := len(log.snapshot()); got != reported {
		t.Fatalf("disconnected client reported %d more error(s)", got-reported)
	}
	if c.IsConnected() {
		t.Fatal("disconnected client is connected")
	}
}

// The default is unchanged: without the option, a failed first connection is
// returned to the caller and not retried.
func TestConnectWithoutPersistenceFailsOnce(t *testing.T) {
	srv, hits := flakyPool(t, math.MaxInt64)
	c := testClient(srv.URL)
	c.backoff = func(int) time.Duration { return time.Millisecond }
	defer c.Disconnect()

	if err := c.Connect(t.Context()); err == nil {
		t.Fatal("Connect succeeded against a refusing pool")
	}
	time.Sleep(100 * time.Millisecond)
	if got := hits.Load(); got != 1 {
		t.Fatalf("registered %d time(s), want 1", got)
	}
}
