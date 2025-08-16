package server

import (
	"fmt"
	"time"

	"github.com/furkansarikaya/tick-storm/internal/protocol"
	"github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

// VersionHandler manages protocol version-specific message handling
type VersionHandler struct {
	supportedVersions map[uint8]bool
	metrics          *protocol.VersionMetrics
}

// NewVersionHandler creates a new version handler
func NewVersionHandler() *VersionHandler {
	supportedVersions := make(map[uint8]bool)
	for version := range protocol.SupportedVersions {
		supportedVersions[version] = true
	}
	
	return &VersionHandler{
		supportedVersions: supportedVersions,
		metrics:          protocol.NewVersionMetrics(),
	}
}

// ValidateFrameVersion validates the version field in a frame
func (vh *VersionHandler) ValidateFrameVersion(frame *protocol.Frame) error {
	if frame == nil {
		return fmt.Errorf("frame is nil")
	}
	
	// Record version usage for metrics
	if vh.IsVersionSupported(frame.Version) {
		vh.metrics.RecordVersionUsage(frame.Version)
	} else {
		vh.metrics.RecordUnsupportedVersion()
	}
	
	// Validate version
	return protocol.ValidateVersion(frame.Version)
}

// IsVersionSupported checks if a version is supported
func (vh *VersionHandler) IsVersionSupported(version uint8) bool {
	return vh.supportedVersions[version]
}

// GetVersionCapabilities returns the capabilities for a specific version
func (vh *VersionHandler) GetVersionCapabilities(version uint8) (*protocol.VersionFeatures, error) {
	return protocol.GetVersionFeatures(version)
}

// HandleVersionSpecificMessage processes messages based on protocol version
func (vh *VersionHandler) HandleVersionSpecificMessage(frame *protocol.Frame, handler func(*protocol.Frame) error) error {
	// Validate version first
	if err := vh.ValidateFrameVersion(frame); err != nil {
		return fmt.Errorf("version validation failed: %w", err)
	}
	
	// Get version capabilities
	capabilities, err := vh.GetVersionCapabilities(frame.Version)
	if err != nil {
		return fmt.Errorf("failed to get version capabilities: %w", err)
	}
	
	// Check if message type is supported in this version
	if !vh.isMessageTypeSupported(frame.Type, capabilities) {
		return fmt.Errorf("message type %d not supported in version 0x%02X", frame.Type, frame.Version)
	}
	
	// Process the message
	return handler(frame)
}

// isMessageTypeSupported checks if a message type is supported in a version
func (vh *VersionHandler) isMessageTypeSupported(msgType protocol.MessageType, capabilities *protocol.VersionFeatures) bool {
	switch msgType {
	case protocol.MessageTypeAuth:
		return capabilities.Authentication
	case protocol.MessageTypeSubscribe:
		return capabilities.Subscription
	case protocol.MessageTypeHeartbeat:
		return capabilities.Heartbeat
	case protocol.MessageTypeDataBatch:
		return capabilities.DataBatch
	case protocol.MessageTypeError:
		return capabilities.ErrorReporting
	case protocol.MessageTypeACK:
		return capabilities.ErrorReporting // ACK is part of error reporting
	case protocol.MessageTypePong:
		return capabilities.Heartbeat // Pong is part of heartbeat
	default:
		return false
	}
}

// CreateVersionSpecificErrorResponse creates an error response appropriate for the client version
func (vh *VersionHandler) CreateVersionSpecificErrorResponse(clientVersion uint8, code pb.ErrorCode, message string) (*pb.ErrorResponse, error) {
	capabilities, err := vh.GetVersionCapabilities(clientVersion)
	if err != nil {
		return nil, err
	}
	
	if !capabilities.ErrorReporting {
		return nil, fmt.Errorf("error reporting not supported in version 0x%02X", clientVersion)
	}
	
	// Create error response with version-specific features
	errorResp := &pb.ErrorResponse{
		Code:        code,
		Message:     message,
		TimestampMs: time.Now().UnixMilli(),
	}
	
	// Add version-specific details if supported
	if capabilities.InputValidation {
		// Enhanced error details available in versions with input validation
		errorResp.Details = fmt.Sprintf("Version 0x%02X error: %s", clientVersion, message)
	}
	
	return errorResp, nil
}

// NegotiateVersion performs version negotiation with a client
func (vh *VersionHandler) NegotiateVersion(clientVersion uint8) (uint8, error) {
	return protocol.GetVersionNegotiationResponse(clientVersion)
}

// GetVersionMetrics returns version usage metrics
func (vh *VersionHandler) GetVersionMetrics() map[string]interface{} {
	return vh.metrics.GetStats()
}

// Global version handler instance
var globalVersionHandler = NewVersionHandler()

// GetGlobalVersionHandler returns the global version handler
func GetGlobalVersionHandler() *VersionHandler {
	return globalVersionHandler
}

// VersionAwareConnectionHandler extends ConnectionHandler with version awareness
type VersionAwareConnectionHandler struct {
	*ConnectionHandler
	versionHandler *VersionHandler
	clientVersion  uint8
}

// NewVersionAwareConnectionHandler creates a version-aware connection handler
func NewVersionAwareConnectionHandler(conn *Connection) *VersionAwareConnectionHandler {
	baseHandler := &ConnectionHandler{conn: conn}
	return &VersionAwareConnectionHandler{
		ConnectionHandler: baseHandler,
		versionHandler:   GetGlobalVersionHandler(),
		clientVersion:    protocol.CurrentProtocolVersion, // Default to current version
	}
}

// SetClientVersion sets the negotiated client version
func (vh *VersionAwareConnectionHandler) SetClientVersion(version uint8) {
	vh.clientVersion = version
}

// GetClientVersion returns the current client version
func (vh *VersionAwareConnectionHandler) GetClientVersion() uint8 {
	return vh.clientVersion
}

// ProcessFrameWithVersionCheck processes a frame with version validation
func (vh *VersionAwareConnectionHandler) ProcessFrameWithVersionCheck(frame *protocol.Frame) error {
	// Update client version if this is the first frame
	if vh.clientVersion == protocol.CurrentProtocolVersion && frame.Version != protocol.CurrentProtocolVersion {
		negotiatedVersion, err := vh.versionHandler.NegotiateVersion(frame.Version)
		if err != nil {
			return fmt.Errorf("version negotiation failed: %w", err)
		}
		vh.clientVersion = negotiatedVersion
	}
	
	// Validate frame version
	if err := vh.versionHandler.ValidateFrameVersion(frame); err != nil {
		return err
	}
	
	// Check version compatibility
	if !protocol.IsVersionCompatible(protocol.CurrentProtocolVersion, frame.Version) {
		return fmt.Errorf("version 0x%02X is not compatible with server version 0x%02X", 
			frame.Version, protocol.CurrentProtocolVersion)
	}
	
	// Process frame with version-specific handling
	return vh.versionHandler.HandleVersionSpecificMessage(frame, func(f *protocol.Frame) error {
		// Delegate to base handler's processFrame method
		return vh.ConnectionHandler.processFrame(nil, f) // Context can be nil for this use case
	})
}

// SendVersionSpecificError sends an error response appropriate for the client version
func (vh *VersionAwareConnectionHandler) SendVersionSpecificError(code pb.ErrorCode, message string) error {
	errorResp, err := vh.versionHandler.CreateVersionSpecificErrorResponse(vh.clientVersion, code, message)
	if err != nil {
		return err
	}
	
	return vh.conn.SendMessage(protocol.MessageTypeError, errorResp)
}

// GetVersionCapabilities returns capabilities for the current client version
func (vh *VersionAwareConnectionHandler) GetVersionCapabilities() (*protocol.VersionFeatures, error) {
	return vh.versionHandler.GetVersionCapabilities(vh.clientVersion)
}
