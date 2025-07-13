package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-search-service/internal/config"
	"ai-search-service/internal/gateway"
	"ai-search-service/internal/logger"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	logger.InitLogger(cfg.LogLevel)

	// Initialize Gin router
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Initialize gateway
	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize gateway: %v", err)
	}

	// Setup routes
	setupRoutes(router, gw)

	// Create HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Gateway.Port),
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Gateway server starting on port %d", cfg.Gateway.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gateway server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Gateway server shutdown complete")
}

func setupRoutes(router *gin.Engine, gw *gateway.Gateway) {
	// Health check
	router.GET("/health", gw.HealthCheck)

	// Metrics endpoint
	router.GET("/metrics", gw.Metrics)

	// API routes
	api := router.Group("/api/v1")
	{
		// Single search endpoint (handles both streaming and non-streaming)
		api.POST("/search", gw.Search)  // Non-streaming: JSON body
		api.GET("/search", gw.Search)   // Streaming: query params + Accept: text/event-stream

		// Utility endpoints
		api.POST("/validate", gw.ValidateInput)
	}

	// Serve static files
	router.Static("/static", "./web/static")
	router.LoadHTMLGlob("web/templates/*")
	router.GET("/", gw.Index)
}
