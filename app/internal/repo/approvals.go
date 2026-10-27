package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func DecideApproval(ctx context.Context, id, actor int, admin bool, action, comments string, next *int) error {
	if action != "approved" && action != "rejected" && action != "forwarded" {
		return fmt.Errorf("审批动作无效")
	}
	if action == "forwarded" && (next == nil || *next <= 0) {
		return fmt.Errorf("转交必须指定审批人")
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var status, kind string
	var requester int
	var approver *int
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT status,request_type,requested_by,current_approver,COALESCE(metadata,'{}'::jsonb) FROM approval_requests WHERE id=$1 FOR UPDATE`, id).Scan(&status, &kind, &requester, &approver, &raw)
	if err != nil {
		return err
	}
	if status != "pending" {
		return fmt.Errorf("审批请求已处理")
	}
	if (approver == nil && !admin) || (approver != nil && *approver != actor) {
		return fmt.Errorf("%w: 当前用户不是指定审批人", ErrForbidden)
	}
	metadata := Object{}
	if err = json.Unmarshal(raw, &metadata); err != nil {
		return err
	}
	if action == "approved" {
		switch kind {
		case "promotion", "demotion", "elimination":
			var level string
			if err = tx.QueryRow(ctx, `SELECT level FROM employees WHERE id=$1 FOR UPDATE`, requester).Scan(&level); err != nil {
				return err
			}
			from, _ := metadata["from_level"].(string)
			to, _ := metadata["to_level"].(string)
			reason, _ := metadata["reason"].(string)
			date, _ := metadata["condition_met_date"].(string)
			if from != "" && from != level {
				return fmt.Errorf("员工等级已变化，请重新发起审批")
			}
			if kind == "elimination" {
				_, err = tx.Exec(ctx, `UPDATE employees SET status='inactive',updated_at=CURRENT_TIMESTAMP WHERE id=$1`, requester)
				to = level
				reason = "淘汰: " + reason
			} else {
				_, err = tx.Exec(ctx, `UPDATE employees SET level=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, requester, to)
			}
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO employee_level_history(employee_id,from_level,to_level,change_date,reason,approved_by,condition_met_date)
			VALUES($1,$2,$3,COALESCE(NULLIF($4::text,'')::date,CURRENT_DATE),$5,$6,NULLIF($4::text,'')::date)`, requester, level, to, date, reason, actor)
			if err != nil {
				return err
			}
		case "leave":
			start, _ := metadata["start_date"].(string)
			end, _ := metadata["end_date"].(string)
			leaveType, _ := metadata["leave_type"].(string)
			_, err = tx.Exec(ctx, `INSERT INTO attendance_records(employee_id,date,status,attendance_days,leave_type,notes,source)
			SELECT $1,generate_series($2::text::date,$3::text::date,'1 day'::interval)::date,'absent',0,$4,$5,'leave'
			ON CONFLICT(employee_id,date)DO UPDATE SET status='absent',attendance_days=0,leave_type=EXCLUDED.leave_type,notes=EXCLUDED.notes,source='leave',updated_at=CURRENT_TIMESTAMP`, requester, start, end, leaveType, comments)
			if err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO approval_records(request_id,approver_id,action,comments,next_approver_id)VALUES($1,$2,$3,$4,$5)`, id, actor, action, comments, next); err != nil {
		return err
	}
	newStatus := action
	var current *int
	if action == "forwarded" {
		newStatus = "pending"
		current = next
	}
	if _, err = tx.Exec(ctx, `UPDATE approval_requests SET status=$2,current_approver=$3,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, newStatus, current); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notifications(user_id,title,message,type) SELECT user_id,'审批结果通知',$2,'info' FROM employees WHERE id=$1 AND user_id IS NOT NULL`, requester, "申请已处理: "+newStatus); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func PendingApproval(ctx context.Context, id int) (Object, error) {
	return One(ctx, Pool, `SELECT * FROM approval_requests WHERE id=$1`, id)
}

// EnsureSystemApproval prevents repeated cron invocations from creating duplicate pending requests.
func EnsureSystemApproval(ctx context.Context, a *ApprovalRequest) (int, bool, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)
	key := fmt.Sprintf("approval:%d:%s", a.RequestedBy, a.RequestType)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return 0, false, err
	}
	var id int
	err = tx.QueryRow(ctx, `SELECT id FROM approval_requests WHERE requested_by=$1 AND request_type=$2 AND status='pending' LIMIT 1`, a.RequestedBy, a.RequestType).Scan(&id)
	if err == nil {
		return id, false, tx.Commit(ctx)
	}
	if err != pgx.ErrNoRows {
		return 0, false, err
	}
	err = tx.QueryRow(ctx, `INSERT INTO approval_requests(request_type,title,priority,requested_by,status,metadata)VALUES($1,$2,$3,$4,'pending',$5)RETURNING id`, a.RequestType, a.Title, a.Priority, a.RequestedBy, a.Metadata).Scan(&id)
	if err != nil {
		return 0, false, err
	}
	return id, true, tx.Commit(ctx)
}
