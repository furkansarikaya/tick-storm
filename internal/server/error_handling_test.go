package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

func TestGetStandardErrorMessage(t *testing.T) {
	tests := []struct {
		name            string
		code            pb.ErrorCode
		expectedMessage string
		expectedDetails string
	}{
		{
			name:            "authentication failed",
			code:            pb.ErrorCode_ERROR_CODE_INVALID_AUTH,
			expectedMessage: "Authentication failed",
			expectedDetails: "Invalid username or password provided",
		},
		{
			name:            "auth required",
			code:            pb.ErrorCode_ERROR_CODE_AUTH_REQUIRED,
			expectedMessage: "Authentication required",
			expectedDetails: "AUTH frame must be the first message sent",
		},
		{
			name:            "already authenticated",
			code:            pb.ErrorCode_ERROR_CODE_ALREADY_AUTHENTICATED,
			expectedMessage: "Already authenticated",
			expectedDetails: "Connection has already been authenticated",
		},
		{
			name:            "invalid subscription",
			code:            pb.ErrorCode_ERROR_CODE_INVALID_SUBSCRIPTION,
			expectedMessage: "Invalid subscription request",
			expectedDetails: "Subscription mode or parameters are invalid",
		},
		{
			name:            "already subscribed",
			code:            pb.ErrorCode_ERROR_CODE_ALREADY_SUBSCRIBED,
			expectedMessage: "Already subscribed",
			expectedDetails: "Connection already has an active subscription",
		},
		{
			name:            "not subscribed",
			code:            pb.ErrorCode_ERROR_CODE_NOT_SUBSCRIBED,
			expectedMessage: "Not subscribed",
			expectedDetails: "No active subscription found for this connection",
		},
		{
			name:            "heartbeat timeout",
			code:            pb.ErrorCode_ERROR_CODE_HEARTBEAT_TIMEOUT,
			expectedMessage: "Heartbeat timeout",
			expectedDetails: "Client failed to send heartbeat within configured interval",
		},
		{
			name:            "invalid message",
			code:            pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE,
			expectedMessage: "Invalid message format",
			expectedDetails: "Message could not be parsed or contains invalid data",
		},
		{
			name:            "checksum failed",
			code:            pb.ErrorCode_ERROR_CODE_CHECKSUM_FAILED,
			expectedMessage: "Checksum validation failed",
			expectedDetails: "Frame CRC32C checksum does not match calculated value",
		},
		{
			name:            "protocol version",
			code:            pb.ErrorCode_ERROR_CODE_PROTOCOL_VERSION,
			expectedMessage: "Unsupported protocol version",
			expectedDetails: "Client protocol version is not supported by server",
		},
		{
			name:            "message too large",
			code:            pb.ErrorCode_ERROR_CODE_MESSAGE_TOO_LARGE,
			expectedMessage: "Message too large",
			expectedDetails: "Message size exceeds maximum allowed limit",
		},
		{
			name:            "rate limited",
			code:            pb.ErrorCode_ERROR_CODE_RATE_LIMITED,
			expectedMessage: "Rate limited",
			expectedDetails: "Too many requests sent within the allowed time window",
		},
		{
			name:            "internal error",
			code:            pb.ErrorCode_ERROR_CODE_INTERNAL_ERROR,
			expectedMessage: "Internal server error",
			expectedDetails: "An unexpected error occurred on the server",
		},
		{
			name:            "unknown error code",
			code:            pb.ErrorCode(999),
			expectedMessage: "Unknown error",
			expectedDetails: "An unrecognized error code was encountered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, details := getStandardErrorMessage(tt.code)
			assert.Equal(t, tt.expectedMessage, message)
			assert.Equal(t, tt.expectedDetails, details)
		})
	}
}

// TestErrorCodeCoverage ensures all error codes have standard messages
func TestErrorCodeCoverage(t *testing.T) {
	errorCodes := []pb.ErrorCode{
		pb.ErrorCode_ERROR_CODE_INVALID_AUTH,
		pb.ErrorCode_ERROR_CODE_AUTH_REQUIRED,
		pb.ErrorCode_ERROR_CODE_ALREADY_AUTHENTICATED,
		pb.ErrorCode_ERROR_CODE_INVALID_SUBSCRIPTION,
		pb.ErrorCode_ERROR_CODE_ALREADY_SUBSCRIBED,
		pb.ErrorCode_ERROR_CODE_NOT_SUBSCRIBED,
		pb.ErrorCode_ERROR_CODE_HEARTBEAT_TIMEOUT,
		pb.ErrorCode_ERROR_CODE_INVALID_MESSAGE,
		pb.ErrorCode_ERROR_CODE_CHECKSUM_FAILED,
		pb.ErrorCode_ERROR_CODE_PROTOCOL_VERSION,
		pb.ErrorCode_ERROR_CODE_MESSAGE_TOO_LARGE,
		pb.ErrorCode_ERROR_CODE_RATE_LIMITED,
		pb.ErrorCode_ERROR_CODE_INTERNAL_ERROR,
	}

	for _, code := range errorCodes {
		t.Run(code.String(), func(t *testing.T) {
			message, details := getStandardErrorMessage(code)
			assert.NotEmpty(t, message, "Error code %s should have a message", code.String())
			assert.NotEmpty(t, details, "Error code %s should have details", code.String())
			assert.NotEqual(t, "Unknown error", message, "Error code %s should not return unknown error", code.String())
		})
	}
}
