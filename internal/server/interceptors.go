package server

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryLoggingInterceptor logs method name, duration, and error.
func UnaryLoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Info().
		Str("method", info.FullMethod).
		Dur("duration", time.Since(start)).
		Err(err).
		Msg("unary rpc")
	return resp, err
}

// StreamLoggingInterceptor logs stream opening and closing.
func StreamLoggingInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	start := time.Now()
	err := handler(srv, ss)
	log.Info().
		Str("method", info.FullMethod).
		Dur("duration", time.Since(start)).
		Err(err).
		Msg("stream rpc")
	return err
}

// UnaryRecoveryInterceptor recovers from panics and converts them to an
// Internal gRPC error, avoiding a server crash on handler bugs.
func UnaryRecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("method", info.FullMethod).
				Interface("panic", r).
				Bytes("stack", debug.Stack()).
				Msg("panic recovered")
			err = status.Errorf(codes.Internal, "internal error: %v", r)
		}
	}()
	return handler(ctx, req)
}

// StreamRecoveryInterceptor is the stream counterpart of UnaryRecoveryInterceptor.
func StreamRecoveryInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("method", info.FullMethod).
				Interface("panic", r).
				Bytes("stack", debug.Stack()).
				Msg("panic recovered")
			err = status.Error(codes.Internal, fmt.Sprintf("internal error: %v", r))
		}
	}()
	return handler(srv, ss)
}
