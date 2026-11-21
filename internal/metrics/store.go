package metrics

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Sample resolutions (seconds) stored in the res column.
const (
	ResRaw    = 60
	ResRollup = 900
)

// Retention per resolution.
const (
	RawRetention    = 48 * time.Hour
	RollupRetention = 8 * 24 * time.Hour
)

// HostRow is one stored host sample.
type HostRow struct {
	TS        int64
	CPUPct    float64
	CPUMax    float64
	MemUsed   int64
	MemTotal  int64
	Load1     float64
	DiskUsed  int64
	DiskTotal int64
	NetRx     float64
	NetTx     float64
}

// SiteRow is one stored per-site sample.
type SiteRow struct {
	TS        int64   `json:"ts"`
	Site      string  `json:"-"`
	CPUPct    float64 `json:"cpu_pct"`
	CPUMax    float64 `json:"-"`
	MemBytes  int64   `json:"mem_bytes"`
	DiskBytes int64   `json:"disk_bytes"`
	Active    bool    `json:"active"`
}

// Store persists samples in the panel state database (migration 003_metrics).
type Store struct {
	DB *sql.DB
}

func (s *Store) InsertHost(res int, r HostRow) error {
	_, err := s.DB.Exec(`
INSERT OR REPLACE INTO metrics_host
  (res, ts, cpu_pct, cpu_max, mem_used, mem_total, load1, disk_used, disk_total, net_rx, net_tx)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		res, r.TS, r.CPUPct, r.CPUMax, r.MemUsed, r.MemTotal, r.Load1, r.DiskUsed, r.DiskTotal, r.NetRx, r.NetTx)
	return err
}

func (s *Store) InsertSites(res int, rows []SiteRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`
INSERT OR REPLACE INTO metrics_site (res, ts, site, cpu_pct, cpu_max, mem_bytes, disk_bytes, active)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		active := 0
		if r.Active {
			active = 1
		}
		if _, err := stmt.Exec(res, r.TS, r.Site, r.CPUPct, r.CPUMax, r.MemBytes, r.DiskBytes, active); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Rollup aggregates raw samples in [bucket, bucket+ResRollup) into one
// rollup row per series. Idempotent.
func (s *Store) Rollup(bucket int64) error {
	end := bucket + ResRollup
	if _, err := s.DB.Exec(`
INSERT OR REPLACE INTO metrics_host
  (res, ts, cpu_pct, cpu_max, mem_used, mem_total, load1, disk_used, disk_total, net_rx, net_tx)
SELECT ?, ?, AVG(cpu_pct), MAX(cpu_max), CAST(AVG(mem_used) AS INTEGER), MAX(mem_total), AVG(load1),
       CAST(AVG(disk_used) AS INTEGER), MAX(disk_total), AVG(net_rx), AVG(net_tx)
FROM metrics_host WHERE res = ? AND ts >= ? AND ts < ?
HAVING COUNT(*) > 0`, ResRollup, bucket, ResRaw, bucket, end); err != nil {
		return fmt.Errorf("rollup host: %w", err)
	}
	if _, err := s.DB.Exec(`
INSERT OR REPLACE INTO metrics_site (res, ts, site, cpu_pct, cpu_max, mem_bytes, disk_bytes, active)
SELECT ?, ?, site, AVG(cpu_pct), MAX(cpu_max), CAST(AVG(mem_bytes) AS INTEGER),
       CAST(AVG(disk_bytes) AS INTEGER), MAX(active)
FROM metrics_site WHERE res = ? AND ts >= ? AND ts < ?
GROUP BY site`, ResRollup, bucket, ResRaw, bucket, end); err != nil {
		return fmt.Errorf("rollup sites: %w", err)
	}
	return nil
}

// Prune drops samples past their retention.
func (s *Store) Prune(now time.Time) error {
	rawCut := now.Add(-RawRetention).Unix()
	rollCut := now.Add(-RollupRetention).Unix()
	for _, table := range []string{"metrics_host", "metrics_site"} {
		if _, err := s.DB.Exec(`DELETE FROM `+table+` WHERE (res = ? AND ts < ?) OR (res = ? AND ts < ?)`,
			ResRaw, rawCut, ResRollup, rollCut); err != nil {
			return fmt.Errorf("prune %s: %w", table, err)
		}
	}
	return nil
}

// Range is a graph window and the bucket size it is downsampled to.
type Range struct {
	Name string
	Span time.Duration
	Step int64 // seconds per point
}

var ranges = map[string]Range{
	"1h":  {Name: "1h", Span: time.Hour, Step: 60},
	"24h": {Name: "24h", Span: 24 * time.Hour, Step: 300},
	"7d":  {Name: "7d", Span: 7 * 24 * time.Hour, Step: 1800},
}

// ParseRange maps "1h", "24h" or "7d" (default "1h" when empty).
func ParseRange(name string) (Range, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "1h"
	}
	r, ok := ranges[name]
	return r, ok
}

// SparkRange is the short window shown inline in site lists.
var SparkRange = Range{Name: "spark", Span: time.Hour, Step: 120}

// Series is a columnar time series: T[i] pairs with every Values[k][i].
type Series struct {
	Range  string               `json:"range"`
	Step   int64                `json:"step"`
	From   int64                `json:"from"`
	To     int64                `json:"to"`
	T      []int64              `json:"t"`
	Values map[string][]float64 `json:"series"`
}

func newSeries(r Range, from, to int64, keys []string) *Series {
	s := &Series{Range: r.Name, Step: r.Step, From: from, To: to, T: []int64{}, Values: map[string][]float64{}}
	for _, k := range keys {
		s.Values[k] = []float64{}
	}
	return s
}

// sourceFilter picks raw rows for short windows, and rollups (plus raw rows
// newer than the last rollup) for windows longer than raw retention.
func (s *Store) sourceFilter(table string, r Range) (string, []any, error) {
	if r.Span <= RawRetention {
		return "res = ?", []any{ResRaw}, nil
	}
	var last sql.NullInt64
	if err := s.DB.QueryRow(`SELECT MAX(ts) FROM `+table+` WHERE res = ?`, ResRollup).Scan(&last); err != nil {
		return "", nil, err
	}
	cut := int64(0)
	if last.Valid {
		cut = last.Int64 + ResRollup
	}
	return "((res = ? AND ts < ?) OR (res = ? AND ts >= ?))", []any{ResRollup, cut, ResRaw, cut}, nil
}

var hostKeys = []string{"cpu_pct", "cpu_max", "mem_used", "mem_total", "load1", "disk_used", "disk_total", "net_rx", "net_tx"}

// HostSeries returns host metrics for the window ending at now.
func (s *Store) HostSeries(r Range, now time.Time) (*Series, error) {
	to := now.Unix()
	from := now.Add(-r.Span).Unix()
	filter, args, err := s.sourceFilter("metrics_host", r)
	if err != nil {
		return nil, err
	}
	q := `
SELECT (ts / ?) * ? AS b, AVG(cpu_pct), MAX(cpu_max), AVG(mem_used), MAX(mem_total), AVG(load1),
       AVG(disk_used), MAX(disk_total), AVG(net_rx), AVG(net_tx)
FROM metrics_host WHERE ts >= ? AND ` + filter + `
GROUP BY b ORDER BY b`
	rows, err := s.DB.Query(q, append([]any{r.Step, r.Step, from}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := newSeries(r, from, to, hostKeys)
	for rows.Next() {
		var b int64
		vals := make([]float64, len(hostKeys))
		ptrs := []any{&b}
		for i := range vals {
			ptrs = append(ptrs, &vals[i])
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		out.T = append(out.T, b)
		for i, k := range hostKeys {
			out.Values[k] = append(out.Values[k], vals[i])
		}
	}
	return out, rows.Err()
}

var siteKeys = []string{"cpu_pct", "cpu_max", "mem_bytes", "disk_bytes", "active"}

// SiteSeries returns one site's metrics for the window ending at now.
func (s *Store) SiteSeries(site string, r Range, now time.Time) (*Series, error) {
	all, err := s.siteSeries(site, r, now)
	if err != nil {
		return nil, err
	}
	if ser, ok := all[site]; ok {
		return ser, nil
	}
	return newSeries(r, now.Add(-r.Span).Unix(), now.Unix(), siteKeys), nil
}

// AllSiteSeries returns every site's metrics for the window, keyed by site.
func (s *Store) AllSiteSeries(r Range, now time.Time) (map[string]*Series, error) {
	return s.siteSeries("", r, now)
}

func (s *Store) siteSeries(site string, r Range, now time.Time) (map[string]*Series, error) {
	to := now.Unix()
	from := now.Add(-r.Span).Unix()
	filter, args, err := s.sourceFilter("metrics_site", r)
	if err != nil {
		return nil, err
	}
	where := "ts >= ? AND " + filter
	qargs := append([]any{r.Step, r.Step, from}, args...)
	if site != "" {
		where += " AND site = ?"
		qargs = append(qargs, site)
	}
	q := `
SELECT site, (ts / ?) * ? AS b, AVG(cpu_pct), MAX(cpu_max), AVG(mem_bytes), AVG(disk_bytes), MAX(active)
FROM metrics_site WHERE ` + where + `
GROUP BY site, b ORDER BY site, b`
	rows, err := s.DB.Query(q, qargs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*Series{}
	for rows.Next() {
		var name string
		var b int64
		vals := make([]float64, len(siteKeys))
		ptrs := []any{&name, &b}
		for i := range vals {
			ptrs = append(ptrs, &vals[i])
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		ser := out[name]
		if ser == nil {
			ser = newSeries(r, from, to, siteKeys)
			out[name] = ser
		}
		ser.T = append(ser.T, b)
		for i, k := range siteKeys {
			ser.Values[k] = append(ser.Values[k], vals[i])
		}
	}
	return out, rows.Err()
}
