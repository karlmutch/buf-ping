package ping

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/go-stack/stack"
	"github.com/karlmutch/kv"
	"google.golang.org/protobuf/types/known/timestamppb"

	pingv1 "buf.build/gen/go/karlmutch/buf-ping/protocolbuffers/go/ping/v1"
)

// PingServer is used to encapsulate a Ping Server implementation state using connectrpc receivers
type PingServer struct {
	logger slog.Logger
	total  *atomic.Int32
	sync.Mutex
}

// NewPingServer returns a new PingServer instance
func NewPingServer(logger slog.Logger) *PingServer {
	return &PingServer{logger: logger}
}

// Ping receives a client ping for the server to determine if it's reachable and will return the sum from previous requests
func (server *PingServer) Ping(ctx context.Context, req *connect.Request[pingv1.PingRequest],
) (resp *connect.Response[pingv1.PingResponse], err error) {

	respMsg := &pingv1.PingResponse{
		Sum: server.total.Load(),
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   int32(time.Now().UnixNano()),
		},
	}
	resp = connect.NewResponse(respMsg)
	return resp, nil
}

// Sum returns the sum of the numbers sent on the SumRequest stream and returns a single total that
// represents the previous sum plus the SumRequest sum
func (server *PingServer) Sum(ctx context.Context, reqStream *connect.ClientStream[pingv1.SumRequest],
) (resp *connect.Response[pingv1.SumResponse], err error) {

	for reqStream.Receive() {
		server.total.Add(reqStream.Msg().Addition)
	}
	if reqStream.Err() != nil {
		return nil, reqStream.Err()
	}

	resp = connect.NewResponse(&pingv1.SumResponse{
		Sum: server.total.Load(),
	})
	return resp, nil
}

// Generate returns a stream of the numbers total -> total+addition up to the given limit specified in the request
func (server *PingServer) Generate(ctx context.Context, req *connect.Request[pingv1.GenerateRequest],
	respStream *connect.ServerStream[pingv1.GenerateResponse]) (err error) {

	for i := int64(0); i < int64(req.Msg.Addition); i++ {
		server.total.Add(1)
		errGo := respStream.Send(&pingv1.GenerateResponse{
			Progress: server.total.Load(),
		})
		if errGo != nil {
			return errGo
		}
	}
	return nil
}

// Count returns a stream of the numbers 1+total -> recieved.addition+total for every received message on the clients stream
func (server *PingServer) Count(ctx context.Context, stream *connect.BidiStream[pingv1.CountRequest, pingv1.CountResponse]) (err error) {
	for {
		msg, errGo := stream.Receive()
		if errGo != nil {
			if errors.Is(errGo, io.EOF) {
				return nil
			}
			return errGo
		}
		for i := int32(0); i != msg.Addition; i++ {
			resp := &pingv1.CountResponse{Sum: server.total.Add(1)}
			if errGo := stream.Send(resp); errGo != nil {
				return errGo
			}
		}
	}

}

func (server *PingServer) HardFailure(ctx context.Context, req *connect.Request[pingv1.HardFailureRequest],
) (resp *connect.Response[pingv1.HardFailureResponse], err error) {
	resp = connect.NewResponse(&pingv1.HardFailureResponse{})
	err = connect.NewError(connect.Code(req.Msg.Code), kv.NewError("intentional failure").With("stack", stack.Trace().TrimRuntime()))
	return resp, err
}
