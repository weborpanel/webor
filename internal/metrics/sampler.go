package metrics

import (
	"context"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/weborpanel/webor/internal/config"
)

// SiteUsage is the raw per-site reading the sampler turns into a SiteRow.
type SiteUsage struct {
	MemBytes     int64
	DiskBytes    int64
	CPUUsageUSec int64 // cumulative
	Active       bool
}

// Sampler periodically records host and site usage.
type Sampler struct {
	Store *Store
	Host  *HostCollector
	// Sites lists configured sites (re-read every tick so new sites appear).
	Sites func() ([]config.Site, error)
	// Usage reads one site; includeDisk requests the slow du walk.
	Usage func(site config.Site, includeDisk bool) (SiteUsage, error)

	Interval  time.Duration // default 60s
	DiskEvery time.Duration // default 15m
	Now       func() time.Time
	Logf      func(format string, args ...any)

	mu          sync.RWMutex
	latestHost  HostSnapshot
	hostReady   bool
	latestSites map[string]SiteRow

	prevCPU    cpuTimes
	prevRx     uint64
	prevTx     uint64
	prevAt     time.Time
	siteCPU    map[string]siteCPUMark
	siteDisk   map[string]diskMark
	lastRollup int64
	lastPrune  time.Time
}

type siteCPUMark struct {
	usec int64
	at   time.Time
}

type diskMark struct {
	bytes int64
	at    time.Time
}

func (s *Sampler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Sampler) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Run samples until ctx is cancelled. The first tick only primes CPU
// counters; a quick follow-up tick gives the dashboard data within seconds.
func (s *Sampler) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	s.tickLogged()
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}
	s.tickLogged()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.tickLogged()
		}
	}
}

func (s *Sampler) tickLogged() {
	if err := s.Tick(); err != nil {
		s.logf("metrics: %v", err)
	}
}

// Tick takes one sample of the host and every site.
func (s *Sampler) Tick() error {
	now := s.now()
	ts := now.Unix() / ResRaw * ResRaw
	host := s.sampleHost(now)
	if host != nil && s.Store != nil {
		if err := s.Store.InsertHost(ResRaw, HostRow{
			TS: ts, CPUPct: host.CPUPct, CPUMax: host.CPUPct,
			MemUsed: host.MemUsed, MemTotal: host.MemTotal, Load1: host.Load1,
			DiskUsed: host.DiskUsed, DiskTotal: host.DiskTotal,
			NetRx: host.NetRx, NetTx: host.NetTx,
		}); err != nil {
			return err
		}
	}

	rows := s.sampleSites(now, ts)
	if s.Store != nil {
		if err := s.Store.InsertSites(ResRaw, rows); err != nil {
			return err
		}
		bucket := ts/ResRollup*ResRollup - ResRollup
		if bucket > s.lastRollup {
			if err := s.Store.Rollup(bucket); err != nil {
				return err
			}
			s.lastRollup = bucket
		}
		if now.Sub(s.lastPrune) >= time.Hour {
			if err := s.Store.Prune(now); err != nil {
				return err
			}
			s.lastPrune = now
		}
	}
	return nil
}

// sampleHost returns nil on the priming tick (no CPU delta yet).
func (s *Sampler) sampleHost(now time.Time) *HostSnapshot {
	hc := s.Host
	if hc == nil {
		hc = &HostCollector{}
	}
	snap := hc.Static()
	cpu, cpuErr := hc.readCPU()
	rx, tx, netErr := hc.readNet()
	primed := !s.prevAt.IsZero()
	if primed {
		if cpuErr == nil {
			snap.CPUPct = cpuPercent(s.prevCPU, cpu)
		}
		if secs := now.Sub(s.prevAt).Seconds(); netErr == nil && secs > 0 {
			if rx >= s.prevRx {
				snap.NetRx = float64(rx-s.prevRx) / secs
			}
			if tx >= s.prevTx {
				snap.NetTx = float64(tx-s.prevTx) / secs
			}
		}
	}
	s.prevCPU, s.prevRx, s.prevTx, s.prevAt = cpu, rx, tx, now

	s.mu.Lock()
	s.latestHost = snap
	s.hostReady = s.hostReady || primed
	s.mu.Unlock()
	if !primed {
		return nil
	}
	return &snap
}

func (s *Sampler) sampleSites(now time.Time, ts int64) []SiteRow {
	if s.Sites == nil || s.Usage == nil {
		return nil
	}
	sites, err := s.Sites()
	if err != nil {
		s.logf("metrics: list sites: %v", err)
		return nil
	}
	if s.siteCPU == nil {
		s.siteCPU = map[string]siteCPUMark{}
		s.siteDisk = map[string]diskMark{}
	}
	diskEvery := s.DiskEvery
	if diskEvery <= 0 {
		diskEvery = 15 * time.Minute
	}
	ncpu := float64(runtime.NumCPU())
	rows := make([]SiteRow, 0, len(sites))
	seen := map[string]bool{}
	for _, site := range sites {
		name := strings.ToLower(strings.TrimSpace(site.Name))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		dm, haveDisk := s.siteDisk[name]
		includeDisk := !haveDisk || now.Sub(dm.at) >= diskEvery
		u, err := s.Usage(site, includeDisk)
		if err != nil {
			s.logf("metrics: site %s: %v", name, err)
			continue
		}
		row := SiteRow{TS: ts, Site: name, MemBytes: u.MemBytes, Active: u.Active}
		if includeDisk {
			dm = diskMark{bytes: u.DiskBytes, at: now}
			s.siteDisk[name] = dm
		}
		row.DiskBytes = dm.bytes
		if prev, ok := s.siteCPU[name]; ok && u.CPUUsageUSec >= prev.usec {
			if wall := now.Sub(prev.at).Microseconds(); wall > 0 {
				row.CPUPct = clampPct(float64(u.CPUUsageUSec-prev.usec) / float64(wall) / ncpu * 100)
			}
		}
		row.CPUMax = row.CPUPct
		s.siteCPU[name] = siteCPUMark{usec: u.CPUUsageUSec, at: now}
		rows = append(rows, row)
	}
	for name := range s.siteCPU {
		if !seen[name] {
			delete(s.siteCPU, name)
			delete(s.siteDisk, name)
		}
	}

	latest := make(map[string]SiteRow, len(rows))
	for _, r := range rows {
		latest[r.Site] = r
	}
	s.mu.Lock()
	s.latestSites = latest
	s.mu.Unlock()
	return rows
}

// LatestHost returns the most recent host snapshot; ok is false until a
// CPU delta is available.
func (s *Sampler) LatestHost() (HostSnapshot, bool) {
	if s == nil {
		return HostSnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestHost, s.hostReady
}

// LatestSite returns the most recent sample for a site.
func (s *Sampler) LatestSite(name string) (SiteRow, bool) {
	if s == nil {
		return SiteRow{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.latestSites[strings.ToLower(name)]
	return r, ok
}
