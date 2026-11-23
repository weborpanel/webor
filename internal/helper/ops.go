// Package helper implements privileged operations for webor-helper.
package helper

// Ops holds runtime options for privileged apply functions.
type Ops struct {
	CaddyAdminURL string
	CaddyLastGood string
	// Backup path overrides (tests / lab). Empty = production defaults.
	BackupEnvPath       string
	BackupStagingRoot   string
	RestoreStagingRoot  string
	ResticBin           string
	SitesRootOverride   string // when set, sites.RootDir is not used; see backup tests
}

// DefaultOps returns production defaults.
func DefaultOps() *Ops {
	return &Ops{}
}
