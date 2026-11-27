package helper

import (
	"strings"
	"testing"
)

func TestMariaAppGrantSQL(t *testing.T) {
	sql := mariaAppGrantSQL("wbr_tbts_in_mariadb_1", "wbr_tbts_in_mariadb_1", "wbr_tbts_in", "s'ecret")
	for _, want := range []string{
		"CREATE USER IF NOT EXISTS `wbr_tbts_in_mariadb_1`@'localhost' IDENTIFIED BY 's''ecret'",
		"GRANT ALL PRIVILEGES ON `wbr_tbts_in_mariadb_1`.* TO `wbr_tbts_in_mariadb_1`@'localhost'",
		"CREATE USER IF NOT EXISTS `wbr_tbts_in_mariadb_1`@'127.0.0.1' IDENTIFIED BY 's''ecret'",
		"GRANT ALL PRIVILEGES ON `wbr_tbts_in_mariadb_1`.* TO `wbr_tbts_in_mariadb_1`@'127.0.0.1'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("missing %s\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "GRANT SELECT") {
		t.Fatalf("app grant must be full privileges, got %s", sql)
	}

	peer := mariaAppGrantSQL("wbr_tbts_in", "wbr_tbts_in", "wbr_tbts_in", "secret")
	if strings.Contains(peer, "@'localhost' IDENTIFIED BY") {
		t.Fatalf("peer localhost account must stay unix socket\n%s", peer)
	}
	if !strings.Contains(peer, "GRANT ALL PRIVILEGES ON `wbr_tbts_in`.* TO `wbr_tbts_in`@'localhost'") {
		t.Fatalf("peer grant missing\n%s", peer)
	}
	if !strings.Contains(peer, "GRANT ALL PRIVILEGES ON `wbr_tbts_in`.* TO `wbr_tbts_in`@'127.0.0.1'") {
		t.Fatalf("tcp grant missing\n%s", peer)
	}
}
