package ipc

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// CredChecker verifies the calling peer. On Linux the default uses SO_PEERCRED.
type CredChecker func(conn net.Conn) (PeerCred, error)

// HandlerFunc handles one validated operation.
type HandlerFunc func(req Request, cred PeerCred) Response

// Server serves typed JSON requests over a Unix socket.
type Server struct {
	SocketPath string
	CheckCred  CredChecker
	Handlers   map[string]HandlerFunc
	AllowedUID map[uint32]struct{} // empty = allow any uid (dev only)
}

// NewServer builds a helper IPC server with the M0 operation surface.
func NewServer(socketPath string, handlers map[string]HandlerFunc) *Server {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	return &Server{
		SocketPath: socketPath,
		CheckCred:  PeerCredFromConn,
		Handlers:   handlers,
	}
}

// ListenAndServe binds the Unix socket and serves forever.
func (s *Server) ListenAndServe() error {
	dir := filepath.Dir(s.SocketPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("socket dir: %w", err)
	}
	// Panel runs as user webor; allow group access to the socket directory.
	if g, err := user.LookupGroup("webor"); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			_ = os.Chown(dir, 0, gid)
		}
	}
	_ = os.Remove(s.SocketPath)
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.SocketPath, 0o660); err != nil {
		_ = ln.Close()
		return err
	}
	if g, err := user.LookupGroup("webor"); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			_ = os.Chown(s.SocketPath, 0, gid)
		}
	}
	defer ln.Close()
	log.Printf("webor-helper listening on %s", s.SocketPath)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	check := s.CheckCred
	if check == nil {
		check = PeerCredFromConn
	}
	cred, err := check(conn)
	if err != nil {
		writeResponse(conn, Response{OK: false, Error: "peercred: " + err.Error()})
		return
	}
	if len(s.AllowedUID) > 0 {
		if _, ok := s.AllowedUID[cred.UID]; !ok {
			writeResponse(conn, Response{OK: false, Error: "caller uid not allowed"})
			return
		}
	}

	dec := json.NewDecoder(conn)
	var req Request
	if err := dec.Decode(&req); err != nil {
		writeResponse(conn, Response{OK: false, Error: "invalid request: " + err.Error()})
		return
	}
	writeResponse(conn, s.dispatch(req, cred))
}

func (s *Server) dispatch(req Request, cred PeerCred) Response {
	if req.Operation == "" {
		return Response{ID: req.ID, OK: false, Error: "operation is required"}
	}
	h, ok := s.Handlers[req.Operation]
	if !ok {
		return Response{ID: req.ID, OK: false, Error: fmt.Sprintf("unknown operation %q", req.Operation)}
	}
	return h(req, cred)
}

func writeResponse(conn net.Conn, resp Response) {
	enc := json.NewEncoder(conn)
	_ = enc.Encode(resp)
}
