package protocol

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

// Validation constants
const (
	MaxUsernameLength    = 64
	MaxPasswordLength    = 128
	MaxClientIDLength    = 64
	MaxVersionLength     = 32
	MaxSymbolLength      = 16
	MaxSymbolsCount      = 100
	MaxMetadataEntries   = 20
	MaxMetadataKeyLength = 64
	MaxMetadataValLength = 256
	MaxMessageLength     = 512
	MaxDetailsLength     = 1024
	MaxTicksPerBatch     = 1000
	MinPrice             = 0.0001
	MaxPrice             = 1000000.0
	MinVolume            = 0.0
	MaxVolume            = 1000000000.0
	MaxTimestampAge      = 24 * time.Hour // Max age for timestamps
)

var (
	// ErrValidation indicates validation failure
	ErrValidation = errors.New("validation failed")
	
	// Validation error types
	ErrRequiredField     = errors.New("required field missing")
	ErrInvalidFieldValue = errors.New("invalid field value")
	ErrFieldTooLong      = errors.New("field exceeds maximum length")
	ErrInvalidEnum       = errors.New("invalid enum value")
	ErrInvalidTimestamp  = errors.New("invalid timestamp")
	ErrTooManyEntries    = errors.New("too many entries")
	ErrInvalidRange      = errors.New("value out of valid range")
	
	// Regex patterns for validation
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	symbolPattern   = regexp.MustCompile(`^[A-Z0-9._-]+$`)
	versionPattern  = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)
)

// ValidationError represents a validation error with context
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// ValidateAuthRequest validates an authentication request
func ValidateAuthRequest(req *pb.AuthRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil", Err: ErrRequiredField}
	}

	// Username validation
	if strings.TrimSpace(req.Username) == "" {
		return &ValidationError{Field: "username", Message: "username is required", Err: ErrRequiredField}
	}
	if len(req.Username) > MaxUsernameLength {
		return &ValidationError{Field: "username", Message: "username too long", Value: len(req.Username), Err: ErrFieldTooLong}
	}
	if !usernamePattern.MatchString(req.Username) {
		return &ValidationError{Field: "username", Message: "username contains invalid characters", Value: req.Username, Err: ErrInvalidFieldValue}
	}

	// Password validation
	if strings.TrimSpace(req.Password) == "" {
		return &ValidationError{Field: "password", Message: "password is required", Err: ErrRequiredField}
	}
	if len(req.Password) > MaxPasswordLength {
		return &ValidationError{Field: "password", Message: "password too long", Value: len(req.Password), Err: ErrFieldTooLong}
	}

	// Optional client ID validation
	if req.ClientId != "" {
		if len(req.ClientId) > MaxClientIDLength {
			return &ValidationError{Field: "client_id", Message: "client ID too long", Value: len(req.ClientId), Err: ErrFieldTooLong}
		}
	}

	// Optional version validation
	if req.Version != "" {
		if len(req.Version) > MaxVersionLength {
			return &ValidationError{Field: "version", Message: "version too long", Value: len(req.Version), Err: ErrFieldTooLong}
		}
		if !versionPattern.MatchString(req.Version) {
			return &ValidationError{Field: "version", Message: "invalid version format", Value: req.Version, Err: ErrInvalidFieldValue}
		}
	}

	return nil
}

