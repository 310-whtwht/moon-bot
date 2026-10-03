package marketdata

import "time"

// Gap is a stretch of missing bars between two stored bars.
type Gap struct {
	After   time.Time // open time of the last bar before the gap
	Before  time.Time // open time of the first bar after the gap
	Missing int       // number of bars that would fit in the gap
}

// maxWeekendGap bounds how long a weekend closure can be.
const maxWeekendGap = 4 * 24 * time.Hour

// FindGaps returns gaps between consecutive open times (ascending) that are
// longer than one bar, excluding weekend closures. Holidays are reported,
// so the caller can judge whether they are expected.
func FindGaps(openTimes []time.Time, barLen time.Duration) []Gap {
	var gaps []Gap
	for i := 1; i < len(openTimes); i++ {
		prev, next := openTimes[i-1], openTimes[i]
		delta := next.Sub(prev)
		if delta <= barLen {
			continue
		}
		if isWeekendClosure(prev.Add(barLen), next, delta) {
			continue
		}
		gaps = append(gaps, Gap{After: prev, Before: next, Missing: int(delta/barLen) - 1})
	}
	return gaps
}

// isWeekendClosure reports whether the missing interval [from, to) covers a
// whole UTC Saturday and is short enough to be a weekend, not an outage.
func isWeekendClosure(from, to time.Time, delta time.Duration) bool {
	if delta > maxWeekendGap {
		return false
	}
	from, to = from.UTC(), to.UTC()
	for d := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC); d.Before(to); d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday && !d.Before(from) && !d.AddDate(0, 0, 1).After(to) {
			return true
		}
	}
	return false
}
