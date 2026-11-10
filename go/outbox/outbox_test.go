package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/FacileStudio/antenne-client/go/catalog"
	"github.com/FacileStudio/tronc/migrate"
	"github.com/FacileStudio/tronc/testdb"
	"gorm.io/gorm"
)

var (
	testDB  *sql.DB
	gormDB  *gorm.DB
	dbSetup = testdb.Config{Prefix: "antenne_outbox_test", Migrate: applyMigration}
)

func applyMigration(g *gorm.DB) error {
	sqlDB, err := g.DB()
	if err != nil {
		return err
	}
	files := fstest.MapFS{"00001_antenne_outbox.sql": {Data: []byte(Migration)}}
	return migrate.Run(context.Background(), migrate.Config{DB: sqlDB, FS: files, LockID: 5661})
}

func TestMain(m *testing.M) {
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce("docker run -d --name pg -e POSTGRES_PASSWORD=pg -p 127.0.0.1:5433:5432 postgres:16")
		os.Exit(m.Run())
	}
	var err error
	gormDB, err = testdb.Open(url, dbSetup)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	testDB, err = gormDB.DB()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func fresh(t *testing.T) *sql.DB {
	t.Helper()
	if testDB == nil {
		t.Skip(testdb.SkipReason(""))
	}
	if err := testdb.Truncate(gormDB, dbSetup); err != nil {
		t.Fatal(err)
	}
	return testDB
}

func enqueue(t *testing.T, db *sql.DB, keys ...string) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if err := Enqueue(context.Background(), tx, catalog.TaskCreated, map[string]string{"key": key}, key); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func countUnsent(db *sql.DB) (int, error) {
	var n int
	err := db.QueryRow(`SELECT count(*) FROM antenne_outbox WHERE sent_at IS NULL`).Scan(&n)
	return n, err
}

func unsent(t *testing.T, db *sql.DB) int {
	t.Helper()
	n, err := countUnsent(db)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func drainUntilEmpty(d *Drainer, db *sql.DB) error {
	for {
		if _, err := d.Drain(context.Background()); err != nil {
			return err
		}
		n, err := countUnsent(db)
		if err != nil || n == 0 {
			return err
		}
	}
}

type recorder struct {
	mu     sync.Mutex
	keys   []string
	failOn string
}

func (r *recorder) emit(channel string, payload any) error {
	var body struct{ Key string }
	if err := json.Unmarshal(payload.(json.RawMessage), &body); err != nil {
		return err
	}
	if body.Key == r.failOn {
		return errors.New("bus down")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, body.Key)
	return nil
}

func TestRolledBackEnqueueLeavesNoRow(t *testing.T) {
	db := fresh(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := Enqueue(context.Background(), tx, catalog.TaskCreated, map[string]string{"key": "a"}, "a"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if n := unsent(t, db); n != 0 {
		t.Fatalf("rolled-back enqueue left %d rows", n)
	}
}

func TestDrainSendsInIDOrder(t *testing.T) {
	db := fresh(t)
	enqueue(t, db, "c", "a", "b")
	rec := &recorder{}
	sent, err := NewDrainer(db, rec.emit).Drain(context.Background())
	if err != nil || sent != 3 {
		t.Fatalf("Drain = %d, %v", sent, err)
	}
	if fmt.Sprint(rec.keys) != "[c a b]" {
		t.Fatalf("sent %v, want insertion order [c a b]", rec.keys)
	}
	if n := unsent(t, db); n != 0 {
		t.Fatalf("%d rows still unsent", n)
	}
}

func TestFailingEmitStopsTheBatch(t *testing.T) {
	db := fresh(t)
	enqueue(t, db, "a", "b", "c")
	rec := &recorder{failOn: "b"}
	sent, err := NewDrainer(db, rec.emit).Drain(context.Background())
	if err == nil || sent != 1 {
		t.Fatalf("Drain = %d, %v; want 1 sent and an error", sent, err)
	}
	var stillUnsent []string
	rows, err := db.Query(`SELECT idempotency_key FROM antenne_outbox WHERE sent_at IS NULL ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		stillUnsent = append(stillUnsent, key)
	}
	if fmt.Sprint(stillUnsent) != "[b c]" {
		t.Fatalf("unsent %v, want [b c]", stillUnsent)
	}
}

func TestDuplicateKeyIsRefused(t *testing.T) {
	db := fresh(t)
	enqueue(t, db, "a")
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	err = Enqueue(context.Background(), tx, catalog.TaskCreated, map[string]string{"key": "a"}, "a")
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate Enqueue = %v, want ErrDuplicate", err)
	}
	if _, err := tx.Exec(`SELECT 1`); err != nil {
		t.Fatalf("transaction unusable after duplicate: %v", err)
	}
}

func TestTwoDrainersNeverSendARowTwice(t *testing.T) {
	db := fresh(t)
	keys := make([]string, 200)
	for i := range keys {
		keys[i] = fmt.Sprintf("k%03d", i)
	}
	enqueue(t, db, keys...)
	rec := &recorder{}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := drainUntilEmpty(NewDrainer(db, rec.emit, WithBatchSize(7)), db); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	seen := map[string]int{}
	for _, k := range rec.keys {
		seen[k]++
	}
	if len(rec.keys) != len(keys) || len(seen) != len(keys) {
		t.Fatalf("sent %d events covering %d keys, want %d each", len(rec.keys), len(seen), len(keys))
	}
}

func TestPruneKeepsUnsentAndRecentRows(t *testing.T) {
	db := fresh(t)
	enqueue(t, db, "old", "recent", "pending")
	if _, err := db.Exec(`UPDATE antenne_outbox SET sent_at = now() - interval '8 days' WHERE idempotency_key = 'old'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE antenne_outbox SET sent_at = now() WHERE idempotency_key = 'recent'`); err != nil {
		t.Fatal(err)
	}
	pruned, err := NewDrainer(db, (&recorder{}).emit).Prune(context.Background())
	if err != nil || pruned != 1 {
		t.Fatalf("Prune = %d, %v; want 1", pruned, err)
	}
}
