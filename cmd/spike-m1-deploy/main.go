// Command spike-m1-deploy creates a site, deploys a zip release, and optionally serves a check.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/webooza/webooza/internal/helper"
	"github.com/webooza/webooza/internal/sites"
)

func main() {
	domain := flag.String("domain", "m1-deploy.test", "site domain")
	zipPath := flag.String("zip", "", "existing zip (default: generate a demo zip under site tmp)")
	deleteData := flag.Bool("cleanup", true, "delete site user and data at the end")
	flag.Parse()

	ops := helper.DefaultOps()
	if err := ops.CreateSiteUser(*domain); err != nil {
		fail(err)
	}

	zp := *zipPath
	if zp == "" {
		zp = filepath.Join(sites.Tmp(*domain), "demo.zip")
		if err := writeDemoZip(zp); err != nil {
			fail(err)
		}
	}

	id, err := ops.DeployStaticZip(*domain, zp)
	if err != nil {
		fail(err)
	}
	body, err := os.ReadFile(filepath.Join(sites.Public(*domain), "index.html"))
	if err != nil {
		fail(err)
	}
	fmt.Printf("ok: release %s → public serves %q\n", id, string(body))

	if *deleteData {
		if err := ops.DeleteSiteUser(*domain, true); err != nil {
			fail(err)
		}
		fmt.Println("cleanup: site removed")
	}
}

func writeDemoZip(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	fw, err := w.Create("index.html")
	if err != nil {
		return err
	}
	if _, err := fw.Write([]byte("<!doctype html><h1>webooza-m1-deploy-ok</h1>\n")); err != nil {
		return err
	}
	return w.Close()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
