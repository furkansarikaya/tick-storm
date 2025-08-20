package acceptance

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/furkansarikaya/tick-storm/internal/server"
	"github.com/furkansarikaya/tick-storm/internal/protocol"
	pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

func startTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	cfg := server.DefaultConfig()
	cfg.ListenAddr = "127.0.0.1:0" // ephemeral port on localhost
	// Ensure TLS disabled for tests
	if cfg.TLS != nil {
		cfg.TLS.Enabled = false
	}

	s := server.NewServer(cfg)
	require.NoError(t, s.Start())

	addr := s.ListenAddr()
	require.NotEmpty(t, addr)

	return s, addr
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()

	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("tcp", addr)
	require.NoError(t, err)
	return conn
}

func writeFrame(t *testing.T, conn net.Conn, frame *protocol.Frame) {
	t.Helper()
	w := protocol.NewFrameWriter(conn)
	require.NoError(t, w.WriteFrame(frame))
}

func readFrame(t *testing.T, conn net.Conn) *protocol.Frame {
	t.Helper()
	r := protocol.NewFrameReader(conn, protocol.DefaultMaxMessageSize)
	frame, err := r.ReadFrame()
	require.NoError(t, err)
	return frame
}

func setCreds(t *testing.T, user, pass string) {
	t.Helper()
	require.NoError(t, os.Setenv("STREAM_USER", user))
	require.NoError(t, os.Setenv("STREAM_PASS", pass))
}

// AC-1: AUTH must be the first frame. Sending any other message first should be rejected.
func TestAC1_AuthMustBeFirstFrame(t *testing.T) {
	setCreds(t, "ac_user", "ac_pass")
	s, addr := startTestServer(t)
	defer func() { _ = s.Stop(context.Background()) }()

	conn := dial(t, addr)
	defer conn.Close()

	// Send SUBSCRIBE as the first frame (invalid per protocol)
	sub := &pb.SubscribeRequest{Mode: pb.SubscriptionMode_SUBSCRIPTION_MODE_SECOND}
	frame, err := protocol.MarshalMessage(protocol.MessageTypeSubscribe, sub)
	require.NoError(t, err)
	writeFrame(t, conn, frame)

	// Expect ERROR with INVALID_AUTH
	resp := readFrame(t, conn)
	require.Equal(t, protocol.MessageTypeError, resp.Type)
	var errResp pb.ErrorResponse
	require.NoError(t, protocol.UnmarshalMessage(resp, &errResp))
	require.Equal(t, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, errResp.Code)
}

// AC-1: Invalid credentials must be rejected with an error.
func TestAC1_InvalidCredentialsRejected(t *testing.T) {
	setCreds(t, "valid_user", "valid_pass")
	s, addr := startTestServer(t)
	defer func() { _ = s.Stop(context.Background()) }()

	conn := dial(t, addr)
	defer conn.Close()

	// Send AUTH with wrong password
	authReq := &pb.AuthRequest{
		Username: "valid_user",
		Password: "wrong_pass",
		ClientId: "ac-test-client",
		Version:  "1.0.0",
	}
	frame, err := protocol.MarshalMessage(protocol.MessageTypeAuth, authReq)
	require.NoError(t, err)
	writeFrame(t, conn, frame)

	resp := readFrame(t, conn)
	require.Equal(t, protocol.MessageTypeError, resp.Type)
	var errResp pb.ErrorResponse
	require.NoError(t, protocol.UnmarshalMessage(resp, &errResp))
	require.Equal(t, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, errResp.Code)
}

// AC-1: Valid credentials should be accepted and ACK returned.
func TestAC1_ValidCredentialsAccepted(t *testing.T) {
	setCreds(t, "ok_user", "ok_pass")
	s, addr := startTestServer(t)
	defer func() { _ = s.Stop(context.Background()) }()

	conn := dial(t, addr)
	defer conn.Close()

	authReq := &pb.AuthRequest{
		Username: "ok_user",
		Password: "ok_pass",
		ClientId: "ac-test-client-2",
		Version:  "1.0.0",
	}
	frame, err := protocol.MarshalMessage(protocol.MessageTypeAuth, authReq)
	require.NoError(t, err)
	writeFrame(t, conn, frame)

	resp := readFrame(t, conn)
	require.Equal(t, protocol.MessageTypeACK, resp.Type)
	var ack pb.AckResponse
	require.NoError(t, protocol.UnmarshalMessage(resp, &ack))
	require.True(t, ack.Success)
	require.Equal(t, pb.MessageType_MESSAGE_TYPE_AUTH, ack.AckType)
}
