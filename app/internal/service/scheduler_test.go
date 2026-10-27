package service

import (
	"testing"
	"time"
)

func TestNextDailyRun(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, tc := range []struct {
		now  string
		want string
	}{{"2026-10-26T01:59:00+08:00", "2026-10-26T02:00:00+08:00"}, {"2026-10-26T02:00:00+08:00", "2026-10-27T02:00:00+08:00"}, {"2026-12-31T23:00:00+08:00", "2027-01-01T02:00:00+08:00"}} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		if got := nextDailyRun(now.In(loc), 2).Format(time.RFC3339); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
}