// ValidateSubscribeRequest validates a subscription request
func ValidateSubscribeRequest(req *pb.SubscribeRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil", Err: ErrRequiredField}
	}

	// Mode validation
	if req.Mode == pb.SubscriptionMode_SUBSCRIPTION_MODE_UNSPECIFIED {
		return &ValidationError{Field: "mode", Message: "subscription mode is required", Err: ErrRequiredField}
	}
	if req.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND && req.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE {
		return &ValidationError{Field: "mode", Message: "invalid subscription mode", Value: req.Mode, Err: ErrInvalidEnum}
	}

	// Symbols validation
	if len(req.Symbols) > MaxSymbolsCount {
		return &ValidationError{Field: "symbols", Message: "too many symbols", Value: len(req.Symbols), Err: ErrTooManyEntries}
	}
	for i, symbol := range req.Symbols {
		if strings.TrimSpace(symbol) == "" {
			return &ValidationError{Field: fmt.Sprintf("symbols[%d]", i), Message: "symbol cannot be empty", Err: ErrRequiredField}
		}
		if len(symbol) > MaxSymbolLength {
			return &ValidationError{Field: fmt.Sprintf("symbols[%d]", i), Message: "symbol too long", Value: len(symbol), Err: ErrFieldTooLong}
		}
		if !symbolPattern.MatchString(symbol) {
			return &ValidationError{Field: fmt.Sprintf("symbols[%d]", i), Message: "invalid symbol format", Value: symbol, Err: ErrInvalidFieldValue}
		}
	}

	// Start time validation
	if req.StartTimeMs != 0 {
		if err := validateTimestamp(req.StartTimeMs, "start_time_ms"); err != nil {
			return err
		}
	}

	// Metadata validation
	if err := validateMetadata(req.Metadata, "metadata"); err != nil {
		return err
	}

	return nil
}

// ValidateHeartbeatRequest validates a heartbeat request
func ValidateHeartbeatRequest(req *pb.HeartbeatRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil", Err: ErrRequiredField}
	}

	// Timestamp validation
	if req.TimestampMs == 0 {
		return &ValidationError{Field: "timestamp_ms", Message: "timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(req.TimestampMs, "timestamp_ms"); err != nil {
		return err
	}

	return nil
}

