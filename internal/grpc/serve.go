package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	pb "github.com/ayushpramanik/gpu-cluster-scheduler/internal/grpc/clusterpb"
)

// Serve starts the NodeService gRPC server on addr and blocks until ctx is
// cancelled, then stops gracefully.
func Serve(ctx context.Context, addr string, srv *NodeServer, log *slog.Logger) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterNodeServiceServer(grpcServer, srv)

	errCh := make(chan error, 1)
	go func() {
		log.Info("grpc server listening", "addr", addr)
		if err := grpcServer.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("grpc server shutting down")
		grpcServer.GracefulStop()
		return nil
	}
}
