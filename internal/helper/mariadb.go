package helper

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/weborpanel/webor/internal/distro"
	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/mariadb"
)

// MariaDBStatusResult reports whether MariaDB/MySQL is usable.
type MariaDBStatusResult struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Message   string `json:"message,omitempty"`
}

// MariaDBStatus checks for mariadb/mysql client and service.
func (o *Ops) MariaDBStatus() (*MariaDBStatusResult, error) {
	res := &MariaDBStatusResult{}
	if _, err := exec.LookPath("mariadb"); err != nil {
		if _, err2 := exec.LookPath("mysql"); err2 != nil {
			res.Message = "mariadb/mysql client not found"
			return res, nil
		}
	}
	res.Installed = true
	for _, unit := range []string{"mariadb", "mysql"} {
		if out, err := exec.Command("systemctl", "is-active", unit).CombinedOutput(); err == nil {
			if strings.TrimSpace(string(out)) == "active" {
				res.Active = true
				res.Message = unit + " active"
				return res, nil
			}
		}
	}
	res.Message = "mariadb installed but not active"
	return res, nil
}

// InstallMariaDB installs and enables MariaDB (Debian/Ubuntu or Alma/RHEL).
func (o *Ops) InstallMariaDB() error {
	info := distro.Detect()
	switch {
	case info.IsRHEL() || distro.LookManager() == "dnf" || distro.LookManager() == "yum":
		if err := distro.InstallPackages("mariadb-server"); err != nil {
			return err
		}
	case info.IsDebian() || distro.LookManager() == "apt-get":
		if err := distro.InstallPackages("mariadb-server", "mariadb-client"); err != nil {
			return err
		}
	default:
		return fmt.Errorf("InstallMariaDB needs apt-get or dnf (Debian/Ubuntu or AlmaLinux/RHEL)")
	}
	if err := distro.RunCommands([][]string{{"systemctl", "enable", "--now", "mariadb"}}); err != nil {
		if err2 := distro.RunCommands([][]string{{"systemctl", "enable", "--now", "mysqld"}}); err2 != nil {
			return err
		}
	}
	st, err := o.MariaDBStatus()
	if err != nil {
		return err
	}
	if !st.Installed || !st.Active {
		return fmt.Errorf("mariadb install finished but service not active: %s", st.Message)
	}
	return nil
}

func (o *Ops) ensureMariaDB(domain, id, stateDir string) (*EnsureSiteDatabaseResult, error) {
	st, err := o.MariaDBStatus()
	if err != nil {
		return nil, err
	}
	if !st.Installed {
		return nil, fmt.Errorf("mariadb not installed")
	}
	if !st.Active {
		_ = exec.Command("systemctl", "start", "mariadb").Run()
		st, _ = o.MariaDBStatus()
		if st == nil || !st.Active {
			return nil, fmt.Errorf("mariadb not active")
		}
	}

	dbName := mariadb.DBName(domain, id)
	peer := mariadb.PeerRole(domain)
	app := mariadb.AppRole(domain, id)
	ro := mariadb.ReadOnlyRole(domain, id)
	secretPath := mariadb.SecretPath(stateDir, domain, id)
	password, err := randomPassword(24)
	if err != nil {
		return nil, err
	}

	var sql string
	if app == peer {
		// Peer socket for same-machine apps; same username@127.0.0.1 for TCP/env password.
		sql = fmt.Sprintf(`
CREATE DATABASE IF NOT EXISTS %s;
CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED VIA unix_socket;
ALTER USER %s@'localhost' IDENTIFIED VIA unix_socket;
GRANT ALL PRIVILEGES ON %s.* TO %s@'localhost';
CREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s;
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
GRANT ALL PRIVILEGES ON %s.* TO %s@'127.0.0.1';
CREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s;
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
GRANT SELECT ON %s.* TO %s@'127.0.0.1';
FLUSH PRIVILEGES;
`, mqIdent(dbName), mqIdent(peer), mqIdent(peer), mqIdent(dbName), mqIdent(peer),
			mqIdent(app), mqQuote(password), mqIdent(app), mqQuote(password),
			mqIdent(dbName), mqIdent(app),
			mqIdent(ro), mqQuote(password), mqIdent(ro), mqQuote(password),
			mqIdent(dbName), mqIdent(ro))
	} else {
		sql = fmt.Sprintf(`
CREATE DATABASE IF NOT EXISTS %s;
CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED VIA unix_socket;
GRANT ALL PRIVILEGES ON %s.* TO %s@'localhost';
CREATE USER IF NOT EXISTS %s@'localhost' IDENTIFIED BY %s;
ALTER USER %s@'localhost' IDENTIFIED BY %s;
GRANT ALL PRIVILEGES ON %s.* TO %s@'localhost';
CREATE USER IF NOT EXISTS %s@'127.0.0.1' IDENTIFIED BY %s;
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
GRANT SELECT ON %s.* TO %s@'127.0.0.1';
FLUSH PRIVILEGES;
`, mqIdent(dbName), mqIdent(peer), mqIdent(dbName), mqIdent(peer),
			mqIdent(app), mqQuote(password), mqIdent(app), mqQuote(password),
			mqIdent(dbName), mqIdent(app),
			mqIdent(ro), mqQuote(password), mqIdent(ro), mqQuote(password),
			mqIdent(dbName), mqIdent(ro))
	}
	if err := o.mysql(sql); err != nil {
		return nil, err
	}
	sec := mariadb.ROSecret{
		User:     ro,
		Password: password,
		Database: dbName,
		Host:     mariadb.DefaultTCPHost,
		Port:     mariadb.DefaultPort,
	}
	if err := mariadb.WriteSecret(secretPath, sec); err != nil {
		return nil, err
	}
	_ = chownWebor(secretPath)
	_ = chownWebor(filepath.Dir(secretPath))
	return &EnsureSiteDatabaseResult{
		Engine:     mariadb.Engine,
		ID:         id,
		Database:   dbName,
		PeerRole:   peer,
		AppRole:    app,
		ReadOnly:   ro,
		PeerDSN:    mariadb.PeerDSN(domain, id),
		PeerURL:    mariadb.PeerURL(domain, id),
		SecretPath: secretPath,
	}, nil
}

