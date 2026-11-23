package mariadb

import "os"

// CommonUnixSockets are candidate MariaDB/MySQL Unix socket paths.
var CommonUnixSockets = []string{
	"/run/mysqld/mysqld.sock",      // Debian/Ubuntu
	"/var/lib/mysql/mysql.sock",    // RHEL / Alma / Rocky
	"/run/mysql/mysql.sock",
	"/var/run/mysqld/mysqld.sock",
	"/tmp/mysql.sock",
}

// UnixSocketPath returns the first existing MariaDB/MySQL socket, or the Debian default.
func UnixSocketPath() string {
	for _, p := range CommonUnixSockets {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return CommonUnixSockets[0]
}