// ValidateDataBatch validates a data batch message
func ValidateDataBatch(batch *pb.DataBatch) error {
	if batch == nil {
		return &ValidationError{Field: "batch", Message: "batch cannot be nil", Err: ErrRequiredField}
	}

	// Ticks validation
	if len(batch.Ticks) == 0 {
		return &ValidationError{Field: "ticks", Message: "batch must contain at least one tick", Err: ErrRequiredField}
	}
	if len(batch.Ticks) > MaxTicksPerBatch {
		return &ValidationError{Field: "ticks", Message: "too many ticks in batch", Value: len(batch.Ticks), Err: ErrTooManyEntries}
	}

	// Validate each tick
	for i, tick := range batch.Ticks {
		if err := ValidateTick(tick); err != nil {
			return &ValidationError{Field: fmt.Sprintf("ticks[%d]", i), Message: err.Error(), Err: err}
		}
	}

	// Batch timestamp validation
	if batch.BatchTimestampMs == 0 {
		return &ValidationError{Field: "batch_timestamp_ms", Message: "batch timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(batch.BatchTimestampMs, "batch_timestamp_ms"); err != nil {
		return err
	}

	return nil
}

// ValidateTick validates a tick message
func ValidateTick(tick *pb.Tick) error {
	if tick == nil {
		return &ValidationError{Field: "tick", Message: "tick cannot be nil", Err: ErrRequiredField}
	}

	// Symbol validation
	if strings.TrimSpace(tick.Symbol) == "" {
		return &ValidationError{Field: "symbol", Message: "symbol is required", Err: ErrRequiredField}
	}
	if len(tick.Symbol) > MaxSymbolLength {
		return &ValidationError{Field: "symbol", Message: "symbol too long", Value: len(tick.Symbol), Err: ErrFieldTooLong}
	}
	if !symbolPattern.MatchString(tick.Symbol) {
		return &ValidationError{Field: "symbol", Message: "invalid symbol format", Value: tick.Symbol, Err: ErrInvalidFieldValue}
	}

	// Timestamp validation
	if tick.TimestampMs == 0 {
		return &ValidationError{Field: "timestamp_ms", Message: "timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(tick.TimestampMs, "timestamp_ms"); err != nil {
		return err
	}

	// Price validation
	if tick.Price < MinPrice || tick.Price > MaxPrice {
		return &ValidationError{Field: "price", Message: "price out of valid range", Value: tick.Price, Err: ErrInvalidRange}
	}

	// Volume validation
	if tick.Volume < MinVolume || tick.Volume > MaxVolume {
		return &ValidationError{Field: "volume", Message: "volume out of valid range", Value: tick.Volume, Err: ErrInvalidRange}
	}

	// Bid/Ask validation
	if tick.Bid != 0 && (tick.Bid < MinPrice || tick.Bid > MaxPrice) {
		return &ValidationError{Field: "bid", Message: "bid price out of valid range", Value: tick.Bid, Err: ErrInvalidRange}
	}
	if tick.Ask != 0 && (tick.Ask < MinPrice || tick.Ask > MaxPrice) {
		return &ValidationError{Field: "ask", Message: "ask price out of valid range", Value: tick.Ask, Err: ErrInvalidRange}
	}

	// Bid/Ask size validation
	if tick.BidSize < 0 {
		return &ValidationError{Field: "bid_size", Message: "bid size cannot be negative", Value: tick.BidSize, Err: ErrInvalidRange}
	}
	if tick.AskSize < 0 {
		return &ValidationError{Field: "ask_size", Message: "ask size cannot be negative", Value: tick.AskSize, Err: ErrInvalidRange}
	}

	// Mode validation
	if tick.Mode == pb.SubscriptionMode_SUBSCRIPTION_MODE_UNSPECIFIED {
		return &ValidationError{Field: "mode", Message: "subscription mode is required", Err: ErrRequiredField}
	}
	if tick.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND && tick.Mode != pb.SubscriptionMode_SUBSCRIPTION_MODE_MINUTE {
		return &ValidationError{Field: "mode", Message: "invalid subscription mode", Value: tick.Mode, Err: ErrInvalidEnum}
	}

	// Metadata validation
	if err := validateMetadata(tick.Metadata, "metadata"); err != nil {
		return err
	}

	return nil
}

// ValidateErrorResponse validates an error response
func ValidateErrorResponse(resp *pb.ErrorResponse) error {
	if resp == nil {
		return &ValidationError{Field: "response", Message: "response cannot be nil", Err: ErrRequiredField}
	}

	// Code validation
	if resp.Code == pb.ErrorCode_ERROR_CODE_UNSPECIFIED {
		return &ValidationError{Field: "code", Message: "error code is required", Err: ErrRequiredField}
	}

	// Message validation
	if strings.TrimSpace(resp.Message) == "" {
		return &ValidationError{Field: "message", Message: "error message is required", Err: ErrRequiredField}
	}
	if len(resp.Message) > MaxMessageLength {
		return &ValidationError{Field: "message", Message: "error message too long", Value: len(resp.Message), Err: ErrFieldTooLong}
	}

	// Details validation
	if len(resp.Details) > MaxDetailsLength {
		return &ValidationError{Field: "details", Message: "error details too long", Value: len(resp.Details), Err: ErrFieldTooLong}
	}

	// Timestamp validation
	if resp.TimestampMs == 0 {
		return &ValidationError{Field: "timestamp_ms", Message: "timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(resp.TimestampMs, "timestamp_ms"); err != nil {
		return err
	}

	return nil
}

// ValidateAckResponse validates an acknowledgment response
func ValidateAckResponse(resp *pb.AckResponse) error {
	if resp == nil {
		return &ValidationError{Field: "response", Message: "response cannot be nil", Err: ErrRequiredField}
	}

	// Ack type validation
	if resp.AckType == pb.MessageType_MESSAGE_TYPE_UNSPECIFIED {
		return &ValidationError{Field: "ack_type", Message: "ack type is required", Err: ErrRequiredField}
	}

	// Message validation (optional)
	if len(resp.Message) > MaxMessageLength {
		return &ValidationError{Field: "message", Message: "message too long", Value: len(resp.Message), Err: ErrFieldTooLong}
	}

	// Timestamp validation
	if resp.TimestampMs == 0 {
		return &ValidationError{Field: "timestamp_ms", Message: "timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(resp.TimestampMs, "timestamp_ms"); err != nil {
		return err
	}

	// Metadata validation
	if err := validateMetadata(resp.Metadata, "metadata"); err != nil {
		return err
	}

	return nil
}

// ValidateHeartbeatResponse validates a heartbeat response
func ValidateHeartbeatResponse(resp *pb.HeartbeatResponse) error {
	if resp == nil {
		return &ValidationError{Field: "response", Message: "response cannot be nil", Err: ErrRequiredField}
	}

	// Client timestamp validation
	if resp.ClientTimestampMs == 0 {
		return &ValidationError{Field: "client_timestamp_ms", Message: "client timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(resp.ClientTimestampMs, "client_timestamp_ms"); err != nil {
		return err
	}

	// Server timestamp validation
	if resp.ServerTimestampMs == 0 {
		return &ValidationError{Field: "server_timestamp_ms", Message: "server timestamp is required", Err: ErrRequiredField}
	}
	if err := validateTimestamp(resp.ServerTimestampMs, "server_timestamp_ms"); err != nil {
		return err
	}

	return nil
}

// Helper function to validate timestamps
func validateTimestamp(timestampMs int64, fieldName string) error {
	if timestampMs <= 0 {
		return &ValidationError{Field: fieldName, Message: "timestamp must be positive", Value: timestampMs, Err: ErrInvalidTimestamp}
	}

	// Check if timestamp is too far in the past or future
	now := time.Now().UnixMilli()
	maxAge := MaxTimestampAge.Milliseconds()
	
	if timestampMs < now-maxAge {
		return &ValidationError{Field: fieldName, Message: "timestamp too old", Value: timestampMs, Err: ErrInvalidTimestamp}
	}
	
	// Allow some future tolerance (5 minutes)
	futureThreshold := 5 * time.Minute.Milliseconds()
	if timestampMs > now+futureThreshold {
		return &ValidationError{Field: fieldName, Message: "timestamp too far in future", Value: timestampMs, Err: ErrInvalidTimestamp}
	}

	return nil
}

// Helper function to validate metadata maps
func validateMetadata(metadata map[string]string, fieldName string) error {
	if len(metadata) > MaxMetadataEntries {
		return &ValidationError{Field: fieldName, Message: "too many metadata entries", Value: len(metadata), Err: ErrTooManyEntries}
	}

	for key, value := range metadata {
		if len(key) > MaxMetadataKeyLength {
			return &ValidationError{Field: fmt.Sprintf("%s[%s]", fieldName, key), Message: "metadata key too long", Value: len(key), Err: ErrFieldTooLong}
		}
		if len(value) > MaxMetadataValLength {
			return &ValidationError{Field: fmt.Sprintf("%s[%s]", fieldName, key), Message: "metadata value too long", Value: len(value), Err: ErrFieldTooLong}
		}
		if strings.TrimSpace(key) == "" {
			return &ValidationError{Field: fmt.Sprintf("%s[%s]", fieldName, key), Message: "metadata key cannot be empty", Err: ErrRequiredField}
		}
	}

	return nil
}

// SanitizeString sanitizes string input by trimming whitespace and removing control characters
func SanitizeString(input string) string {
	// Trim whitespace
	sanitized := strings.TrimSpace(input)
	
	// Remove control characters except newlines and tabs
	var result strings.Builder
	for _, r := range sanitized {
		if r >= 32 || r == '\n' || r == '\t' {
			result.WriteRune(r)
		}
	}
	
	return result.String()
}

// ValidateMessageType validates if a message type is known and supported
func ValidateMessageType(msgType MessageType) error {
	switch msgType {
	case MessageTypeAuth, MessageTypeSubscribe, MessageTypeHeartbeat, 
		 MessageTypeDataBatch, MessageTypeError, MessageTypeACK, MessageTypePong:
		return nil
	default:
		return &ValidationError{Field: "message_type", Message: "unknown message type", Value: msgType, Err: ErrInvalidFieldValue}
	}
}
