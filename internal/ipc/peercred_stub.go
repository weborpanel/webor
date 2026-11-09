//go:build !linux

package ipc

import (
	"fmt"
	"net"
)

// PeerCred holds the credentials of the connected peer.
type PeerCred struct {
	PID int32
	UID uint32
	GID uint32
}

// PeerCredFromConn is a non-Linux stub; tests may inject a CredChecker instead.
func PeerCredFromConn(conn net.Conn) (PeerCred, error) {
	return PeerCred{}, fmt.Errorf("SO_PEERCRED is only available on Linux")
}
