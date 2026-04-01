package fakespiffeapi

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Config holds the responses the fake server should return.
type Config struct {
	X509Response *workload.X509SVIDResponse
	JWTResponse  *workload.JWTSVIDResponse
}

// Start creates a fake SPIFFE Workload API gRPC server listening on a Unix
// domain socket. It returns the address in `unix://<path>` format suitable
// for passing to workloadapi.WithAddr. The server is automatically stopped
// when the test completes.
func Start(t *testing.T, cfg Config) string {
	t.Helper()

	// Use a short temp dir path to stay within Unix socket path length
	// limits (~104 bytes on macOS).
	sockDir, err := os.MkdirTemp("", "ws")
	if err != nil {
		t.Fatalf("creating temp dir for socket: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(sockDir) })

	socketPath := filepath.Join(sockDir, "w.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listening on unix socket: %v", err)
	}

	srv := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(srv, &server{
		x509Response: cfg.X509Response,
		jwtResponse:  cfg.JWTResponse,
	})

	go func() {
		_ = srv.Serve(listener)
	}()
	t.Cleanup(func() {
		srv.Stop()
	})

	return fmt.Sprintf("unix://%s", socketPath)
}

type server struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	x509Response *workload.X509SVIDResponse
	jwtResponse  *workload.JWTSVIDResponse
}

func (s *server) FetchX509SVID(_ *workload.X509SVIDRequest, stream workload.SpiffeWorkloadAPI_FetchX509SVIDServer) error {
	if err := checkHeader(stream.Context()); err != nil {
		return err
	}
	if s.x509Response == nil {
		return status.Error(codes.PermissionDenied, "no identity issued")
	}
	if err := stream.Send(s.x509Response); err != nil {
		return err
	}
	// Block until the client disconnects - this is the streaming behavior
	// the go-spiffe workloadapi client expects.
	<-stream.Context().Done()
	return stream.Context().Err()
}

func (s *server) FetchJWTSVID(ctx context.Context, _ *workload.JWTSVIDRequest) (*workload.JWTSVIDResponse, error) {
	if err := checkHeader(ctx); err != nil {
		return nil, err
	}
	if s.jwtResponse == nil {
		return nil, status.Error(codes.PermissionDenied, "no identity issued")
	}
	return s.jwtResponse, nil
}

func checkHeader(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.InvalidArgument, "request has no metadata")
	}
	values := md.Get("workload.spiffe.io")
	if len(values) == 0 || values[0] != "true" {
		return status.Error(codes.InvalidArgument, "missing workload.spiffe.io metadata header")
	}
	return nil
}
