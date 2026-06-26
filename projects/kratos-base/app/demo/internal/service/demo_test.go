package service_test

import (
	"context"
	"testing"
	"time"

	kErrors "github.com/go-kratos/kratos/v2/errors"

	v1 "github.com/z-mate/kratos-base/api/demo/v1"
	"github.com/z-mate/kratos-base/app/demo/internal/biz"
	"github.com/z-mate/kratos-base/app/demo/internal/service"
	"github.com/z-mate/kratos-base/pkg/errs"
)

// fakeGreetRepo is a hand-written stub for biz.GreetRepo.
type fakeGreetRepo struct {
	greet *biz.Greet
	err   error
}

func (f *fakeGreetRepo) Get(_ context.Context, _ int64) (*biz.Greet, error) {
	return f.greet, f.err
}

// fakeHitsRepo is a hand-written stub for biz.HitsRepo. It records the key it was
// asked to increment so service tests can assert the key threaded through the
// real path service.Hits -> biz.Hit -> repo.Incr.
type fakeHitsRepo struct {
	count   int64
	err     error
	lastKey string
}

func (f *fakeHitsRepo) Incr(_ context.Context, key string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.lastKey = key
	f.count++
	return f.count, nil
}

// fakeEventRepo is a hand-written stub for biz.EventRepo.
type fakeEventRepo struct {
	id  string
	err error
}

func (f *fakeEventRepo) Emit(_ context.Context, _ string) (string, error) {
	return f.id, f.err
}

func newSvc(greetRepo biz.GreetRepo, hitsRepo biz.HitsRepo) *service.DemoService {
	return service.NewDemoService(
		biz.NewGreetUsecase(greetRepo),
		biz.NewHitsUsecase(hitsRepo),
		biz.NewEventUsecase(&fakeEventRepo{id: "test-id"}),
	)
}

func newSvcWithEvent(greetRepo biz.GreetRepo, hitsRepo biz.HitsRepo, eventRepo biz.EventRepo) *service.DemoService {
	return service.NewDemoService(
		biz.NewGreetUsecase(greetRepo),
		biz.NewHitsUsecase(hitsRepo),
		biz.NewEventUsecase(eventRepo),
	)
}

func TestDemoService_Ping(t *testing.T) {
	svc := newSvc(&fakeGreetRepo{}, &fakeHitsRepo{})
	resp, err := svc.Ping(context.Background(), &v1.PingRequest{})
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if resp.GetMessage() != "pong" {
		t.Fatalf("Ping.Message = %q, want %q", resp.GetMessage(), "pong")
	}
}

func TestDemoService_GetGreet_Success(t *testing.T) {
	greet := &biz.Greet{ID: 7, Content: "hi there", CreatedAt: time.Now()}
	svc := newSvc(&fakeGreetRepo{greet: greet}, &fakeHitsRepo{})

	resp, err := svc.GetGreet(context.Background(), &v1.GetGreetRequest{Id: 7})
	if err != nil {
		t.Fatalf("GetGreet returned error: %v", err)
	}
	if resp.GetId() != 7 {
		t.Fatalf("GetGreet.Id = %d, want 7", resp.GetId())
	}
	if resp.GetContent() != "hi there" {
		t.Fatalf("GetGreet.Content = %q, want %q", resp.GetContent(), "hi there")
	}
}

