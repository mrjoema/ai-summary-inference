package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Environment string         `mapstructure:"environment"`
	LogLevel    string         `mapstructure:"log_level"`
	Gateway     GatewayConfig  `mapstructure:"gateway"`
	Services    ServicesConfig `mapstructure:"services"`
	VLLM        VLLMConfig     `mapstructure:"vllm"`
	Ollama      OllamaConfig   `mapstructure:"ollama"`
	Redis       RedisConfig    `mapstructure:"redis"`
	Google      GoogleConfig   `mapstructure:"google"`
	LLM         LLMConfig      `mapstructure:"llm"`
}

type GatewayConfig struct {
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type ServicesConfig struct {
	Search    ServiceConfig `mapstructure:"search"`
	Tokenizer ServiceConfig `mapstructure:"tokenizer"`
	Inference ServiceConfig `mapstructure:"inference"`
	Safety    ServiceConfig `mapstructure:"safety"`
	LLM       ServiceConfig `mapstructure:"llm"`
}

type ServiceConfig struct {
	Host    string        `mapstructure:"host"`
	Port    int           `mapstructure:"port"`
	Timeout time.Duration `mapstructure:"timeout"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type GoogleConfig struct {
	APIKey string `mapstructure:"api_key"`
	CX     string `mapstructure:"cx"`
}

type OllamaConfig struct {
	Host        string        `mapstructure:"host"`
	Port        int           `mapstructure:"port"`
	Model       string        `mapstructure:"model"`
	Temperature float64       `mapstructure:"temperature"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type VLLMConfig struct {
	Host        string        `mapstructure:"host"`
	Port        int           `mapstructure:"port"`
	Model       string        `mapstructure:"model"`
	Temperature float64       `mapstructure:"temperature"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type LLMConfig struct {
	MaxWorkers   int `mapstructure:"max_workers"`
	MaxQueueSize int `mapstructure:"max_queue_size"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/ai-search")

	// Set defaults
	setDefaults()

	// Override with environment variables
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Override with environment variables
	overrideWithEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// GetRedisAddress returns the Redis address
func (c *Config) GetRedisAddress() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}


// GetInferenceAddress returns the inference service address
func (c *Config) GetInferenceAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.Inference.Host, c.Services.Inference.Port)
}

// GetTokenizerAddress returns the tokenizer service address
func (c *Config) GetTokenizerAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.Tokenizer.Host, c.Services.Tokenizer.Port)
}

// GetLLMAddress returns the LLM orchestrator service address
func (c *Config) GetLLMAddress() string {
	return fmt.Sprintf("%s:%d", c.Services.LLM.Host, c.Services.LLM.Port)
}

func setDefaults() {
	// Environment
	viper.SetDefault("environment", "development")
	viper.SetDefault("log_level", "info")

	// Gateway
	viper.SetDefault("gateway.port", 8080)
	viper.SetDefault("gateway.timeout", "30s")

	// Services
	viper.SetDefault("services.search.host", "localhost")
	viper.SetDefault("services.search.port", 8081)
	viper.SetDefault("services.search.timeout", "10s")

	viper.SetDefault("services.tokenizer.host", "localhost")
	viper.SetDefault("services.tokenizer.port", 8082)
	viper.SetDefault("services.tokenizer.timeout", "5s")

	viper.SetDefault("services.inference.host", "localhost")
	viper.SetDefault("services.inference.port", 8083)
	viper.SetDefault("services.inference.timeout", "30s")

	viper.SetDefault("services.safety.host", "localhost")
	viper.SetDefault("services.safety.port", 8084)
	viper.SetDefault("services.safety.timeout", "5s")

	viper.SetDefault("services.llm.host", "localhost")
	viper.SetDefault("services.llm.port", 8085)
	viper.SetDefault("services.llm.timeout", "30s")

	// vLLM (Enterprise)
	viper.SetDefault("vllm.host", "localhost")
	viper.SetDefault("vllm.port", 8000)
	viper.SetDefault("vllm.model", "microsoft/DialoGPT-small")
	viper.SetDefault("vllm.temperature", 0.7)
	viper.SetDefault("vllm.max_tokens", 500)
	viper.SetDefault("vllm.timeout", "30s")

	// Ollama (Fallback)
	viper.SetDefault("ollama.host", "localhost")
	viper.SetDefault("ollama.port", 11434)
	viper.SetDefault("ollama.model", "llama3.2:3b")
	viper.SetDefault("ollama.temperature", 0.7)
	viper.SetDefault("ollama.max_tokens", 500)
	viper.SetDefault("ollama.timeout", "30s")

	// Redis
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Google
	viper.SetDefault("google.api_key", "")
	viper.SetDefault("google.cx", "")

	// LLM
	viper.SetDefault("llm.max_workers", 10)
	viper.SetDefault("llm.max_queue_size", 10000)
}

func overrideWithEnv() {
	if val := os.Getenv("ENVIRONMENT"); val != "" {
		viper.Set("environment", val)
	}
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		viper.Set("log_level", val)
	}
	if val := os.Getenv("GATEWAY_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			viper.Set("gateway.port", port)
		}
	}
	if val := os.Getenv("GOOGLE_API_KEY"); val != "" {
		viper.Set("google.api_key", val)
	}
	if val := os.Getenv("GOOGLE_CX"); val != "" {
		viper.Set("google.cx", val)
	}
	if val := os.Getenv("REDIS_HOST"); val != "" {
		viper.Set("redis.host", val)
	}
	if val := os.Getenv("REDIS_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			viper.Set("redis.port", port)
		}
	}
	if val := os.Getenv("REDIS_PASSWORD"); val != "" {
		viper.Set("redis.password", val)
	}
	if val := os.Getenv("VLLM_HOST"); val != "" {
		viper.Set("vllm.host", val)
	}
	if val := os.Getenv("OLLAMA_HOST"); val != "" {
		viper.Set("ollama.host", val)
	}
	if val := os.Getenv("SEARCH_HOST"); val != "" {
		viper.Set("services.search.host", val)
	}
	if val := os.Getenv("TOKENIZER_HOST"); val != "" {
		viper.Set("services.tokenizer.host", val)
	}
	if val := os.Getenv("INFERENCE_HOST"); val != "" {
		viper.Set("services.inference.host", val)
	}
	if val := os.Getenv("SAFETY_HOST"); val != "" {
		viper.Set("services.safety.host", val)
	}
	if val := os.Getenv("LLM_HOST"); val != "" {
		viper.Set("services.llm.host", val)
	}
}
