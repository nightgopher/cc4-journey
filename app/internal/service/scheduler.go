package service

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

func BusinessLocation() *time.Location {
	name := os.Getenv("BUSINESS_TIMEZONE")
	if name == "" {
		name = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("invalid BUSINESS_TIMEZONE %q, using Asia/Shanghai", name)
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	return loc
}
func BusinessNow() time.Time { return time.Now().In(BusinessLocation()) }

func nextDailyRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// StartScheduler owns cancellable daily jobs. It does not run catch-up mutations
// on startup; API triggers remain available for an intentional rerun.
func StartScheduler(ctx context.Context) {
	if os.Getenv("ENABLE_CRON") == "false" {
		log.Print("[cron] disabled")
		return
	}
	loc := BusinessLocation()
	job := func(hour int, name string, run func(context.Context, time.Time) error) {
		go func() {
			for {
				now := time.Now().In(loc)
				next := nextDailyRun(now, hour)
				timer := time.NewTimer(time.Until(next))
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				err := run(jobCtx, time.Now().In(loc))
				cancel()
				if err != nil {
					log.Printf("[cron] %s failed: %v", name, err)
				} else {
					log.Printf("[cron] %s completed", name)
				}
			}
		}()
	}
	job(1, "attendance", func(ctx context.Context, now time.Time) error {
		_, err := repo.GenerateDefaultAttendance(ctx, now.AddDate(0, 0, -1).Format("2006-01-02"))
		return err
	})
	job(2, "level-check and monthly aggregation", func(ctx context.Context, now time.Time) error {
		if _, err := RunDailyLevelCheck(ctx, now); err != nil {
			return err
		}
		day := now.AddDate(0, 0, -1)
		_, err := repo.AggregateMonthlyPerformance(ctx, day.Year(), int(day.Month()), nil)
		return err
	})
	log.Printf("[cron] daily attendance 01:00, level check and aggregation 02:00 (%s)", loc)
}
