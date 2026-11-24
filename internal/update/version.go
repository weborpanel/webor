package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Normalize strips a leading "v" and surrounding space.
func Normalize(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// InferChannelFromVersion picks a release channel from a build stamp.
// Versions containing "beta" default to beta; everything else to stable.
func InferChannelFromVersion(v string) string {
	v = strings.ToLower(Normalize(v))
	if strings.Contains(v, "beta") {
		return "beta"
	}
	return "stable"
}

// Compare returns -1 if a < b, 0 if equal, 1 if a > b.
// Supports dotted numeric cores with optional -prerelease (test/beta/rc/…).
// A release without prerelease is newer than the same core with prerelease
// (1.0.0 > 1.0.0-beta.1). Prerelease labels use Webor order:
// dev < test < alpha < beta < rc < (unknown lex) so beta.N > test.N.
func Compare(a, b string) int {
	a = Normalize(a)
	b = Normalize(b)
	if a == b {
		return 0
	}
	ac, ap := splitCorePre(a)
	bc, bp := splitCorePre(b)
	if c := compareDots(ac, bc); c != 0 {
		return c
	}
	if ap == "" && bp == "" {
		return 0
	}
	if ap == "" {
		return 1 // release > prerelease
	}
	if bp == "" {
		return -1
	}
	return comparePre(ap, bp)
}

func splitCorePre(v string) (core, pre string) {
	if i := strings.IndexByte(v, '-'); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

func compareDots(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		avar, bvar := 0, 0
		if i < len(as) {
			avar, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bvar, _ = strconv.Atoi(bs[i])
		}
		if avar < bvar {
			return -1
		}
		if avar > bvar {
			return 1
		}
	}
	return 0
}

func comparePre(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		avar, bvar := "", ""
		if i < len(as) {
			avar = as[i]
		}
		if i < len(bs) {
			bvar = bs[i]
		}
		an, aNum := strconv.Atoi(avar)
		bn, bNum := strconv.Atoi(bvar)
		if aNum == nil && bNum == nil {
			if an < bn {
				return -1
			}
			if an > bn {
				return 1
			}
			continue
		}
		if aNum == nil {
			return -1 // numeric < non-numeric per semver-ish
		}
		if bNum == nil {
			return 1
		}
		if c := comparePreLabel(avar, bvar); c != 0 {
			return c
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	default:
		return 0
	}
}

// preLabelRank: lower = older. Unknown labels sort after known ones, by name.
func preLabelRank(s string) int {
	switch strings.ToLower(s) {
	case "dev", "dirty":
		return 1
	case "test":
		return 2
	case "alpha", "a":
		return 3
	case "beta", "b":
		return 4
	case "rc", "pre":
		return 5
	default:
		return 100
	}
}

func comparePreLabel(a, b string) int {
	ar, br := preLabelRank(a), preLabelRank(b)
	if ar != 100 || br != 100 {
		if ar < br {
			return -1
		}
		if ar > br {
			return 1
		}
		// same known rank (shouldn't happen for distinct labels) → fall through
		if ar != 100 {
			return 0
		}
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// ValidateChannel returns an error unless channel is stable or beta.
func ValidateChannel(ch string) error {
	switch strings.ToLower(strings.TrimSpace(ch)) {
	case "stable", "beta":
		return nil
	default:
		return fmt.Errorf("unsupported channel %q (want stable or beta)", ch)
	}
}
