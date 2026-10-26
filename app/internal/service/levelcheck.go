package service

import (
	"context"
	"fmt"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/domain"
	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

// RunDailyLevelCheck 对齐存量 cron/employeeLevelCheck.js：
// 遍历全部活跃员工 → 判定 → 生成审批单（条件满足日记为昨天）。
// 实习生转正保持自动（无审批），与旧行为一致。
func RunDailyLevelCheck(ctx context.Context, now time.Time) ([]map[string]any, error) {
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")

	// 活跃员工清单（对齐旧 cron 查询）。
	ids, err := activeEmployeeIDs(ctx)
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, id := range ids {
		emp, err := repo.GetEmployeeByID(ctx, id)
		if err != nil {
			continue // 单个员工失败不阻塞整体（对齐旧 cron 的 per-employee try/catch）
		}

		switch emp.Level {
		case domain.LevelTrainee:
			// 满 3 个工作日自动转 C（无审批），对齐旧 promoteTraineeToCLevel。
			joinDays, err := workdaysSinceJoin(ctx, id, now)
			if err == nil && joinDays >= domain.CPromotionWindow {
				if err := autoPromoteTrainee(ctx, id, now); err == nil {
					results = append(results, map[string]any{
						"employee_id": id, "action": "auto_promoted_to_c", "date": yesterday,
					})
				}
			}

		case domain.LevelC, domain.LevelB, domain.LevelA:
			r, err := CheckLevelTransition(ctx, id, now)
			if err != nil || r == nil || r.Action == domain.TransitionKeep {
				continue
			}
			// 生成审批单（对齐旧 createSystemApprovalRequest）。
			requestType := map[string]string{
				domain.TransitionPendingPromotion:   "promotion",
				domain.TransitionPendingElimination: "elimination",
				domain.TransitionPendingDemotion:    "demotion",
			}[r.Action]
			metadata := map[string]any{
				"employee_id":        id,
				"from_level":         emp.Level,
				"to_level":           r.Target,
				"reason":             r.Reason,
				"condition_met_date": yesterday,
			}
			title := fmt.Sprintf("%s 等级变更申请（%s → %s）", emp.Name, emp.Level, r.Target)
			_, err = repo.InsertApprovalRequest(ctx, &repo.ApprovalRequest{
				RequestType: requestType,
				Title:       title,
				Priority:    "high",
				RequestedBy: id,
				Status:      "pending",
				Metadata:    metadata,
			})
			if err == nil {
				repo.WriteAudit(ctx, "cron", "approval_created",
					fmt.Sprintf("employee=%d type=%s %s", id, requestType, r.Reason))
				results = append(results, map[string]any{
					"employee_id": id, "action": requestType, "date": yesterday, "reason": r.Reason,
				})
			}
		}
	}
	return results, nil
}

func activeEmployeeIDs(ctx context.Context) ([]int, error) {
	rows, err := repo.Pool.Query(ctx, `SELECT id FROM employees WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func workdaysSinceJoin(ctx context.Context, employeeID int, now time.Time) (int, error) {
	emp, err := repo.GetEmployeeByID(ctx, employeeID)
	if err != nil || emp.JoinDate == nil {
		return 0, fmt.Errorf("无入职日期")
	}
	days, err := repo.WorkdaysUpTo(ctx, *emp.JoinDate, now)
	if err != nil {
		return 0, err
	}
	return len(days), nil
}

// autoPromoteTrainee 实习生自动转 C 级：写 level_history + 更新 employees.level。
// 单事务，对齐旧行为（自动、无审批）。
func autoPromoteTrainee(ctx context.Context, employeeID int, now time.Time) error {
	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO employee_level_history (employee_id, from_level, to_level, change_date, reason)
		VALUES ($1, $2, $3, $4, '入职满3个工作日，自动转正')`,
		employeeID, domain.LevelTrainee, domain.LevelC, now.Format("2006-01-02")); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE employees SET level = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1`,
		employeeID, domain.LevelC); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
