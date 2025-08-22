package acceptance

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/stretchr/testify/require"

    "github.com/furkansarikaya/tick-storm/internal/protocol"
    pb "github.com/furkansarikaya/tick-storm/internal/protocol/pb"
)

// AC-RateLimit: Verify per-IP auth rate limiting and metrics at server level
func TestACRateLimitingPerIPAndMetrics(t *testing.T) {
    // Set credentials and rate limit env overrides
    setCreds(t, "rl_user", "rl_pass")
    require.NoError(t, os.Setenv("AUTH_MAX_ATTEMPTS", "1"))
    require.NoError(t, os.Setenv("AUTH_RATE_LIMIT_WINDOW", "100ms"))
    t.Cleanup(func() {
        _ = os.Unsetenv("AUTH_MAX_ATTEMPTS")
        _ = os.Unsetenv("AUTH_RATE_LIMIT_WINDOW")
    })

    s, addr := startTestServer(t)
    defer func() { _ = s.Stop(context.Background()) }()

    // Helper to send AUTH and receive error code
    sendAuthAndGetCode := func(username, password string) pb.ErrorCode {
        conn := dial(t, addr)
        defer conn.Close()

        req := &pb.AuthRequest{
            Username: username,
            Password: password,
            ClientId: "ac-rl-client",
            Version:  "1.0.0",
        }
        frame, err := protocol.MarshalMessage(protocol.MessageTypeAuth, req)
        require.NoError(t, err)
        writeFrame(t, conn, frame)

        resp := readFrame(t, conn)
        require.Equal(t, protocol.MessageTypeError, resp.Type)
        var errResp pb.ErrorResponse
        require.NoError(t, protocol.UnmarshalMessage(resp, &errResp))
        return errResp.Code
    }

    // 1) First attempt: invalid credentials => INVALID_AUTH
    code1 := sendAuthAndGetCode("rl_user", "wrong")
    require.Equal(t, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, code1)

    // 2) Second attempt from same IP (new connection) => RATE_LIMITED
    code2 := sendAuthAndGetCode("rl_user", "wrong")
    require.Equal(t, pb.ErrorCode_ERROR_CODE_RATE_LIMITED, code2)

    // Verify metrics after two attempts
    stats := s.GetStats()
    require.EqualValues(t, 1, stats["auth_failures"])    // first invalid
    require.EqualValues(t, 1, stats["auth_rate_limited"]) // second blocked

    // 3) After block period (> 3 * window due to penalty), attempts should be allowed again
    time.Sleep(350 * time.Millisecond)
    code3 := sendAuthAndGetCode("rl_user", "wrong")
    require.Equal(t, pb.ErrorCode_ERROR_CODE_INVALID_AUTH, code3)

    // Metrics reflect the third invalid attempt
    stats = s.GetStats()
    require.EqualValues(t, 2, stats["auth_failures"])    // now two invalids
    require.EqualValues(t, 1, stats["auth_rate_limited"]) // still one rate-limited
}
