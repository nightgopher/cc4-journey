package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/nightgopher/cc4-journey/app/internal/domain"
)

var ErrClaimConflict = errors.New("报销状态冲突")

type ClaimActor struct {
	ID     int
	Role   string
	TeamID *int
}

func (a ClaimActor) Global() bool { return a.Role == "admin" || a.Role == "finance" }
func (a ClaimActor) Reviewer() bool {
	return a.Global() || a.Role == "manager" || a.Role == "workplace_manager"
}
func (a ClaimActor) CanRead(employee int, team *int) bool {
	return a.ID == employee || a.Global() || (a.Reviewer() && a.TeamID != nil && team != nil && *a.TeamID == *team)
}

const claimProjection = `r.id,r.employee_id,e.name AS employee_name,e.team_id,r.title,r.category,r.merchant,r.invoice_number,r.invoice_code,
 TO_CHAR(r.invoice_date,'YYYY-MM-DD') AS invoice_date,r.currency,r.invoice_total,r.amount,r.reason,r.human_edited,r.review_note,
 r.status,r.receipt_mime,r.receipt_pages,r.ocr_status,r.ocr_model,r.ocr_progress,r.ocr_result,r.ocr_error,r.confirmed_at,
 r.submitted_at,r.approved_by,r.approved_at,r.paid_by,r.paid_at,TO_CHAR(r.payment_date,'YYYY-MM-DD') AS payment_date,r.finance_record_id,
 r.created_at,r.updated_at,'/reimbursements/'||r.id||'/receipt' AS receipt_url`

func Claim(ctx context.Context, a ClaimActor, id int) (Object, error) {
	row, err := One(ctx, Pool, `SELECT `+claimProjection+` FROM reimbursement_claims r JOIN employees e ON e.id=r.employee_id WHERE r.id=$1`, id)
	if err != nil {
		return nil, err
	}
	var team *int
	if n, ok := row["team_id"].(float64); ok {
		value := int(n)
		team = &value
	}
	if !a.CanRead(int(row["employee_id"].(float64)), team) {
		return nil, ErrForbidden
	}
	return row, nil
}

func Claims(ctx context.Context, a ClaimActor, scope, status string, limit, offset int) ([]Object, int, error) {
	where, args := "true", []any{}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if scope == "mine" || !a.Reviewer() {
		where = "r.employee_id=" + add(a.ID)
	} else if !a.Global() {
		if a.TeamID == nil {
			where = "r.employee_id=" + add(a.ID)
		} else {
			where = "(r.employee_id=" + add(a.ID) + " OR e.team_id=" + add(*a.TeamID) + ")"
		}
	}
	if scope == "review" {
		where += " AND r.employee_id!=" + add(a.ID)
		if a.Global() {
			where += " AND r.status IN ('submitted','approved')"
		} else {
			where += " AND r.status='submitted'"
		}
	}
	if status != "" {
		where += " AND r.status=" + add(status)
	}
	var count int
	if err := Pool.QueryRow(ctx, `SELECT count(*) FROM reimbursement_claims r JOIN employees e ON e.id=r.employee_id WHERE `+where, args...).Scan(&count); err != nil {
		return nil, 0, err
	}
	rows, err := Objects(ctx, Pool, `SELECT `+claimProjection+` FROM reimbursement_claims r JOIN employees e ON e.id=r.employee_id WHERE `+where+` ORDER BY r.id DESC LIMIT `+add(limit)+` OFFSET `+add(offset), args...)
	return rows, count, err
}

func claimEvent(ctx context.Context, tx DBTX, id int, actor *int, action, note string, details any) error {
	if details == nil {
		details = Object{}
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO reimbursement_events(claim_id,actor_id,action,note,details) VALUES($1,$2,$3,$4,$5)`, id, actor, action, note, raw)
	return err
}

func CreateClaim(ctx context.Context, a ClaimActor, file, mime, hash, model string, configured bool) (int, error) {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	status := "queued"
	var note *string
	if !configured {
		status = "failed"
		message := "识别服务尚未配置，可先人工填写或稍后重试"
		note = &message
	}
	var id int
	err = tx.QueryRow(ctx, `INSERT INTO reimbursement_claims(employee_id,receipt_file,receipt_mime,receipt_hash,ocr_model,ocr_status,ocr_error)
 VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, a.ID, file, mime, hash, model, status, note).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err = claimEvent(ctx, tx, id, &a.ID, "uploaded", "上传票据，等待识别及人工核对", nil); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

type ClaimInput struct {
	Title         string   `json:"title"`
	Category      string   `json:"category"`
	Merchant      string   `json:"merchant"`
	InvoiceNumber string   `json:"invoiceNumber"`
	InvoiceCode   string   `json:"invoiceCode"`
	InvoiceDate   string   `json:"invoiceDate"`
	Currency      string   `json:"currency"`
	InvoiceTotal  *float64 `json:"invoiceTotal"`
	Amount        *float64 `json:"amount"`
	Reason        string   `json:"reason"`
}

func InvoiceKey(code, number string) *string {
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				return unicode.ToUpper(r)
			}
			return -1
		}, s)
	}
	if number = clean(number); number == "" {
		return nil
	}
	value := clean(code) + ":" + number
	return &value
}

