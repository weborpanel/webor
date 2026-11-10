// Package helper implements privileged operations for webooza-helper.
package helper

// Ops holds runtime options for privileged apply functions.
type Ops struct {
	CaddyAdminURL string
	CaddyLastGood string
}

// DefaultOps returns production defaults.
func DefaultOps() *Ops {
	return &Ops{}
}
