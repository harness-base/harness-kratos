package data_test

// R2F6: contract tests for GreetRepo.Get against a real (sqlite-backed) ent
// client. The critical contract is that a *missing row* records MarkSuccess on
// the breaker (a business miss must NOT count toward tripping the circuit),
// while a hit also records MarkSuccess and an infrastructure error records
// MarkFailed.

import (
	"context"
	"sync/atomic"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/z-mate/kratos-base/app/demo/internal/data"
	"github.com/z-mate/kratos-base/app/demo/internal/data/ent"
	"github.com/z-mate/kratos-base/app/demo/internal/data/ent/enttest"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// recordingBreaker counts MarkSuccess / MarkFailed calls so tests can assert the
// breaker contract. Allow always permits the call (we are testing the post-call
// marking, not the open-circuit fast-fail path).
type recordingBreaker struct {
	success atomic.Int64
	failed  atomic.Int64
}

func (b *recordingBreaker) Allow() error { return nil }
func (b *recordingBreaker) MarkSuccess() { b.success.Add(1) }
func (b *recordingBreaker) MarkFailed()  { b.failed.Add(1) }

// newSqliteData returns a *Data backed by a fresh in-memory sqlite ent client
// plus the client for direct seeding. Each test gets an isolated database via a
// unique DSN name so cache=shared connections don't bleed across tests.
func newSqliteData(t *testing.T, name string) (*data.Data, *ent.Client) {
	t.Helper()
	client := enttest.Open(t, "sqlite3",
		"file:greet_"+name+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })
	return data.NewWithEntClient(client), client
}

// TestGreetRepo_Get_Hit verifies a present row is returned with correct fields
// and the breaker is marked successful.
func TestGreetRepo_Get_Hit(t *testing.T) {
	d, client := newSqliteData(t, "hit")
	want, err := client.Greet.Create().SetContent("hello world").Save(context.Background())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	br := &recordingBreaker{}
	repo := data.NewGreetRepoWithBreaker(d, br)

	g, err := repo.Get(context.Background(), int64(want.ID))
	if err != nil {
		t.Fatalf("Get(existing): unexpected error: %v", err)
	}
	if g.ID != int64(want.ID) {
		t.Errorf("ID = %d, want %d", g.ID, want.ID)
	}
	if g.Content != "hello world" {
		t.Errorf("Content = %q, want %q", g.Content, "hello world")
	}
	if !g.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", g.CreatedAt, want.CreatedAt)
	}
	if got := br.success.Load(); got != 1 {
		t.Errorf("MarkSuccess called %d times, want 1 on hit", got)
	}
	if got := br.failed.Load(); got != 0 {
		t.Errorf("MarkFailed called %d times, want 0 on hit", got)
	}
}

// TestGreetRepo_Get_Miss verifies the critical contract: a missing row returns
// errs.NotFound AND records MarkSuccess (NOT MarkFailed) — a business miss must
// never contribute to tripping the circuit breaker.
func TestGreetRepo_Get_Miss(t *testing.T) {
	d, _ := newSqliteData(t, "miss")

	br := &recordingBreaker{}
	repo := data.NewGreetRepoWithBreaker(d, br)

	_, err := repo.Get(context.Background(), 999999)
	if err == nil {
		t.Fatal("Get(absent): expected error, got nil")
	}
	if !errs.IsNotFound(err) {
		t.Fatalf("Get(absent): expected errs.NotFound, got %T: %v", err, err)
	}
	if ke := errs.FromError(err); ke.Code != 404 {
		t.Fatalf("Get(absent): expected HTTP 404, got %d", ke.Code)
	}
	// The breaker contract: a miss is success, not failure.
	if got := br.success.Load(); got != 1 {
		t.Errorf("MarkSuccess called %d times, want 1 on miss (must not trip breaker)", got)
	}
	if got := br.failed.Load(); got != 0 {
		t.Errorf("MarkFailed called %d times, want 0 on miss (would cause false breaker trip)", got)
	}
}

// TestGreetRepo_Get_DBError verifies that a non-NotFound DB error (here: the ent
// client is closed, so the query fails at the driver level) returns
// errs.DBUnavailable and records MarkFailed.
func TestGreetRepo_Get_DBError(t *testing.T) {
	d, client := newSqliteData(t, "dberr")
	// Close the client so subsequent queries fail with a driver error rather
	// than a NotFound — this drives the infrastructure-error branch.
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	br := &recordingBreaker{}
	repo := data.NewGreetRepoWithBreaker(d, br)

	_, err := repo.Get(context.Background(), 1)
	if err == nil {
		t.Fatal("Get(closed client): expected error, got nil")
	}
	if errs.IsNotFound(err) {
		t.Fatalf("Get(closed client): a driver error must not be reported as NotFound: %v", err)
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("Get(closed client): expected errs.DBUnavailable, got %T: %v", err, err)
	}
	if got := br.failed.Load(); got != 1 {
		t.Errorf("MarkFailed called %d times, want 1 on infra error", got)
	}
	if got := br.success.Load(); got != 0 {
		t.Errorf("MarkSuccess called %d times, want 0 on infra error", got)
	}
}

// TestGreetRepo_Get_CallerCanceled verifies the R10F4 fix: when the query fails
// because the CALLER cancelled ctx (client disconnect), Get still returns
// errs.DBUnavailable but must NOT trip the breaker — neither MarkFailed nor
// MarkSuccess — so a wave of client disconnects cannot open the circuit against a
// healthy DB. It is the mutation-self-proving partner of
// TestGreetRepo_Get_DBError (a genuine infra error DOES MarkFailed once): if the
// caller-cancel guard were removed, failed here would become 1 and the test
// fails.
func TestGreetRepo_Get_CallerCanceled(t *testing.T) {
	d, client := newSqliteData(t, "cancel")
	// Seed a row so that, absent cancellation, this would be a hit (MarkSuccess) —
	// proving the cancellation path is what suppresses BOTH marks, not a miss.
	want, err := client.Greet.Create().SetContent("x").Save(context.Background())
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	br := &recordingBreaker{}
	repo := data.NewGreetRepoWithBreaker(d, br)

	// Already-cancelled caller ctx simulates a client that disconnected mid-request.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = repo.Get(ctx, int64(want.ID))
	if err == nil {
		t.Fatal("Get(cancelled ctx): expected error, got nil")
	}
	// A caller cancellation must not be misread as a missing row.
	if errs.IsNotFound(err) {
		t.Fatalf("Get(cancelled ctx): cancellation must not be reported as NotFound: %v", err)
	}
	if !errs.IsDBUnavailable(err) {
		t.Fatalf("Get(cancelled ctx): expected errs.DBUnavailable, got %T: %v", err, err)
	}
	// The crux: a caller cancellation is NOT a DB fault.
	if got := br.failed.Load(); got != 0 {
		t.Errorf("caller cancellation must NOT MarkFailed, got %d", got)
	}
	if got := br.success.Load(); got != 0 {
		t.Errorf("caller cancellation must NOT MarkSuccess, got %d", got)
	}
}
