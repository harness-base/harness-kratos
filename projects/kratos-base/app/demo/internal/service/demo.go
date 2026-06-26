// Package service implements the proto-defined DemoService using the biz layer.
package service

import (
	"context"

	v1 "github.com/z-mate/kratos-base/api/demo/v1"
	"github.com/z-mate/kratos-base/app/demo/internal/biz"
)

// DemoService implements v1.DemoServiceServer.
type DemoService struct {
	v1.UnimplementedDemoServiceServer
	uc      *biz.GreetUsecase
	hitsUc  *biz.HitsUsecase
	eventUc *biz.EventUsecase
}

// NewDemoService constructs a DemoService.
func NewDemoService(uc *biz.GreetUsecase, hitsUc *biz.HitsUsecase, eventUc *biz.EventUsecase) *DemoService {
	return &DemoService{uc: uc, hitsUc: hitsUc, eventUc: eventUc}
}

// Ping implements DemoServiceServer.Ping.
func (s *DemoService) Ping(_ context.Context, _ *v1.PingRequest) (*v1.PingResponse, error) {
	return &v1.PingResponse{Message: s.uc.Ping()}, nil
}

// GetGreet implements DemoServiceServer.GetGreet.
func (s *DemoService) GetGreet(ctx context.Context, req *v1.GetGreetRequest) (*v1.Greet, error) {
	g, err := s.uc.Get(ctx, req.GetId())
	if err != nil {
		// errs are already kratos errors – gRPC codec will encode them correctly.
		return nil, err
	}
	return &v1.Greet{
		Id:      g.ID,
		Content: g.Content,
	}, nil
}

// Hits implements DemoServiceServer.Hits.
//
// The optional key (e.g. ?key=foo) is threaded to the usecase, which falls back
// to the default counter key when it is empty — matching the proto contract.
func (s *DemoService) Hits(ctx context.Context, req *v1.HitsRequest) (*v1.HitsReply, error) {
	count, err := s.hitsUc.Hit(ctx, req.GetKey())
	if err != nil {
		return nil, err
	}
	return &v1.HitsReply{Count: count}, nil
}

// PublishEvent implements DemoServiceServer.PublishEvent.
func (s *DemoService) PublishEvent(ctx context.Context, req *v1.PublishEventRequest) (*v1.PublishEventReply, error) {
	id, err := s.eventUc.Publish(ctx, req.GetPayload())
	if err != nil {
		return nil, err
	}
	return &v1.PublishEventReply{Id: id}, nil
}
