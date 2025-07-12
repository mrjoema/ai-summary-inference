package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/services/inference"
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
	lis, err := net.Listen("tcp", ":8083")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	s := grpc.NewServer()

	// Initialize inference service
	inferenceService, err := inference.NewInferenceService(cfg)
	if err != nil {
		log.Fatalf("Failed to create inference service: %v", err)
	}

	// Register service
	pb.RegisterInferenceServiceServer(s, inferenceService)

	// Start server in goroutine
	go func() {
		log.Printf("Inference service starting on port 8083")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down inference service...")
	s.GracefulStop()
	log.Println("Inference service shutdown complete")
}