func (in *ClaimInput) validate() error {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		in.Title = "票据报销"
	}
	in.Merchant = strings.TrimSpace(in.Merchant)
	in.InvoiceNumber = strings.TrimSpace(in.InvoiceNumber)
	in.InvoiceCode = strings.TrimSpace(in.InvoiceCode)
	in.Currency = strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Currency == "" {
		in.Currency = "CNY"
	}
	if !strings.Contains("|office|transport|travel|meals|other|", "|"+in.Category+"|") || in.Category == "" {
		return errors.New("费用类别无效")
	}
	for _, f := range []struct {
		s   string
		max int
	}{{in.Title, 160}, {in.Merchant, 200}, {in.InvoiceNumber, 64}, {in.InvoiceCode, 64}, {in.Reason, 2000}, {in.Currency, 8}} {
		if utf8.RuneCountInString(f.s) > f.max {
			return errors.New("填写内容过长")
		}
	}
	for _, amount := range []*float64{in.InvoiceTotal, in.Amount} {
		if amount != nil && (!ValidMoney(*amount) && *amount != 0) {
			return errors.New("金额须为非负且最多两位小数")
		}
	}
	if in.InvoiceDate != "" {
		if _, err := time.Parse("2006-01-02", in.InvoiceDate); err != nil {
			return errors.New("票据日期无效")
		}
	}
	return nil
}

type lockedClaim struct {
	Owner                                    int
	Team                                     *int
	Status, OCRStatus                        string
	Amount, Total                            *float64
	Date                                     *time.Time
	Merchant, Number, Code, Currency, Reason string
	Finance                                  *int
}

func lockClaim(ctx context.Context, tx pgx.Tx, id int) (lockedClaim, error) {
	var r lockedClaim
	err := tx.QueryRow(ctx, `SELECT r.employee_id,e.team_id,r.status,r.ocr_status,r.amount,r.invoice_total,r.invoice_date,r.merchant,r.invoice_number,r.invoice_code,r.currency,r.reason,r.finance_record_id
 FROM reimbursement_claims r JOIN employees e ON e.id=r.employee_id WHERE r.id=$1 FOR UPDATE OF r`, id).Scan(&r.Owner, &r.Team, &r.Status, &r.OCRStatus, &r.Amount, &r.Total, &r.Date, &r.Merchant, &r.Number, &r.Code, &r.Currency, &r.Reason, &r.Finance)
	return r, err
}

