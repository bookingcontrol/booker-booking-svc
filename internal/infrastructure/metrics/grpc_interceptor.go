package metrics

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerMetricsInterceptor returns a gRPC unary server interceptor that collects metrics
func UnaryServerMetricsInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		// Extract method name from full method path (e.g., "/venue.VenueService/CreateVenue" -> "CreateVenue")
		method := extractMethodName(info.FullMethod)

		// Log incoming request
		log.Info().
			Str("service", serviceName).
			Str("method", method).
			Str("full_method", info.FullMethod).
			Msg("gRPC request received")

		resp, err := handler(ctx, req)

		duration := time.Since(start).Seconds()
		statusCode := status.Code(err).String()

		// Log response
		if err != nil {
			log.Error().
				Err(err).
				Str("service", serviceName).
				Str("method", method).
				Str("status", statusCode).
				Float64("duration_seconds", duration).
				Msg("gRPC request failed")
		} else {
			log.Info().
				Str("service", serviceName).
				Str("method", method).
				Str("status", statusCode).
				Float64("duration_seconds", duration).
				Msg("gRPC request completed")
		}

		GRPCServerRequestsTotal.WithLabelValues(method, statusCode, serviceName).Inc()
		GRPCServerRequestDuration.WithLabelValues(method, statusCode, serviceName).Observe(duration)

		return resp, err
	}
}

// extractMethodName extracts the method name from a full gRPC method path
// Example: "/venue.VenueService/CreateVenue" -> "CreateVenue"
func extractMethodName(fullMethod string) string {
	// Full method format: /package.Service/Method
	// We want just the Method part
	for i := len(fullMethod) - 1; i >= 0; i-- {
		if fullMethod[i] == '/' {
			return fullMethod[i+1:]
		}
	}
	return fullMethod
}

