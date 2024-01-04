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
	"google.golang.org/protobuf/types/known/timestamppb"

	pingv1 "buf.build/gen/go/karlmutch/buf-ping/protocolbuffers/go/ping/v1"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-stack/stack"
	"github.com/karlmutch/kv"
)

var (
	apiPing            = otel.GetMeterProvider().Meter("bufping/ping")
	apiPingCounter     metric.Int64Counter
	apiSumCounter      metric.Int64Counter
	apiGenerateCounter metric.Int64Counter
	apiCountCounter    metric.Int64Counter
	apiFailCounter     metric.Int64Counter
)

func init() {
	apiPingCounter, _ = apiPing.Int64Counter(
		"pingbuf.api.ping.counter",
		metric.WithDescription("Number of Ping API calls."),
		metric.WithUnit("{call}"),
	)
	apiSumCounter, _ = apiPing.Int64Counter(
		"pingbuf.api.sum.counter",
		metric.WithDescription("Number of Sum API calls."),
		metric.WithUnit("{call}"),
	)
	apiGenerateCounter, _ = apiPing.Int64Counter(
		"pingbuf.api.generate.counter",
		metric.WithDescription("Number of Generate API calls."),
		metric.WithUnit("{call}"),
	)
	apiCountCounter, _ = apiPing.Int64Counter(
		"pingbuf.api.count.counter",
		metric.WithDescription("Number of Count API calls."),
		metric.WithUnit("{call}"),
	)
	apiFailCounter, _ = apiPing.Int64Counter(
		"pingbuf.api.fail.counter",
		metric.WithDescription("Number of HardFail API calls."),
		metric.WithUnit("{call}"),
	)
}

// PingServer is used to encapsulate a Ping Server implementation state using connectrpc receivers
type PingServer struct {
	logger slog.Logger
	total  int32
	sync.Mutex
}

// NewPingServer returns a new PingServer instance
func NewPingServer(logger slog.Logger) *PingServer {
	return &PingServer{logger: logger}
}

// Ping receives a client ping for the server to determine if it's reachable and will return the sum from previous requests
func (server *PingServer) Ping(ctx context.Context, req *connect.Request[pingv1.PingRequest],
) (resp *connect.Response[pingv1.PingResponse], err error) {

	apiPingCounter.Add(ctx, 1)

	respMsg := &pingv1.PingResponse{
		Sum: atomic.LoadInt32(&server.total),
		Timestamp: &timestamppb.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   int32(time.Now().Nanosecond()),
		},
	}
	resp = connect.NewResponse(respMsg)
	return resp, nil
}

// Sum returns the sum of the numbers sent on the SumRequest stream and returns a single total that
// represents the previous sum plus the SumRequest sum
func (server *PingServer) Sum(ctx context.Context, reqStream *connect.ClientStream[pingv1.SumRequest],
) (resp *connect.Response[pingv1.SumResponse], err error) {

	apiSumCounter.Add(ctx, 1)

	for reqStream.Receive() {
		atomic.AddInt32(&server.total, reqStream.Msg().Addition)
	}
	if reqStream.Err() != nil {
		return nil, reqStream.Err()
	}

	resp = connect.NewResponse(&pingv1.SumResponse{
		Sum: atomic.LoadInt32(&server.total),
	})
	return resp, nil
}

// Generate returns a stream of the numbers total -> total+addition up to the given limit specified in the request
func (server *PingServer) Generate(ctx context.Context, req *connect.Request[pingv1.GenerateRequest],
	respStream *connect.ServerStream[pingv1.GenerateResponse]) (err error) {

	apiGenerateCounter.Add(ctx, 1)

	for i := int64(0); i < int64(req.Msg.Addition); i++ {
		atomic.AddInt32(&server.total, 1)
		errGo := respStream.Send(&pingv1.GenerateResponse{
			Progress: atomic.LoadInt32(&server.total),
		})
		if errGo != nil {
			return errGo
		}
	}
	return nil
}

// Count returns a stream of the numbers 1+total -> recieved.addition+total for every received message on the clients stream
func (server *PingServer) Count(ctx context.Context, stream *connect.BidiStream[pingv1.CountRequest, pingv1.CountResponse]) (err error) {
	// The following is an example of extracting the OpenTelemetry span and using it to post events
	span := trace.SpanFromContext(ctx)

	apiCountCounter.Add(ctx, 1)

	for {
		msg, errGo := stream.Receive()
		if errGo != nil {
			if errors.Is(errGo, io.EOF) {
				return nil
			}
			return errGo
		}

		if msg.Addition >= 10 {
			span.AddEvent("counting in bulk, will not be generating individual OTel events")
		}

		for i := int32(0); i != msg.Addition; i++ {
			if msg.Addition < 10 {
				span.AddEvent("counting")
			}

			resp := &pingv1.CountResponse{Sum: atomic.AddInt32(&server.total, 1)}
			if errGo := stream.Send(resp); errGo != nil {
				return errGo
			}
		}
	}

}

// HardFail generates an error and returns it to the client when invoked
func (server *PingServer) HardFail(ctx context.Context, req *connect.Request[pingv1.HardFailRequest],
) (resp *connect.Response[pingv1.HardFailResponse], err error) {

	apiFailCounter.Add(ctx, 1)

	// Use OTel span state to post an error event
	trace.SpanFromContext(ctx).SetStatus(codes.Error, "HardFail invoked")

	resp = connect.NewResponse(&pingv1.HardFailResponse{})
	err = connect.NewError(connect.Code(req.Msg.FailureCode), kv.NewError("intentional failure").With("stack", stack.Trace().TrimRuntime()))
	return resp, err
}