func UpdateClaim(ctx context.Context, a ClaimActor, id int, in ClaimInput) error {
	if err := in.validate(); err != nil {
		return err
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	r, err := lockClaim(ctx, tx, id)
	if err != nil {
		return err
	}
	if r.Owner != a.ID {
		return ErrForbidden
	}
	if r.Status != "draft" && r.Status != "rejected" {
		return fmt.Errorf("%w: 只能修改草稿或被驳回的报销", ErrClaimConflict)
	}
	_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET title=$2,category=$3,merchant=$4,invoice_number=$5,invoice_code=$6,invoice_key=$7,
 invoice_date=NULLIF($8::text,'')::date,currency=$9,invoice_total=$10,amount=$11,reason=$12,human_edited=true,confirmed_at=NULL,status='draft',updated_at=now() WHERE id=$1`,
		id, in.Title, in.Category, in.Merchant, in.InvoiceNumber, in.InvoiceCode, InvoiceKey(in.InvoiceCode, in.InvoiceNumber), in.InvoiceDate, in.Currency, in.InvoiceTotal, in.Amount, in.Reason)
	if err != nil {
		return err
	}
	if err = claimEvent(ctx, tx, id, &a.ID, "updated", "人工核对并保存报销信息", in); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func DecideClaim(ctx context.Context, a ClaimActor, id int, action, note, paymentDate, today string, confirmed bool) error {
	if utf8.RuneCountInString(note) > 2000 {
		return errors.New("说明过长")
	}
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	r, err := lockClaim(ctx, tx, id)
	if err != nil {
		return err
	}
	if !a.CanRead(r.Owner, r.Team) {
		return ErrForbidden
	}
	owner := r.Owner == a.ID
	conflict := func(message string) error { return fmt.Errorf("%w: %s", ErrClaimConflict, message) }
	switch action {
	case "submit":
		if !owner {
			return ErrForbidden
		}
		if r.Status != "draft" && r.Status != "rejected" {
			return conflict("当前状态不能提交")
		}
		if !confirmed || strings.TrimSpace(r.Merchant) == "" || InvoiceKey(r.Code, r.Number) == nil || strings.TrimSpace(r.Reason) == "" || r.Date == nil || r.Currency != "CNY" || r.Amount == nil || r.Total == nil || !ValidMoney(*r.Amount) || !ValidMoney(*r.Total) || math.Round(*r.Amount*100) > math.Round(*r.Total*100) {
			return errors.New("请核对商家、票号、日期、用途与人民币金额，确认申请金额不超过票面金额，并勾选人工确认")
		}
		if r.Date.Format("2006-01-02") > today {
			return errors.New("票据日期不能晚于当前业务日")
		}
		if r.OCRStatus == "queued" || r.OCRStatus == "submitting" || r.OCRStatus == "running" {
			return conflict("识别仍在进行，请完成后核对再提交")
		}
		_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET status='submitted',confirmed_at=now(),submitted_at=now(),invoice_key=$2,review_note='',updated_at=now() WHERE id=$1`, id, InvoiceKey(r.Code, r.Number))
	case "approve", "reject":
		if owner || !a.Reviewer() {
			return ErrForbidden
		}
		if r.Status != "submitted" && !(action == "reject" && r.Status == "approved" && a.Global()) {
			return conflict("当前状态不能审批")
		}
		if action == "reject" && strings.TrimSpace(note) == "" {
			return errors.New("请填写驳回原因")
		}
		if action == "approve" {
			_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET status='approved',approved_by=$2,approved_at=now(),review_note=$3,updated_at=now() WHERE id=$1`, id, a.ID, note)
		} else {
			_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET status='rejected',approved_by=NULL,approved_at=NULL,review_note=$2,updated_at=now() WHERE id=$1`, id, note)
		}
	case "withdraw", "cancel":
		if !owner {
			return ErrForbidden
		}
		if action == "withdraw" {
			if r.Status != "submitted" {
				return conflict("只能撤回待审批报销")
			}
			_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET status='draft',confirmed_at=NULL,updated_at=now() WHERE id=$1`, id)
		} else {
			if r.Status != "draft" && r.Status != "rejected" && r.Status != "submitted" {
				return conflict("已批准或已付款报销不能撤销")
			}
			_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET status='cancelled',ocr_status='failed',ocr_error='报销已撤销',ocr_generation=ocr_generation+1,ocr_lease=NULL,ocr_locked_until=NULL,updated_at=now() WHERE id=$1`, id)
		}
	case "pay":
		if owner || !a.Global() {
			return ErrForbidden
		}
		if r.Status == "paid" {
			return nil
		}
		if r.Status != "approved" {
			return conflict("只有已批准报销可以付款")
		}
		if _, err = time.Parse("2006-01-02", paymentDate); err != nil || paymentDate > today || (r.Date != nil && paymentDate < r.Date.Format("2006-01-02")) {
			return errors.New("付款日期须在票据日期与当前业务日之间")
		}
		var finance int
		err = tx.QueryRow(ctx, `INSERT INTO finance_records(type,category,amount,transaction_date,description,reference_id,reference_type,recorded_by,status,source,cost_type)
 SELECT 'expense','员工报销',amount,$2::text::date,'报销#'||id||' · '||merchant||' · '||reason,'reimbursement-'||id,'reimbursement',employee_id,'confirmed','auto','onetime'
 FROM reimbursement_claims WHERE id=$1 RETURNING id`, id, paymentDate).Scan(&finance)
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET status='paid',finance_record_id=$2,paid_by=$3,paid_at=now(),payment_date=$4::text::date,updated_at=now() WHERE id=$1`, id, finance, a.ID, paymentDate)
		}
	default:
		return errors.New("无效的报销操作")
	}
	if err != nil {
		return err
	}
	if err = claimEvent(ctx, tx, id, &a.ID, action, note, Object{"paymentDate": paymentDate}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ClaimEvents(ctx context.Context, id int) ([]Object, error) {
	return Objects(ctx, Pool, `SELECT v.id,v.action,v.note,v.details,v.created_at,e.name AS actor_name FROM reimbursement_events v LEFT JOIN employees e ON e.id=v.actor_id WHERE v.claim_id=$1 ORDER BY v.id`, id)
}

func ClaimFile(ctx context.Context, a ClaimActor, id int) (string, string, error) {
	if _, err := Claim(ctx, a, id); err != nil {
		return "", "", err
	}
	var file, mime string
	err := Pool.QueryRow(ctx, `SELECT receipt_file,receipt_mime FROM reimbursement_claims WHERE id=$1`, id).Scan(&file, &mime)
	return file, mime, err
}

func RetryClaimOCR(ctx context.Context, a ClaimActor, id int, model string) error {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	r, err := lockClaim(ctx, tx, id)
	if err != nil {
		return err
	}
	if r.Owner != a.ID {
		return ErrForbidden
	}
	if (r.Status != "draft" && r.Status != "rejected") || (r.OCRStatus != "failed" && r.OCRStatus != "ready") {
		return fmt.Errorf("%w: 当前不能重新识别", ErrClaimConflict)
	}
	_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET ocr_status='queued',ocr_model=$2,ocr_generation=ocr_generation+1,ocr_job_id=NULL,ocr_result=NULL,ocr_error=NULL,ocr_attempts=0,
 ocr_lease=NULL,ocr_locked_until=NULL,ocr_started_at=NULL,ocr_next_poll=now(),ocr_progress='{}',updated_at=now() WHERE id=$1`, id, model)
	if err != nil {
		return err
	}
	if err = claimEvent(ctx, tx, id, &a.ID, "ocr_retry", "重新提交票据识别", nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// OCRJob holds only server-side coordinates. It is never returned to the browser.
type OCRJob struct {
	ID, Generation, Attempts int
	File, Mime, Model, State string
	JobID                    *string
	Started                  *time.Time
	Lease                    string
}

func AcquireOCRJob(ctx context.Context, lease string) (OCRJob, error) {
	var j OCRJob
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return j, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,ocr_generation,receipt_file,receipt_mime,ocr_model,ocr_status,ocr_job_id,ocr_attempts,ocr_started_at FROM reimbursement_claims
 WHERE ocr_status IN ('queued','submitting','running') AND status!='cancelled' AND ocr_next_poll<=now()
 AND (ocr_locked_until IS NULL OR ocr_locked_until<now()) ORDER BY ocr_next_poll,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&j.ID, &j.Generation, &j.File, &j.Mime, &j.Model, &j.State, &j.JobID, &j.Attempts, &j.Started)
	if err != nil {
		return j, err
	}
	j.Lease = lease
	_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET ocr_lease=$2,ocr_locked_until=now()+interval '75 seconds',ocr_started_at=COALESCE(ocr_started_at,now()),
 ocr_status=CASE WHEN ocr_status='queued' THEN 'submitting' ELSE ocr_status END WHERE id=$1`, j.ID, lease)
	if err != nil {
		return j, err
	}
	return j, tx.Commit(ctx)
}

func SaveOCRJob(ctx context.Context, j OCRJob, state string, jobID *string, progress any, result *domain.ReceiptRecognition, message string) error {
	tx, err := Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var rawResult any
	if result != nil {
		rawResult, err = json.Marshal(result)
		if err != nil {
			return err
		}
	}
	if progress == nil {
		progress = Object{}
	}
	rawProgress, _ := json.Marshal(progress)
	tag, err := tx.Exec(ctx, `UPDATE reimbursement_claims SET ocr_status=$4,ocr_job_id=COALESCE($5,ocr_job_id),ocr_progress=$6,ocr_result=COALESCE($7::jsonb,ocr_result),
 ocr_error=NULLIF($8,''),ocr_attempts=CASE WHEN $8!='' THEN ocr_attempts+1 ELSE 0 END,ocr_next_poll=now()+interval '5 seconds',ocr_lease=NULL,ocr_locked_until=NULL,updated_at=now()
 WHERE id=$1 AND ocr_generation=$2 AND ocr_lease=$3`, j.ID, j.Generation, j.Lease, state, jobID, rawProgress, rawResult, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil
	}
	if result != nil {
		f := result.Fields
		_, err = tx.Exec(ctx, `UPDATE reimbursement_claims SET merchant=$2,invoice_number=$3,invoice_code=$4,invoice_date=NULLIF($5::text,'')::date,currency=$6,invoice_total=$7,amount=$7,category=$8
 WHERE id=$1 AND NOT human_edited AND status IN ('draft','rejected')`, j.ID, f.Merchant, f.InvoiceNumber, f.InvoiceCode, f.InvoiceDate, f.Currency, f.Total, f.Category)
		if err != nil {
			return err
		}
	}
	if state == "ready" || state == "failed" {
		if err = claimEvent(ctx, tx, j.ID, nil, "ocr_"+state, message, Object{"model": j.Model}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
