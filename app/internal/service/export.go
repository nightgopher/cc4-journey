package service

import "context"

// ActiveEmployeeIDs 供批量计算与 cron 共用。
func ActiveEmployeeIDs(ctx context.Context) ([]int, error) {
	return activeEmployeeIDs(ctx)
}
