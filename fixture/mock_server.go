package fixture

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/test/grpc_testing"
)

type PathResponseHandler struct {
	Path            string
	PathHandlerFunc func(rw http.ResponseWriter, r *http.Request)
}

// StartGrpcTargetTestServer starts a gRPC server on the provided port
// It uses the test.proto from grpc-testing: https://github.com/grpc/grpc-go/blob/40a879c23a0dc77234d17e0699d074d5fd151bd0/test/grpc_testing/test.proto
func StartGrpcTargetTestServer(port int) *grpc.Server {
	server := grpc.NewServer()
	grpc_testing.RegisterTestServiceServer(server, &grpc_testing.UnimplementedTestServiceServer{})
	reflection.Register(server)

	uri := ":" + fmt.Sprint(port)
	l, _ := net.Listen("tcp", uri)

	go func() {
		err := server.Serve(l)
		if err != nil {
			log.Fatal("Server failed : ", err)
		}
	}()
	return server
}

// StartHttpTargetTestServer starts a HTTP server on the provided port
// Optionally, it receives a list of handler functions
func StartHttpTargetTestServer(pathHandlers []PathResponseHandler) (*http.Server, int) {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Sleep for half a second to simulate a slow server
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	for _, pathHandler := range pathHandlers {
		http.HandleFunc(pathHandler.Path, pathHandler.PathHandlerFunc)
	}

	server := &http.Server{}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		err = server.Serve(listener)

		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed. Err: %v", err)
		}
	}()
	return server, port
}
