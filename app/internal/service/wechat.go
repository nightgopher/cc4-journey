package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/repo"
)

type WechatRecord struct {
	CallRecordID   string     `json:"call_record_id"`
	CustomerName   string     `json:"customer_name"`
	RealPhone      string     `json:"real_phone"`
	EncryptedPhone string     `json:"encrypted_phone"`
	CallStatus     string     `json:"call_status"`
	CallDuration   int        `json:"call_duration"`
	CallTime       *time.Time `json:"call_time"`
	AudioURL       string     `json:"audio_url"`
	City           string     `json:"city"`
	Province       string     `json:"province"`
	WechatAdded    bool       `json:"wechat_added"`
	AssignedTo     *int       `json:"assigned_to"`
	Notes          string     `json:"notes"`
	DataSource     string     `json:"data_source"`
}

func ImportWechatRecords(ctx context.Context, records []WechatRecord) (int, int, error) {
	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	saved, duplicate := 0, 0
	for _, r := range records {
		if r.RealPhone == "" || r.CallRecordID == "" {
			return 0, 0, fmt.Errorf("线索手机号和外呼记录编号不能为空")
		}
		source := r.DataSource
		if source != "mock" && source != "gooki_api" {
			source = "import"
		}
		tag, err := tx.Exec(ctx, `INSERT INTO wechat_leads(call_record_id,customer_name,real_phone,encrypted_phone,call_status,call_duration,call_time,audio_url,city,province,wechat_added,wechat_add_time,assigned_to,notes,status,data_source)
		VALUES($1,$2,$3,$4,$5,$6,$7::timestamptz,$8,$9,$10,$11::boolean,CASE WHEN $11::boolean THEN $7::timestamptz ELSE NULL END,$12,$13,'new',$14) ON CONFLICT(call_record_id)DO NOTHING`, r.CallRecordID, r.CustomerName, r.RealPhone, r.EncryptedPhone, r.CallStatus, r.CallDuration, r.CallTime, r.AudioURL, r.City, r.Province, r.WechatAdded, r.AssignedTo, r.Notes, source)
		if err != nil {
			return 0, 0, err
		}
		if tag.RowsAffected() == 1 {
			saved++
		} else {
			duplicate++
		}
	}
	return saved, duplicate, tx.Commit(ctx)
}
func WechatConfigured() bool {
	return os.Getenv("GOOKI_HOST") != "" && os.Getenv("GOOKI_USERNAME") != "" && os.Getenv("GOOKI_PASSWORD") != ""
}
func SyncWechat(ctx context.Context, start, end string, assignee *int) (map[string]any, error) {
	if len(start) < 10 || len(end) < 10 {
		return nil, fmt.Errorf("同步日期不能为空")
	}
	client := &http.Client{Timeout: 15 * time.Second}
	host := strings.TrimRight(os.Getenv("GOOKI_HOST"), "/")
	get := func(path, token string, params url.Values, dest any) error {
		req, err := http.NewRequestWithContext(ctx, "GET", host+path+"?"+params.Encode(), nil)
		if err != nil {
			return err
		}
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("外呼服务请求失败")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("外呼服务返回 %d", resp.StatusCode)
		}
		return json.NewDecoder(io.LimitReader(resp.Body, 10<<20)).Decode(dest)
	}
	var login struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := get("/token", "", url.Values{"username": {os.Getenv("GOOKI_USERNAME")}, "password": {os.Getenv("GOOKI_PASSWORD")}}, &login); err != nil {
		return nil, err
	}
	if login.Code != 200 {
		return nil, fmt.Errorf("外呼服务登录失败")
	}
	var token string
	if json.Unmarshal(login.Data, &token) != nil {
		var d struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(login.Data, &d); err != nil {
			return nil, err
		}
		token = d.Token
	}
	if token == "" {
		return nil, fmt.Errorf("外呼服务未返回访问令牌")
	}
	total, processed, saved, duplicates := 0, 0, 0, 0
	prefix := os.Getenv("GOOKI_FILTER_PREFIX")
	if prefix == "" {
		prefix = "AF3加微信成功挂断"
	}
	for page := 1; page <= 1000; page++ {
		var result struct {
			Code int  `json:"code"`
			Mock bool `json:"mock"`
			Data struct {
				TotalCount int `json:"totalCount"`
				Data       []struct {
					ID           json.Number `json:"id"`
					CustomerName string      `json:"customerName"`
					RealPhone    string      `json:"realPhone"`
					Phone        string      `json:"phone"`
					Grade        string      `json:"grade"`
					Duration     int         `json:"duration"`
					CallTime     int64       `json:"callTime"`
					Audio        string      `json:"audio"`
					City         string      `json:"city"`
					Province     string      `json:"province"`
				} `json:"data"`
			} `json:"data"`
		}
		if err := get("/task/cdr/all2", token, url.Values{"page": {strconv.Itoa(page)}, "pageSize": {"100"}, "startTime": {start}, "endTime": {end}}, &result); err != nil {
			return nil, err
		}
		if result.Code != 200 {
			return nil, fmt.Errorf("外呼服务同步失败")
		}
		if page == 1 {
			total = result.Data.TotalCount
		}
		records := []WechatRecord{}
		for _, r := range result.Data.Data {
			processed++
			if !strings.HasPrefix(r.Grade, prefix) {
				continue
			}
			when := time.Unix(r.CallTime, 0)
			name := strings.TrimSpace(r.CustomerName)
			if name == "" {
				name = "微信客户"
			}
			source := "gooki_api"
			if result.Mock {
				source = "mock"
			}
			records = append(records, WechatRecord{CallRecordID: r.ID.String(), CustomerName: name, RealPhone: r.RealPhone, EncryptedPhone: r.Phone, CallStatus: r.Grade, CallDuration: r.Duration, CallTime: &when, AudioURL: r.Audio, City: r.City, Province: r.Province, WechatAdded: true, AssignedTo: assignee, DataSource: source})
		}
		n, d, err := ImportWechatRecords(ctx, records)
		if err != nil {
			return nil, err
		}
		saved += n
		duplicates += d
		if len(result.Data.Data) < 100 || processed >= total {
			summary := map[string]any{"totalCount": total, "processedCount": processed, "savedCount": saved, "duplicateCount": duplicates, "startTime": start, "endTime": end, "mock": result.Mock}
			b, _ := json.Marshal(summary)
			category := "sync"
			key := "gooki_last_sync"
			if start[:10] == end[:10] {
				key = "gooki_summary_" + start[:10]
			}
			if err := repo.UpsertSetting(ctx, &repo.Setting{Key: key, Value: string(b), Category: &category}); err != nil {
				return nil, err
			}
			return summary, nil
		}
	}
	return nil, fmt.Errorf("外呼同步超过单次 100000 条限制，请缩小时间范围")
}
