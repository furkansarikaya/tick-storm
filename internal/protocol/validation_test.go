package protocol

import (
	"strings"
	"testing"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAuthRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.AuthRequest
		wantErr bool
		errType error
	}{
		{
			name: "valid auth request",
			req: &pb.AuthRequest{
				Username: "testuser",
				Password: "testpass",
				ClientId: "client123",
				Version:  "1.0.0",
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "empty username",
			req: &pb.AuthRequest{
				Username: "",
				Password: "testpass",
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "username too long",
			req: &pb.AuthRequest{
				Username: strings.Repeat("a", MaxUsernameLength+1),
				Password: "testpass",
			},
			wantErr: true,
			errType: ErrFieldTooLong,
		},
		{
			name: "invalid username characters",
			req: &pb.AuthRequest{
				Username: "test@user",
				Password: "testpass",
			},
			wantErr: true,
			errType: ErrInvalidFieldValue,
		},
		{
			name: "empty password",
			req: &pb.AuthRequest{
				Username: "testuser",
				Password: "",
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "password too long",
			req: &pb.AuthRequest{
				Username: "testuser",
				Password: strings.Repeat("a", MaxPasswordLength+1),
			},
			wantErr: true,
			errType: ErrFieldTooLong,
		},
		{
			name: "invalid version format",
			req: &pb.AuthRequest{
				Username: "testuser",
				Password: "testpass",
				Version:  "invalid-version",
			},
			wantErr: true,
			errType: ErrInvalidFieldValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAuthRequest(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				if tt.errType != nil {
					assert.ErrorIs(t, validationErr.Err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSubscribeRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.SubscribeRequest
		wantErr bool
		errType error
	}{
		{
			name: "valid subscribe request",
			req: &pb.SubscribeRequest{
				Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
				Symbols:     []string{"AAPL", "GOOGL"},
				StartTimeMs: time.Now().UnixMilli(),
				Metadata:    map[string]string{"key": "value"},
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "unspecified mode",
			req: &pb.SubscribeRequest{
				Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_UNSPECIFIED,
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "too many symbols",
			req: &pb.SubscribeRequest{
				Mode:    pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
				Symbols: make([]string, MaxSymbolsCount+1),
			},
			wantErr: true,
			errType: ErrTooManyEntries,
		},
		{
			name: "invalid symbol format",
			req: &pb.SubscribeRequest{
				Mode:    pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
				Symbols: []string{"invalid@symbol"},
			},
			wantErr: true,
			errType: ErrInvalidFieldValue,
		},
		{
			name: "future timestamp",
			req: &pb.SubscribeRequest{
				Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
				StartTimeMs: time.Now().Add(10 * time.Minute).UnixMilli(),
			},
			wantErr: true,
			errType: ErrInvalidTimestamp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubscribeRequest(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				if tt.errType != nil {
					assert.ErrorIs(t, validationErr.Err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateHeartbeatRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *pb.HeartbeatRequest
		wantErr bool
		errType error
	}{
		{
			name: "valid heartbeat request",
			req: &pb.HeartbeatRequest{
				TimestampMs: time.Now().UnixMilli(),
				Sequence:    1,
			},
			wantErr: false,
		},
		{
			name:    "nil request",
			req:     nil,
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "zero timestamp",
			req: &pb.HeartbeatRequest{
				TimestampMs: 0,
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "old timestamp",
			req: &pb.HeartbeatRequest{
				TimestampMs: time.Now().Add(-25 * time.Hour).UnixMilli(),
			},
			wantErr: true,
			errType: ErrInvalidTimestamp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHeartbeatRequest(tt.req)
			if tt.wantErr {
				require.Error(t, err)
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				if tt.errType != nil {
					assert.ErrorIs(t, validationErr.Err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTick(t *testing.T) {
	tests := []struct {
		name    string
		tick    *pb.Tick
		wantErr bool
		errType error
	}{
		{
			name: "valid tick",
			tick: &pb.Tick{
				Symbol:      "AAPL",
				TimestampMs: time.Now().UnixMilli(),
				Price:       150.50,
				Volume:      1000,
				Bid:         150.45,
				Ask:         150.55,
				BidSize:     100,
				AskSize:     200,
				Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
				Metadata:    map[string]string{"exchange": "NASDAQ"},
			},
			wantErr: false,
		},
		{
			name:    "nil tick",
			tick:    nil,
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "empty symbol",
			tick: &pb.Tick{
				Symbol:      "",
				TimestampMs: time.Now().UnixMilli(),
				Price:       150.50,
				Volume:      1000,
				Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "invalid price range",
			tick: &pb.Tick{
				Symbol:      "AAPL",
				TimestampMs: time.Now().UnixMilli(),
				Price:       -1.0,
				Volume:      1000,
				Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
			},
			wantErr: true,
			errType: ErrInvalidRange,
		},
		{
			name: "negative bid size",
			tick: &pb.Tick{
				Symbol:      "AAPL",
				TimestampMs: time.Now().UnixMilli(),
				Price:       150.50,
				Volume:      1000,
				BidSize:     -1,
				Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
			},
			wantErr: true,
			errType: ErrInvalidRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTick(tt.tick)
			if tt.wantErr {
				require.Error(t, err)
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				if tt.errType != nil {
					assert.ErrorIs(t, validationErr.Err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateDataBatch(t *testing.T) {
	validTick := &pb.Tick{
		Symbol:      "AAPL",
		TimestampMs: time.Now().UnixMilli(),
		Price:       150.50,
		Volume:      1000,
		Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
	}

	tests := []struct {
		name    string
		batch   *pb.DataBatch
		wantErr bool
		errType error
	}{
		{
			name: "valid data batch",
			batch: &pb.DataBatch{
				Ticks:            []*pb.Tick{validTick},
				BatchTimestampMs: time.Now().UnixMilli(),
				BatchSequence:    1,
				IsSnapshot:       false,
			},
			wantErr: false,
		},
		{
			name:    "nil batch",
			batch:   nil,
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "empty ticks",
			batch: &pb.DataBatch{
				Ticks:            []*pb.Tick{},
				BatchTimestampMs: time.Now().UnixMilli(),
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "too many ticks",
			batch: &pb.DataBatch{
				Ticks:            make([]*pb.Tick, MaxTicksPerBatch+1),
				BatchTimestampMs: time.Now().UnixMilli(),
			},
			wantErr: true,
			errType: ErrTooManyEntries,
		},
		{
			name: "zero batch timestamp",
			batch: &pb.DataBatch{
				Ticks:            []*pb.Tick{validTick},
				BatchTimestampMs: 0,
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDataBatch(tt.batch)
			if tt.wantErr {
				require.Error(t, err)
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				if tt.errType != nil {
					assert.ErrorIs(t, validationErr.Err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		resp    *pb.ErrorResponse
		wantErr bool
		errType error
	}{
		{
			name: "valid error response",
			resp: &pb.ErrorResponse{
				Code:        pb.ErrorCode_ERROR_CODE_INVALID_AUTH,
				Message:     "Authentication failed",
				Details:     "Invalid username or password",
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: false,
		},
		{
			name:    "nil response",
			resp:    nil,
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "unspecified error code",
			resp: &pb.ErrorResponse{
				Code:        pb.ErrorCode_ERROR_CODE_UNSPECIFIED,
				Message:     "Error message",
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "empty message",
			resp: &pb.ErrorResponse{
				Code:        pb.ErrorCode_ERROR_CODE_INVALID_AUTH,
				Message:     "",
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: true,
			errType: ErrRequiredField,
		},
		{
			name: "message too long",
			resp: &pb.ErrorResponse{
				Code:        pb.ErrorCode_ERROR_CODE_INVALID_AUTH,
				Message:     strings.Repeat("a", MaxMessageLength+1),
				TimestampMs: time.Now().UnixMilli(),
			},
			wantErr: true,
			errType: ErrFieldTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorResponse(tt.resp)
			if tt.wantErr {
				require.Error(t, err)
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				if tt.errType != nil {
					assert.ErrorIs(t, validationErr.Err, tt.errType)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMessageType(t *testing.T) {
	tests := []struct {
		name    string
		msgType MessageType
		wantErr bool
	}{
		{name: "auth", msgType: MessageTypeAuth, wantErr: false},
		{name: "subscribe", msgType: MessageTypeSubscribe, wantErr: false},
		{name: "heartbeat", msgType: MessageTypeHeartbeat, wantErr: false},
		{name: "data_batch", msgType: MessageTypeDataBatch, wantErr: false},
		{name: "error", msgType: MessageTypeError, wantErr: false},
		{name: "ack", msgType: MessageTypeACK, wantErr: false},
		{name: "pong", msgType: MessageTypePong, wantErr: false},
		{name: "invalid", msgType: MessageType(99), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessageType(tt.msgType)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "string with whitespace",
			input:    "  hello world  ",
			expected: "hello world",
		},
		{
			name:     "string with control characters",
			input:    "hello\x00\x01world",
			expected: "helloworld",
		},
		{
			name:     "string with newlines and tabs",
			input:    "hello\nworld\t",
			expected: "hello\nworld\t",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \t\n   ",
			expected: "\t\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidationErrorFormatting(t *testing.T) {
	err := &ValidationError{
		Field:   "username",
		Message: "username is required",
		Value:   nil,
		Err:     ErrRequiredField,
	}
	
	expected := "validation failed for field 'username': username is required"
	assert.Equal(t, expected, err.Error())
	
	errWithValue := &ValidationError{
		Field:   "username",
		Message: "username too long",
		Value:   100,
		Err:     ErrFieldTooLong,
	}
	
	expectedWithValue := "validation failed for field 'username': username too long (value: 100)"
	assert.Equal(t, expectedWithValue, errWithValue.Error())
}

func BenchmarkValidateAuthRequest(b *testing.B) {
	req := &pb.AuthRequest{
		Username: "testuser",
		Password: "testpass",
		ClientId: "client123",
		Version:  "1.0.0",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateAuthRequest(req)
	}
}

func BenchmarkValidateTick(b *testing.B) {
	tick := &pb.Tick{
		Symbol:      "AAPL",
		TimestampMs: time.Now().UnixMilli(),
		Price:       150.50,
		Volume:      1000,
		Bid:         150.45,
		Ask:         150.55,
		BidSize:     100,
		AskSize:     200,
		Mode:        pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND,
		Metadata:    map[string]string{"exchange": "NASDAQ"},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateTick(tick)
	}
}
