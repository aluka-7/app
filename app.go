package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/aluka-7/configuration"
	"github.com/aluka-7/grpc"
	"github.com/aluka-7/zipkin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func init() {
	fmt.Println("Loading App Engine")
}

type RpcApp func(s *grpc.Server)

func App(ra RpcApp, systemId string, conf configuration.Configuration) {
	server := OptApp(ra, true, systemId, conf)
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	server.Close(func() {})
}

func Daemon(handle func(), close func()) {
	fmt.Println("Daemon Engine Start Server ...")
	handle()
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	close()
	fmt.Println("Daemon Engine Shutdown Server ...")
}

type rpc struct {
	server *grpc.Server
}

func OptApp(ra RpcApp, monitor bool, systemId string, conf configuration.Configuration) rpc {
	server, config := grpc.Engine(systemId, conf).Server(monitor, "base", "app", systemId)
	if len(config.Tag) > 0 {
		zipkin.Init(systemId, conf, config.Tag)
	}
	// Register the health service.
	grpc_health_v1.RegisterHealthServer(server.Server(), &Health{})
	ra(server)
	go func() {
		isStartReady = true
		appPort := os.Getenv("AppPort")
		if len(appPort) == 0 {
			appPort = config.Addr
		}
		if err := server.Run(appPort); err != nil {
			panic("App Engine Start has error")
		}
	}()
	return rpc{server}
}
func (r rpc) Close(close func()) {
	close()
	fmt.Println("App Engine Shutdown Server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.server.Shutdown(ctx); err != nil {
		fmt.Println("App Engine Shutdown has error")
	}
	fmt.Println("App Engine exiting")
}

// Initially the app is not ready and the isStartReady flag is false.
var isStartReady = false

// Health struct
type Health struct{}

// Check does the health check and changes the status of the server based on weather the app is ready or not.
func (h *Health) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if isStartReady == true {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
	} else if isStartReady == false {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING}, nil
	} else {
		return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_UNKNOWN}, nil
	}
}

// Watch is used by clients to receive updates when the service status changes.
// Watch only dummy implemented just to satisfy the interface.
func (h *Health) Watch(req *grpc_health_v1.HealthCheckRequest, w grpc_health_v1.Health_WatchServer) error {
	return status.Error(codes.Unimplemented, "Watching is not supported")
}
