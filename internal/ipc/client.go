package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Call sends one request over the helper Unix socket and returns the response.
func Call(socketPath string, req Request) (Response, error) {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return Response{}, fmt.Errorf("dial helper: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return Response{}, fmt.Errorf("encode: %w", err)
	}
	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return Response{}, fmt.Errorf("decode: %w", err)
	}
	return resp, nil
}
