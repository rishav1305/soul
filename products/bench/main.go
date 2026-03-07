package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	soulv1 "github.com/rishav1305/soul/proto/soul/v1"
	"google.golang.org/grpc"
)

func main() {
	socketPath := flag.String("socket", "", "Path to unix socket for gRPC server")
	flag.Parse()

	if *socketPath == "" {
		log.Fatal("--socket flag is required")
	}

	if err := os.Remove(*socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	lis, err := net.Listen("unix", *socketPath)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", *socketPath, err)
	}
	defer lis.Close()

	srv := grpc.NewServer()
	soulv1.RegisterProductServiceServer(srv, &BenchService{})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Printf("received signal %v, shutting down\n", sig)
		srv.GracefulStop()
		os.Remove(*socketPath)
	}()

	log.Printf("bench gRPC server listening on %s", *socketPath)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("gRPC server error: %v", err)
	}
}
