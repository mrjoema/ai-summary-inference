package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/services/tokenizer"
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
	lis, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	s := grpc.NewServer()

	// Initialize tokenizer service
	tokenizerService, err := tokenizer.NewTokenizerService(cfg)
	if err != nil {
		log.Fatalf("Failed to create tokenizer service: %v", err)
	}

	// Register service
	pb.RegisterTokenizerServiceServer(s, tokenizerService)

	// Start server in goroutine
	go func() {
		log.Printf("Tokenizer service starting on port 8082")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down tokenizer service...")
	s.GracefulStop()
	log.Println("Tokenizer service shutdown complete")
}
