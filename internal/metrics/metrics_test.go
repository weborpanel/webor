package metrics

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/weborpanel/webor/internal/config"
)

func TestParseProcStat(t *testing.T) {
	data := []byte("cpu  100 0 50 800 50 0 0 0 0 0\ncpu0 1 2 3 4\n")
	got, err := parseProcStat(data)
	if err != nil {
		t.Fatal(err)
	}
	if got.total != 1000 || got.idle != 850 {
		t.Fatalf("got %+v", got)
	}
	next := cpuTimes{total: 2000, idle: 1350}
	if pct := cpuPercent(got, next); math.Abs(pct-50) > 0.001 {
		t.Fatalf("cpu pct = %v", pct)
	}
	if pct := cpuPercent(next, got); pct != 0 {
		t.Fatalf("backwards counter pct = %v", pct)
	}
}

func TestParseMeminfo(t *testing.T) {
	mi, err := parseMeminfo([]byte("MemTotal:  2048 kB\nMemFree: 512 kB\nMemAvailable:  1024 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if mi.total != 2048*1024 || mi.available != 1024*1024 {
		t.Fatalf("got %+v", mi)
	}
	mi, err = parseMeminfo([]byte("MemTotal: 100 kB\nMemFree: 10 kB\nBuffers: 5 kB\nCached: 5 kB\n"))
	if err != nil {
		t.Fatal(err)
	}
	if mi.available != 20*1024 {
		t.Fatalf("fallback available = %d", mi.available)
	}
	if _, err := parseMeminfo([]byte("nope")); err == nil {
		t.Fatal("expected error without MemTotal")
	}
}

func TestParseLoadNetUptime(t *testing.T) {
	l1, l5, l15, err := parseLoadavg([]byte("0.50 0.25 0.10 1/100 1234\n"))
	if err != nil || l1 != 0.5 || l5 != 0.25 || l15 != 0.1 {
		t.Fatalf("load = %v %v %v %v", l1, l5, l15, err)
	}
	net := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 9999 1 0 0 0 0 0 0 9999 1 0 0 0 0 0 0
  eth0: 1000 10 0 0 0 0 0 0 2000 20 0 0 0 0 0 0
  eth1: 1 1 0 0 0 0 0 0 2 1 0 0 0 0 0 0
`
	rx, tx, err := parseNetDev([]byte(net))
	if err != nil || rx != 1001 || tx != 2002 {
		t.Fatalf("net = %d %d %v", rx, tx, err)
	}
	up, err := parseUptime([]byte("12345.67 999.0\n"))
	if err != nil || up != 12345.67 {
		t.Fatalf("uptime = %v %v", up, err)
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"":         0,
		"infinity": 0,
		"1024":     1024,
		"512M":     512 << 20,
		"1G":       1 << 30,
		"1GiB":     1 << 30,
		"2k":       2048,
		"1.5G":     3 << 29,
		"junk":     0,
	}
	for in, want := range cases {
		if got := ParseSize(in); got != want {
			t.Errorf("ParseSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func openStore(t *testing.T) *Store {
	t.Helper()
	db, err := config.OpenDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Store{DB: db}
}

func TestStoreRollupPruneSeries(t *testing.T) {
	st := openStore(t)
	base := time.Unix(1_800_000_000/ResRollup*ResRollup, 0)
	for i := 0; i < 15; i++ {
		ts := base.Unix() + int64(i*ResRaw)
		if err := st.InsertHost(ResRaw, HostRow{TS: ts, CPUPct: float64(i), CPUMax: float64(i), MemUsed: 100, MemTotal: 200}); err != nil {
			t.Fatal(err)
		}
		if err := st.InsertSites(ResRaw, []SiteRow{{TS: ts, Site: "a.example", CPUPct: 10, MemBytes: 50, DiskBytes: 7, Active: i%2 == 0}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.Rollup(base.Unix()); err != nil {
		t.Fatal(err)
	}
	var avg, max float64
	if err := st.DB.QueryRow(`SELECT cpu_pct, cpu_max FROM metrics_host WHERE res = ? AND ts = ?`, ResRollup, base.Unix()).Scan(&avg, &max); err != nil {
		t.Fatal(err)
	}
	if avg != 7 || max != 14 {
		t.Fatalf("rollup avg=%v max=%v", avg, max)
	}
	var active int
	if err := st.DB.QueryRow(`SELECT active FROM metrics_site WHERE res = ? AND site = ?`, ResRollup, "a.example").Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 1 {
		t.Fatalf("rollup active = %d", active)
	}

	now := base.Add(15 * time.Minute)
	r, _ := ParseRange("1h")
	ser, err := st.HostSeries(r, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ser.T) != 15 || len(ser.Values["cpu_pct"]) != 15 || ser.Values["mem_total"][0] != 200 {
		t.Fatalf("1h series: %d points %+v", len(ser.T), ser.Values["mem_total"])
	}
	r24, _ := ParseRange("24h")
	ser, err = st.HostSeries(r24, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ser.T) != 3 {
		t.Fatalf("24h series expected 3 five-minute buckets, got %d", len(ser.T))
	}
	r7, _ := ParseRange("7d")
	ser, err = st.HostSeries(r7, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ser.T) != 1 || ser.Values["cpu_pct"][0] != 7 {
		t.Fatalf("7d series should come from rollup only: %+v %+v", ser.T, ser.Values["cpu_pct"])
	}
	site, err := st.SiteSeries("a.example", r, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(site.T) != 15 || site.Values["disk_bytes"][0] != 7 {
		t.Fatalf("site series: %+v", site)
	}
	empty, err := st.SiteSeries("missing.example", r, now)
	if err != nil || len(empty.T) != 0 || empty.Values["cpu_pct"] == nil {
		t.Fatalf("missing site series: %+v %v", empty, err)
	}

	if err := st.Prune(base.Add(RawRetention + time.Hour)); err != nil {
		t.Fatal(err)
	}
	var raw, roll int
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM metrics_host WHERE res = ?`, ResRaw).Scan(&raw)
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM metrics_host WHERE res = ?`, ResRollup).Scan(&roll)
	if raw != 0 || roll != 1 {
		t.Fatalf("after prune raw=%d rollup=%d", raw, roll)
	}
}

func TestParseRange(t *testing.T) {
	if r, ok := ParseRange(""); !ok || r.Name != "1h" {
		t.Fatalf("default range = %+v %v", r, ok)
	}
	if _, ok := ParseRange("30d"); ok {
		t.Fatal("30d should be rejected")
	}
}

func writeProc(t *testing.T, dir, stat, netdev string) {
	t.Helper()
	files := map[string]string{
		"stat":    stat,
		"net/dev": netdev,
		"meminfo": "MemTotal: 1000 kB\nMemAvailable: 250 kB\n",
		"loadavg": "1.00 0.50 0.25 1/1 1\n",
		"uptime":  "3600.0 1.0\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSamplerTick(t *testing.T) {
	st := openStore(t)
	proc := t.TempDir()
	writeProc(t, proc, "cpu 0 0 0 1000 0 0 0 0\n", "eth0: 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0\n")

	now := time.Unix(1_800_000_000, 0)
	var usec int64
	diskCalls := 0
	s := &Sampler{
		Store: st,
		Host:  &HostCollector{ProcRoot: proc},
		Sites: func() ([]config.Site, error) {
			return []config.Site{{Name: "A.example"}}, nil
		},
		Usage: func(site config.Site, includeDisk bool) (SiteUsage, error) {
			u := SiteUsage{MemBytes: 42, CPUUsageUSec: usec, Active: true}
			if includeDisk {
				diskCalls++
				u.DiskBytes = 1234
			}
			return u, nil
		},
		Now:  func() time.Time { return now },
		Logf: t.Logf,
	}
	if err := s.Tick(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.LatestHost(); ok {
		t.Fatal("host should not be ready after priming tick")
	}
	var n int
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM metrics_host`).Scan(&n)
	if n != 0 {
		t.Fatalf("priming tick stored %d host rows", n)
	}

	now = now.Add(time.Minute)
	usec = 60_000_000 // one core fully busy for the minute
	writeProc(t, proc, "cpu 500 0 0 1500 0 0 0 0\n", "eth0: 6000 0 0 0 0 0 0 0 12000 0 0 0 0 0 0 0\n")
	if err := s.Tick(); err != nil {
		t.Fatal(err)
	}
	host, ok := s.LatestHost()
	if !ok || math.Abs(host.CPUPct-50) > 0.01 || host.NetRx != 100 || host.NetTx != 200 {
		t.Fatalf("host = %+v ok=%v", host, ok)
	}
	if host.MemTotal != 1000*1024 || host.MemUsed != 750*1024 {
		t.Fatalf("host mem = %+v", host)
	}
	site, ok := s.LatestSite("a.example")
	if !ok || site.MemBytes != 42 || site.DiskBytes != 1234 || site.CPUPct <= 0 {
		t.Fatalf("site = %+v ok=%v", site, ok)
	}
	if diskCalls != 1 {
		t.Fatalf("disk should be sampled once within DiskEvery, got %d", diskCalls)
	}
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM metrics_site WHERE res = ?`, ResRaw).Scan(&n)
	if n != 2 {
		t.Fatalf("site rows = %d", n)
	}
}