func (o *Ops) dropMariaDB(domain, id, stateDir string) error {
	dbName := mariadb.DBName(domain, id)
	peer := mariadb.PeerRole(domain)
	app := mariadb.AppRole(domain, id)
	ro := mariadb.ReadOnlyRole(domain, id)
	secretPath := mariadb.ResolveSecretPath(stateDir, domain, id)
	legacyPath := mariadb.LegacySecretPath(stateDir, domain)
	st, err := o.MariaDBStatus()
	if err != nil {
		return err
	}
	if st.Installed && st.Active {
		sql := fmt.Sprintf(`
DROP DATABASE IF EXISTS %s;
DROP USER IF EXISTS %s@'127.0.0.1';
DROP USER IF EXISTS %s@'127.0.0.1';
`, mqIdent(dbName), mqIdent(ro), mqIdent(app))
		if app != peer {
			sql += fmt.Sprintf("DROP USER IF EXISTS %s@'localhost';\n", mqIdent(app))
		}
		sql += "FLUSH PRIVILEGES;\n"
		_ = o.mysql(sql)
	}
	_ = mariadb.RemoveSecret(secretPath)
	if dbengine.IsDefaultID(domain, id) {
		_ = mariadb.RemoveSecret(legacyPath)
	}
	return nil
}

func (o *Ops) rotateMariaDBPassword(domain, id, stateDir string) (*EnsureSiteDatabaseResult, error) {
	st, err := o.MariaDBStatus()
	if err != nil {
		return nil, err
	}
	if !st.Installed || !st.Active {
		return nil, fmt.Errorf("mariadb not active")
	}
	dbName := mariadb.DBName(domain, id)
	peer := mariadb.PeerRole(domain)
	app := mariadb.AppRole(domain, id)
	ro := mariadb.ReadOnlyRole(domain, id)
	secretPath := mariadb.SecretPath(stateDir, domain, id)
	password, err := randomPassword(24)
	if err != nil {
		return nil, err
	}
	var sql string
	if app == peer {
		sql = fmt.Sprintf(`
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
FLUSH PRIVILEGES;
`, mqIdent(ro), mqQuote(password), mqIdent(app), mqQuote(password))
	} else {
		sql = fmt.Sprintf(`
ALTER USER %s@'127.0.0.1' IDENTIFIED BY %s;
ALTER USER %s@'localhost' IDENTIFIED BY %s;
FLUSH PRIVILEGES;
`, mqIdent(ro), mqQuote(password), mqIdent(app), mqQuote(password))
	}
	if err := o.mysql(sql); err != nil {
		return nil, err
	}
	sec := mariadb.ROSecret{
		User:     ro,
		Password: password,
		Database: dbName,
		Host:     mariadb.DefaultTCPHost,
		Port:     mariadb.DefaultPort,
	}
	if err := mariadb.WriteSecret(secretPath, sec); err != nil {
		return nil, err
	}
	_ = chownWebor(secretPath)
	return &EnsureSiteDatabaseResult{
		Engine:     mariadb.Engine,
		ID:         id,
		Database:   dbName,
		PeerRole:   peer,
		AppRole:    app,
		ReadOnly:   ro,
		PeerDSN:    mariadb.PeerDSN(domain, id),
		PeerURL:    mariadb.PeerURL(domain, id),
		SecretPath: secretPath,
	}, nil
}

func (o *Ops) mysql(sql string) error {
	bin := "mariadb"
	if _, err := exec.LookPath(bin); err != nil {
		bin = "mysql"
	}
	cmd := exec.Command(bin, "-u", "root", "-e", sql)
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("sudo", bin, "-u", "root", "-e", sql)
		out, err = cmd.CombinedOutput()
	}
	if err != nil {
		return fmt.Errorf("%s: %w (%s)", bin, err, truncateOut(out))
	}
	return nil
}

func mqQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func mqIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
