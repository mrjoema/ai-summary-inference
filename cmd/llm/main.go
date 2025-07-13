package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	"ai-search-service/internal/services/llm"
	pb "ai-search-service/proto"

	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting LLM Orchestrator Service...")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger.InitLogger(cfg.LogLevel)

	// Create listener
	lis, err := net.Listen("tcp", ":8085")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	s := grpc.NewServer()

	// Initialize LLM service
	llmService, err := llm.NewLLMService(cfg)
	if err != nil {
		log.Fatalf("Failed to create LLM service: %v", err)
	}

	// Register service
	pb.RegisterLLMOrchestratorServiceServer(s, llmService)

	// Start server in goroutine
	go func() {
		log.Printf("LLM Orchestrator service starting on port 8085")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan struct{})
	go func() {
		llmService.Stop()
		s.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("LLM Orchestrator Service stopped gracefully")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
	}
}
