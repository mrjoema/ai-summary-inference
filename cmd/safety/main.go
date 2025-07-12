package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/services/safety"
	pb "ai-search-service/proto"

	"google.golang.org/grpc"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger.InitLogger(cfg.LogLevel)

	// Create listener
	lis, err := net.Listen("tcp", ":8084")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	s := grpc.NewServer()

	// Initialize safety service
	safetyService, err := safety.NewSafetyService(cfg)
	if err != nil {
		log.Fatalf("Failed to create safety service: %v", err)
	}

	// Register service
	pb.RegisterSafetyServiceServer(s, safetyService)

	// Start server in goroutine
	go func() {
		log.Printf("Safety service starting on port 8084")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down safety service...")
	s.GracefulStop()
	log.Println("Safety service shutdown complete")
}
