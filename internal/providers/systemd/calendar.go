package systemd

import (
	"fmt"
	"strconv"
	"strings"
)

var weekdayNames = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// WithTimezone appends an IANA/UTC zone to each OnCalendar expression
// (systemd expects the zone after the timestamp, e.g. "*-*-* 03:00:00 Asia/Kolkata").
func WithTimezone(cals []string, tz string) []string {
	tz = strings.TrimSpace(tz)
	if tz == "" || len(cals) == 0 {
		return cals
	}
	out := make([]string, len(cals))
	for i, c := range cals {
		c = strings.TrimSpace(c)
		if c == "" {
			out[i] = c
			continue
		}
		if strings.HasSuffix(c, " "+tz) {
			out[i] = c
			continue
		}
		out[i] = c + " " + tz
	}
	return out
}

// CronToOnCalendar converts a 5-field cron schedule to one or more systemd
// OnCalendar expressions. When both day-of-month and day-of-week are restricted,
// two expressions are returned (cron OR semantics).
func CronToOnCalendar(schedule string) ([]string, error) {
	fields := strings.Fields(strings.TrimSpace(schedule))
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron schedule must have 5 fields (got %d)", len(fields))
	}
	min, hour, dom, mon, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	minPart, err := cronTimePart(min, 0, 59, true)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hourPart, err := cronTimePart(hour, 0, 23, true)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	domPart, err := cronDatePart(dom, 1, 31, true)
	if err != nil {
		return nil, fmt.Errorf("day-of-month: %w", err)
	}
	monPart, err := cronDatePart(mon, 1, 12, true)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	dowPart, err := cronWeekdayPart(dow)
	if err != nil {
		return nil, fmt.Errorf("day-of-week: %w", err)
	}

	date := fmt.Sprintf("*-%s-%s", monPart, domPart)
	timePart := fmt.Sprintf("%s:%s:00", hourPart, minPart)

	domRestricted := dom != "*"
	dowRestricted := dow != "*"
	if domRestricted && dowRestricted {
		// cron ORs these; systemd ANDs weekday+date — emit two lines.
		return []string{
			date + " " + timePart,
			dowPart + " *-*-* " + timePart,
		}, nil
	}

	cal := date + " " + timePart
	if dowPart != "" {
		cal = dowPart + " " + cal
	}
	return []string{cal}, nil
}

func cronTimePart(field string, min, max int, zeroPad bool) (string, error) {
	return cronField(field, min, max, zeroPad, false)
}

func cronDatePart(field string, min, max int, zeroPad bool) (string, error) {
	return cronField(field, min, max, zeroPad, true)
}

func cronField(field string, min, max int, zeroPad, dateStyle bool) (string, error) {
	field = strings.TrimSpace(field)
	if field == "*" {
		return "*", nil
	}
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil || step < 1 {
			return "", fmt.Errorf("invalid step %q", field)
		}
		start := min
		if dateStyle && min == 1 {
			start = 1
		} else {
			start = 0
			if min > 0 && !dateStyle {
				start = min
			}
		}
		if dateStyle {
			// days/months: 01/5
			return fmt.Sprintf("%02d/%d", start, step), nil
		}
		// minutes/hours in time position: use start/step (e.g. 00/5)
		if zeroPad {
			return fmt.Sprintf("%02d/%d", start, step), nil
		}
		return fmt.Sprintf("%d/%d", start, step), nil
	}

	parts := strings.Split(field, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return "", fmt.Errorf("empty list item")
		}
		if strings.Contains(p, "-") && !strings.Contains(p, "/") {
			a, b, err := parseRange(p, min, max)
			if err != nil {
				return "", err
			}
			if zeroPad {
				out = append(out, fmt.Sprintf("%02d..%02d", a, b))
			} else {
				out = append(out, fmt.Sprintf("%d..%d", a, b))
			}
			continue
		}
		if strings.Contains(p, "/") {
			// n/step or a-b/step — support start/step only for v1
			bits := strings.SplitN(p, "/", 2)
			if len(bits) != 2 {
				return "", fmt.Errorf("invalid stepped field %q", p)
			}
			start, err := strconv.Atoi(bits[0])
			if err != nil {
				return "", fmt.Errorf("invalid start in %q", p)
			}
			step, err := strconv.Atoi(bits[1])
			if err != nil || step < 1 {
				return "", fmt.Errorf("invalid step in %q", p)
			}
			if start < min || start > max {
				return "", fmt.Errorf("value %d out of range", start)
			}
			if zeroPad {
				out = append(out, fmt.Sprintf("%02d/%d", start, step))
			} else {
				out = append(out, fmt.Sprintf("%d/%d", start, step))
			}
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return "", fmt.Errorf("invalid value %q", p)
		}
		if n < min || n > max {
			return "", fmt.Errorf("value %d out of range [%d,%d]", n, min, max)
		}
		if zeroPad {
			out = append(out, fmt.Sprintf("%02d", n))
		} else {
			out = append(out, strconv.Itoa(n))
		}
	}
	return strings.Join(out, ","), nil
}

func parseRange(p string, min, max int) (int, int, error) {
	bits := strings.SplitN(p, "-", 2)
	if len(bits) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q", p)
	}
	a, err := strconv.Atoi(bits[0])
	if err != nil {
		return 0, 0, err
	}
	b, err := strconv.Atoi(bits[1])
	if err != nil {
		return 0, 0, err
	}
	if a < min || b > max || a > b {
		return 0, 0, fmt.Errorf("range %q out of bounds", p)
	}
	return a, b, nil
}

func cronWeekdayPart(field string) (string, error) {
	field = strings.TrimSpace(field)
	if field == "*" {
		return "", nil
	}
	parts := strings.Split(field, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.Contains(p, "-") {
			a, b, err := parseWeekdayRange(p)
			if err != nil {
				return "", err
			}
			out = append(out, a+".."+b)
			continue
		}
		name, err := weekdayName(p)
		if err != nil {
			return "", err
		}
		out = append(out, name)
	}
	return strings.Join(out, ","), nil
}

func parseWeekdayRange(p string) (string, string, error) {
	bits := strings.SplitN(p, "-", 2)
	if len(bits) != 2 {
		return "", "", fmt.Errorf("invalid weekday range %q", p)
	}
	a, err := weekdayName(bits[0])
	if err != nil {
		return "", "", err
	}
	b, err := weekdayName(bits[1])
	if err != nil {
		return "", "", err
	}
	return a, b, nil
}

func weekdayName(p string) (string, error) {
	p = strings.TrimSpace(p)
	if n, err := strconv.Atoi(p); err == nil {
		if n == 7 {
			n = 0
		}
		if n < 0 || n > 6 {
			return "", fmt.Errorf("weekday %d out of range", n)
		}
		return weekdayNames[n], nil
	}
	switch strings.ToLower(p) {
	case "sun", "sunday":
		return "Sun", nil
	case "mon", "monday":
		return "Mon", nil
	case "tue", "tues", "tuesday":
		return "Tue", nil
	case "wed", "wednesday":
		return "Wed", nil
	case "thu", "thur", "thurs", "thursday":
		return "Thu", nil
	case "fri", "friday":
		return "Fri", nil
	case "sat", "saturday":
		return "Sat", nil
	default:
		return "", fmt.Errorf("invalid weekday %q", p)
	}
}
