// Command spike-m4-postgres ensures two site databases and checks CONNECT isolation.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/exec"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/weborpanel/webor/internal/helper"
	"github.com/weborpanel/webor/internal/providers/dbengine"
	"github.com/weborpanel/webor/internal/providers/postgres"
)

func main() {
	domainA := flag.String("domain", "example.com", "primary site domain")
	domainB := flag.String("other", "other.example.com", "second site for isolation")
	stateDir := flag.String("state-dir", "/var/lib/webor", "state dir for RO secrets")
	skipInstall := flag.Bool("skip-install-check", false, "do not fail when postgres missing")
	flag.Parse()

	ops := helper.DefaultOps()
	st, err := ops.PostgresStatus()
	if err != nil {
		fatal(err)
	}
	if !st.Installed || !st.Active {
		fmt.Fprintf(os.Stderr, "postgres status: installed=%v active=%v (%s)\n", st.Installed, st.Active, st.Message)
		fmt.Fprintln(os.Stderr, "install via approved InstallPostgres, then re-run")
		if !*skipInstall {
			os.Exit(2)
		}
	}

	for _, d := range []string{*domainA, *domainB} {
		if err := ops.CreateSiteUser(d); err != nil {
			fatal(fmt.Errorf("CreateSiteUser %s: %w", d, err))
		}
		res, err := ops.EnsureSiteDatabase(d, "postgres", "", *stateDir)
		if err != nil {
			fatal(fmt.Errorf("EnsureSiteDatabase %s: %w", d, err))
		}
		fmt.Printf("ensured %s db=%s peer=%s secret=%s\n", d, res.Database, res.PeerRole, res.SecretPath)
	}

	idA := dbengine.DefaultID(*domainA)
	dbA := postgres.DBName(*domainA, idA)
	roleB := postgres.PeerRole(*domainB)
	roleA := postgres.PeerRole(*domainA)

	// Peer A can connect to its own DB.
	if out, err := exec.Command("runuser", "-u", roleA, "--", "psql", "-d", dbA, "-At", "-c", "SELECT 1").CombinedOutput(); err != nil {
		fatal(fmt.Errorf("peer %s should CONNECT to %s: %w (%s)", roleA, dbA, err, out))
	}
	fmt.Printf("ok: peer user %s can CONNECT to %s\n", roleA, dbA)

	// Peer B must not CONNECT to A's database (PUBLIC CONNECT revoked).
	if out, err := exec.Command("runuser", "-u", roleB, "--", "psql", "-d", dbA, "-c", "SELECT 1").CombinedOutput(); err == nil {
		fatal(fmt.Errorf("isolation FAIL: peer %s connected to %s:\n%s", roleB, dbA, out))
	}
	fmt.Printf("ok: peer user %s cannot CONNECT to %s\n", roleB, dbA)

	// Panel RO secret for A should open.
	sec, err := postgres.ReadSecret(postgres.ResolveSecretPath(*stateDir, *domainA, idA))
	if err != nil {
		fatal(err)
	}
	db, err := sql.Open("pgx", sec.DSN())
	if err != nil {
		fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fatal(fmt.Errorf("readonly ping: %w", err))
	}
	fmt.Printf("ok: readonly DSN for %s\n", *domainA)
	fmt.Printf("peer_url %s\n", postgres.PeerURL(*domainA, idA))
	fmt.Println("ok: lab-postgres")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
