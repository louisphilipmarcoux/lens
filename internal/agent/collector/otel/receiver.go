package otel

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	collectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/louispm/lens/internal/common/model"
)

// SpanHandler is called for each batch of received spans.
type SpanHandler func(spans []model.Span)

// Receiver accepts OTLP trace data over gRPC and forwards it as internal spans.
type Receiver struct {
	collectorpb.UnimplementedTraceServiceServer

	addr    string
	handler SpanHandler
	logger  *zap.Logger
	server  *grpc.Server
}

// NewReceiver creates an OTel trace receiver.
func NewReceiver(addr string, handler SpanHandler, logger *zap.Logger) *Receiver {
	return &Receiver{
		addr:    addr,
		handler: handler,
		logger:  logger,
	}
}

// Start begins listening for OTLP trace data.
func (r *Receiver) Start() error {
	lis, err := net.Listen("tcp", r.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", r.addr, err)
	}

	r.server = grpc.NewServer()
	collectorpb.RegisterTraceServiceServer(r.server, r)

	r.logger.Info("OTel gRPC receiver started", zap.String("addr", r.addr))

	go func() {
		if err := r.server.Serve(lis); err != nil {
			r.logger.Error("gRPC server error", zap.Error(err))
		}
	}()

	return nil
}

// Stop gracefully stops the gRPC server.
func (r *Receiver) Stop() {
	if r.server != nil {
		r.server.GracefulStop()
	}
}

// Export implements the OTLP TraceService Export RPC.
func (r *Receiver) Export(ctx context.Context, req *collectorpb.ExportTraceServiceRequest) (*collectorpb.ExportTraceServiceResponse, error) {
	spans := convertSpans(req.GetResourceSpans())
	if len(spans) > 0 {
		r.handler(spans)
		r.logger.Debug("received spans", zap.Int("count", len(spans)))
	}
	return &collectorpb.ExportTraceServiceResponse{}, nil
}

func convertSpans(resourceSpans []*tracepb.ResourceSpans) []model.Span {
	var result []model.Span

	for _, rs := range resourceSpans {
		serviceName := ""
		if rs.GetResource() != nil {
			for _, attr := range rs.GetResource().GetAttributes() {
				if attr.GetKey() == "service.name" {
					serviceName = attr.GetValue().GetStringValue()
				}
			}
		}

		for _, scopeSpans := range rs.GetScopeSpans() {
			for _, s := range scopeSpans.GetSpans() {
				span := model.Span{
					TraceID:   fmt.Sprintf("%x", s.GetTraceId()),
					SpanID:    fmt.Sprintf("%x", s.GetSpanId()),
					ParentID:  fmt.Sprintf("%x", s.GetParentSpanId()),
					Service:   serviceName,
					Operation: s.GetName(),
					StartTime: time.Unix(0, int64(s.GetStartTimeUnixNano())),
					Duration:  time.Duration(s.GetEndTimeUnixNano() - s.GetStartTimeUnixNano()),
					Tags:      make(map[string]string),
				}

				switch s.GetStatus().GetCode() {
				case tracepb.Status_STATUS_CODE_OK:
					span.Status = model.SpanStatusOK
				case tracepb.Status_STATUS_CODE_ERROR:
					span.Status = model.SpanStatusError
				default:
					span.Status = model.SpanStatusUnset
				}

				for _, attr := range s.GetAttributes() {
					span.Tags[attr.GetKey()] = attr.GetValue().GetStringValue()
				}

				for _, event := range s.GetEvents() {
					se := model.SpanEvent{
						Name:       event.GetName(),
						Timestamp:  time.Unix(0, int64(event.GetTimeUnixNano())),
						Attributes: make(map[string]string),
					}
					for _, attr := range event.GetAttributes() {
						se.Attributes[attr.GetKey()] = attr.GetValue().GetStringValue()
					}
					span.Events = append(span.Events, se)
				}

				result = append(result, span)
			}
		}
	}

	return result
}
