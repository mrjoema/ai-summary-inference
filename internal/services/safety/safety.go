package safety

import (
	"context"
	"html"
	"regexp"
	"strings"
	"time"
	"unicode"

	"ai-search-service/internal/config"
	"ai-search-service/internal/logger"
	pb "ai-search-service/proto"
)

type SafetyService struct {
	pb.UnimplementedSafetyServiceServer
	config                *config.Config
	dangerousPatterns     []*regexp.Regexp
	inappropriatePatterns []*regexp.Regexp
	sqlPatterns           []*regexp.Regexp
	cmdPatterns           []*regexp.Regexp
}

func NewSafetyService(cfg *config.Config) (*SafetyService, error) {
	service := &SafetyService{
		config: cfg,
	}

	// Compile regex patterns
	service.compileDangerousPatterns()
	service.compileInappropriatePatterns()
	service.compileSQLPatterns()
	service.compileCmdPatterns()

	return service, nil
}

func (s *SafetyService) ValidateInput(ctx context.Context, req *pb.ValidateInputRequest) (*pb.ValidateInputResponse, error) {
	log := logger.GetLogger()

	log.Infof("Validating input from IP: %s", req.ClientIp)

	text := req.Text
	warnings := []string{}

	// Basic validation
	if len(text) == 0 {
		return &pb.ValidateInputResponse{
			IsSafe:        false,
			SanitizedText: "",
			Warnings:      []string{"Empty input"},
		}, nil
	}

	// Length check
	if len(text) > 500 {
		warnings = append(warnings, "Input too long, truncated")
		text = text[:500]
	}

	// Check for dangerous patterns
	for _, pattern := range s.dangerousPatterns {
		if pattern.MatchString(text) {
			return &pb.ValidateInputResponse{
				IsSafe:        false,
				SanitizedText: "",
				Warnings:      []string{"Dangerous pattern detected"},
			}, nil
		}
	}

	// Check for SQL injection
	for _, pattern := range s.sqlPatterns {
		if pattern.MatchString(text) {
			return &pb.ValidateInputResponse{
				IsSafe:        false,
				SanitizedText: "",
				Warnings:      []string{"SQL injection pattern detected"},
			}, nil
		}
	}

	// Check for command injection
	for _, pattern := range s.cmdPatterns {
		if pattern.MatchString(text) {
			return &pb.ValidateInputResponse{
				IsSafe:        false,
				SanitizedText: "",
				Warnings:      []string{"Command injection pattern detected"},
			}, nil
		}
	}

	// Check for inappropriate content
	for _, pattern := range s.inappropriatePatterns {
		if pattern.MatchString(text) {
			if req.SafeSearch {
				return &pb.ValidateInputResponse{
					IsSafe:        false,
					SanitizedText: "",
					Warnings:      []string{"Inappropriate content detected and blocked by safe search"},
				}, nil
			}
			warnings = append(warnings, "Potentially inappropriate content detected")
			break
		}
	}

	// Sanitize the text
	sanitizedText := s.sanitizeText(text)

	log.Infof("Input validation complete. Safe: %t, Warnings: %d", true, len(warnings))

	return &pb.ValidateInputResponse{
		IsSafe:        true,
		SanitizedText: sanitizedText,
		Warnings:      warnings,
	}, nil
}

func (s *SafetyService) SanitizeOutput(ctx context.Context, req *pb.SanitizeOutputRequest) (*pb.SanitizeOutputResponse, error) {
	log := logger.GetLogger()

	log.Infof("Sanitizing output text of length: %d", len(req.Text))

	text := req.Text
	warnings := []string{}

	// Length check
	if len(text) > 1000 {
		warnings = append(warnings, "Output too long, truncated")
		text = text[:1000] + "..."
	}

	// Sanitize the text
	sanitizedText := s.sanitizeText(text)

	// Remove any remaining dangerous patterns
	for _, pattern := range s.dangerousPatterns {
		if pattern.MatchString(sanitizedText) {
			sanitizedText = pattern.ReplaceAllString(sanitizedText, "[FILTERED]")
			warnings = append(warnings, "Dangerous content filtered")
		}
	}

	// Filter inappropriate content from AI output
	for _, pattern := range s.inappropriatePatterns {
		if pattern.MatchString(sanitizedText) {
			sanitizedText = pattern.ReplaceAllString(sanitizedText, "[CONTENT FILTERED]")
			warnings = append(warnings, "Inappropriate content filtered from AI output")
		}
	}

	log.Infof("Output sanitization complete. Warnings: %d", len(warnings))

	return &pb.SanitizeOutputResponse{
		SanitizedText: sanitizedText,
		Warnings:      warnings,
	}, nil
}

func (s *SafetyService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status:    "healthy",
		Service:   "safety",
		Timestamp: time.Now().Unix(),
	}, nil
}

func (s *SafetyService) sanitizeText(text string) string {
	// Normalize unicode
	text = strings.ToValidUTF8(text, "")

	// Remove control characters
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, text)

	// Don't HTML escape for AI summaries - this causes &#34; and &#39; issues
	// Only escape if the text contains actual HTML tags
	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		text = html.EscapeString(text)
	}

	// Normalize whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	return text
}

func (s *SafetyService) compileDangerousPatterns() {
	patterns := []string{
		`<script[^>]*>.*?</script>`,
		`javascript:`,
		`on\w+\s*=`,
		`<iframe[^>]*>.*?</iframe>`,
		`<object[^>]*>.*?</object>`,
		`<embed[^>]*>.*?</embed>`,
		`<link[^>]*>`,
		`<meta[^>]*>`,
		`<form[^>]*>.*?</form>`,
		`<input[^>]*>`,
		`<textarea[^>]*>.*?</textarea>`,
		`<button[^>]*>.*?</button>`,
	}

	s.dangerousPatterns = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		s.dangerousPatterns[i] = regexp.MustCompile(`(?i)` + pattern)
	}
}

func (s *SafetyService) compileInappropriatePatterns() {
	patterns := []string{
		`\b(hack|crack|exploit|malware|virus|trojan)\b`,
		`\b(illegal|piracy|torrents?)\b`,
		`\b(drugs?|cocaine|heroin|marijuana)\b`,
		`\b(adult|porn|sex|xxx)\b`,
		`\b(violence|kill|murder|bomb)\b`,
		`\b(fuck|shit|damn|bitch|ass|crap)\b`,
		`\b(wtf|what the fuck|fucking|fucked)\b`,
		`\b(hell|goddamn|jesus christ)\b`,
		`\b(stupid|idiot|moron|retard)\b`,
		`\b(hate|racist|nazi|terrorist)\b`,
	}

	s.inappropriatePatterns = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		s.inappropriatePatterns[i] = regexp.MustCompile(`(?i)` + pattern)
	}
}

func (s *SafetyService) compileSQLPatterns() {
	patterns := []string{
		`\b(union|select|insert|delete|update|drop|create|alter|exec|execute)\b`,
		`[\'";]`,
		`--`,
		`/\*.*?\*/`,
	}

	s.sqlPatterns = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		s.sqlPatterns[i] = regexp.MustCompile(`(?i)` + pattern)
	}
}

func (s *SafetyService) compileCmdPatterns() {
	patterns := []string{
		`[;&|` + "`" + `$]`,
		`\b(cat|ls|rm|mv|cp|chmod|chown|sudo|su|wget|curl|nc|netcat)\b`,
	}

	s.cmdPatterns = make([]*regexp.Regexp, len(patterns))
	for i, pattern := range patterns {
		s.cmdPatterns[i] = regexp.MustCompile(`(?i)` + pattern)
	}
}