func TestDemoService_GetGreet_DBUnavailable(t *testing.T) {
	dbErr := errs.DBUnavailable(nil)
	svc := newSvc(&fakeGreetRepo{err: dbErr}, &fakeHitsRepo{})

	_, err := svc.GetGreet(context.Background(), &v1.GetGreetRequest{Id: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d", ke.Code)
	}
}

func TestDemoService_GetGreet_InvalidID(t *testing.T) {
	svc := newSvc(&fakeGreetRepo{}, &fakeHitsRepo{})

	_, err := svc.GetGreet(context.Background(), &v1.GetGreetRequest{Id: 0})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 400 {
		t.Fatalf("expected HTTP 400, got %d", ke.Code)
	}
}

// TestDemoService_Hits_Success verifies that a successful Hits call returns the
// incremented counter.
func TestDemoService_Hits_Success(t *testing.T) {
	svc := newSvc(&fakeGreetRepo{}, &fakeHitsRepo{})

	resp, err := svc.Hits(context.Background(), &v1.HitsRequest{})
	if err != nil {
		t.Fatalf("Hits returned error: %v", err)
	}
	if resp.GetCount() != 1 {
		t.Fatalf("Hits.Count = %d, want 1", resp.GetCount())
	}
}

// TestDemoService_Hits_CustomKey proves the proto contract end-to-end through the
// real service path: a non-empty req.Key (what HTTP BindQuery fills from ?key=...)
// is threaded service.Hits -> biz.Hit -> repo.Incr verbatim. Mutation guard: the
// original service.Hits ignored req.Key (param was `_`); with that bug lastKey
// would be the default "demo:hits", and this fails.
func TestDemoService_Hits_CustomKey(t *testing.T) {
	hitsRepo := &fakeHitsRepo{}
	svc := service.NewDemoService(
		biz.NewGreetUsecase(&fakeGreetRepo{}),
		biz.NewHitsUsecase(hitsRepo),
		biz.NewEventUsecase(&fakeEventRepo{id: "test-id"}),
	)

	if _, err := svc.Hits(context.Background(), &v1.HitsRequest{Key: "custom:counter"}); err != nil {
		t.Fatalf("Hits(custom key) returned error: %v", err)
	}
	if hitsRepo.lastKey != "custom:counter" {
		t.Fatalf("Hits routed to key %q, want %q", hitsRepo.lastKey, "custom:counter")
	}
}

// TestDemoService_Hits_EmptyKeyUsesDefault proves the empty-key fallback through
// the real service path: an absent ?key (empty req.Key) lands on the default
// counter "demo:hits". Mutation guard: drop the fallback in biz.Hit and lastKey
// becomes "" instead of the default, failing here.
func TestDemoService_Hits_EmptyKeyUsesDefault(t *testing.T) {
	hitsRepo := &fakeHitsRepo{}
	svc := service.NewDemoService(
		biz.NewGreetUsecase(&fakeGreetRepo{}),
		biz.NewHitsUsecase(hitsRepo),
		biz.NewEventUsecase(&fakeEventRepo{id: "test-id"}),
	)

	if _, err := svc.Hits(context.Background(), &v1.HitsRequest{}); err != nil {
		t.Fatalf("Hits(empty key) returned error: %v", err)
	}
	if hitsRepo.lastKey != biz.DefaultHitsKey {
		t.Fatalf("Hits(empty key) routed to %q, want default %q", hitsRepo.lastKey, biz.DefaultHitsKey)
	}
}

// TestDemoService_Hits_InvalidKey proves an illegal key is rejected as HTTP 400
// through the real service path, never reaching the repo.
func TestDemoService_Hits_InvalidKey(t *testing.T) {
	hitsRepo := &fakeHitsRepo{}
	svc := service.NewDemoService(
		biz.NewGreetUsecase(&fakeGreetRepo{}),
		biz.NewHitsUsecase(hitsRepo),
		biz.NewEventUsecase(&fakeEventRepo{id: "test-id"}),
	)

	_, err := svc.Hits(context.Background(), &v1.HitsRequest{Key: "bad key"})
	if err == nil {
		t.Fatal("Hits(invalid key): expected error, got nil")
	}
	if ke := kErrors.FromError(err); ke.Code != 400 {
		t.Fatalf("Hits(invalid key): expected HTTP 400, got %d", ke.Code)
	}
	if hitsRepo.lastKey != "" {
		t.Fatalf("Hits(invalid key): repo called with %q despite rejection", hitsRepo.lastKey)
	}
}

// TestDemoService_Hits_DBUnavailable verifies that a Redis unavailability is
// surfaced as HTTP 503 without panicking.
func TestDemoService_Hits_DBUnavailable(t *testing.T) {
	dbErr := errs.DBUnavailable(nil)
	svc := newSvc(&fakeGreetRepo{}, &fakeHitsRepo{err: dbErr})

	_, err := svc.Hits(context.Background(), &v1.HitsRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d", ke.Code)
	}
}

// TestDemoService_PublishEvent_Success verifies a successful PublishEvent
// returns an id from the event usecase.
func TestDemoService_PublishEvent_Success(t *testing.T) {
	svc := newSvcWithEvent(&fakeGreetRepo{}, &fakeHitsRepo{}, &fakeEventRepo{id: "abc-123"})

	resp, err := svc.PublishEvent(context.Background(), &v1.PublishEventRequest{Payload: "hello"})
	if err != nil {
		t.Fatalf("PublishEvent: unexpected error: %v", err)
	}
	if resp.GetId() != "abc-123" {
		t.Fatalf("Id = %q, want abc-123", resp.GetId())
	}
}

// TestDemoService_PublishEvent_DBUnavailable verifies that a 503 from the
// event usecase is propagated to the caller.
func TestDemoService_PublishEvent_DBUnavailable(t *testing.T) {
	mqErr := errs.DBUnavailable(nil)
	svc := newSvcWithEvent(&fakeGreetRepo{}, &fakeHitsRepo{}, &fakeEventRepo{err: mqErr})

	_, err := svc.PublishEvent(context.Background(), &v1.PublishEventRequest{Payload: "hello"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ke := kErrors.FromError(err)
	if ke.Code != 503 {
		t.Fatalf("expected HTTP 503, got %d", ke.Code)
	}
}
