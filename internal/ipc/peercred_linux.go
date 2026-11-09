//go:build linux

package ipc

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// PeerCred holds the credentials of the connected peer from SO_PEERCRED.
type PeerCred struct {
	PID int32
	UID uint32
	GID uint32
}

// PeerCredFromConn reads SO_PEERCRED for a Unix socket connection.
func PeerCredFromConn(conn net.Conn) (PeerCred, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return PeerCred{}, fmt.Errorf("not a unix connection")
	}
	raw, err := unixConn.SyscallConn()
	if err != nil {
		return PeerCred{}, err
	}
	var (
		cred  *unix.Ucred
		opErr error
	)
	err = raw.Control(func(fd uintptr) {
		cred, opErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if err != nil {
		return PeerCred{}, err
	}
	if opErr != nil {
		return PeerCred{}, opErr
	}
	if cred == nil {
		return PeerCred{}, fmt.Errorf("empty peer credentials")
	}
	return PeerCred{PID: cred.Pid, UID: cred.Uid, GID: cred.Gid}, nil
}
