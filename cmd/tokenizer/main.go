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
	logInstance := logger.GetLogger()

	// Initialize tokenizer service
	service, err := tokenizer.NewTokenizerService(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize tokenizer service: %v", err)
	}

	// Setup gRPC server
	lis, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatalf("Failed to listen on port 8082: %v", err)
	}

	server := grpc.NewServer()
	pb.RegisterTokenizerServiceServer(server, service)

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		logInstance.Info("Shutting down tokenizer service...")
		server.GracefulStop()
	}()

	logInstance.Info("Enterprise Tokenizer Service starting on port 8082")
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}