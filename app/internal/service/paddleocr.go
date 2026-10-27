package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/domain"
)

const PaddleJobsURL = "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs"

var PaddleModels = []string{"PaddleOCR-VL-1.6", "PP-OCRv6", "PP-StructureV3"}
var paddleJobID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type PaddleClient struct {
	base, token, Model string
	API, Download      *http.Client
	allowLocal         bool
}
type PaddleProgress struct {
	State            string
	Total, Extracted int
	Recognition      *domain.ReceiptRecognition
}
type PaddleError struct {
	Message   string
	Retryable bool
}

func (e *PaddleError) Error() string { return e.Message }

func NewPaddleClient(base, token, model string, allowLocal bool) (*PaddleClient, error) {
	valid := false
	for _, candidate := range PaddleModels {
		valid = valid || model == candidate
	}
	if !valid {
		return nil, errors.New("不支持的OCR模型")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (!allowLocal && base != PaddleJobsURL) {
		return nil, errors.New("无效的OCR服务地址")
	}
	p := &PaddleClient{base: strings.TrimRight(base, "/"), token: token, Model: model, allowLocal: allowLocal}
	p.API = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	p.Download = &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !p.resultURLAllowed(req.URL) {
			return errors.New("不允许的OCR结果地址")
		}
		return nil
	}}
	return p, nil
}

func PaddleFromEnv() (*PaddleClient, error) {
	model := os.Getenv("PADDLEOCR_MODEL")
	if model == "" {
		model = PaddleModels[0]
	}
	return NewPaddleClient(PaddleJobsURL, os.Getenv("PADDLEOCR_API_KEY"), model, false)
}
func (p *PaddleClient) Configured() bool { return p != nil && strings.TrimSpace(p.token) != "" }

func (p *PaddleClient) api(ctx context.Context, method, suffix string, body io.Reader, contentType string) (map[string]json.RawMessage, error) {
	if !p.Configured() {
		return nil, &PaddleError{Message: "识别服务尚未配置，请联系管理员"}
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+suffix, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "bearer "+p.token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := p.API.Do(req)
	if err != nil {
		return nil, &PaddleError{Message: "识别服务连接失败或超时", Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := "识别服务请求失败，请稍后重试"
		if response.StatusCode == 401 || response.StatusCode == 403 {
			message = "识别服务认证失败，请联系管理员"
		}
		return nil, &PaddleError{Message: message, Retryable: response.StatusCode == 429 || response.StatusCode >= 500}
	}
	var result struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil || result.Data == nil {
		return nil, &PaddleError{Message: "识别服务返回格式异常"}
	}
	return result.Data, nil
}

func (p *PaddleClient) Submit(ctx context.Context, path string) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("model", p.Model)
	options := map[string]bool{"useDocOrientationClassify": false, "useDocUnwarping": false}
	if p.Model == "PP-OCRv6" {
		options["useTextlineOrientation"] = false
	} else {
		options["useChartRecognition"] = false
	}
	b, _ := json.Marshal(options)
	_ = w.WriteField("optionalPayload", string(b))
	file, err := os.Open(path)
	if err != nil {
		return "", errors.New("票据文件不可读取")
	}
	defer file.Close()
	part, err := w.CreateFormFile("file", "receipt"+filepath.Ext(path))
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(part, io.LimitReader(file, (8<<20)+1)); err != nil {
		return "", err
	}
	_ = w.Close()
	data, err := p.api(ctx, "POST", "", &body, w.FormDataContentType())
	if err != nil {
		return "", err
	}
	var id string
	if json.Unmarshal(data["jobId"], &id) != nil || !paddleJobID.MatchString(id) {
		return "", &PaddleError{Message: "识别服务没有返回有效任务编号，未自动重发"}
	}
	return id, nil
}

func (p *PaddleClient) Poll(ctx context.Context, id string) (PaddleProgress, error) {
	var result PaddleProgress
	if !paddleJobID.MatchString(id) {
		return result, errors.New("无效的识别任务编号")
	}
	data, err := p.api(ctx, "GET", "/"+id, nil, "")
	if err != nil {
		return result, err
	}
	_ = json.Unmarshal(data["state"], &result.State)
	var progress struct {
		Total     int `json:"totalPages"`
		Extracted int `json:"extractedPages"`
	}
	_ = json.Unmarshal(data["extractProgress"], &progress)
	result.Total, result.Extracted = progress.Total, progress.Extracted
	switch result.State {
	case "pending", "running":
		return result, nil
	case "failed":
		return result, &PaddleError{Message: "识别服务未能处理该票据，请重试或人工核对"}
	case "done":
		var target struct {
			URL string `json:"jsonUrl"`
		}
		_ = json.Unmarshal(data["resultUrl"], &target)
		u, err := url.Parse(target.URL)
		if err != nil || !p.resultURLAllowed(u) {
			return result, errors.New("识别结果下载地址不受支持")
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		// Separate client: never forward the supplier token to result storage.
		response, err := p.Download.Do(req)
		if err != nil {
			return result, &PaddleError{Message: "识别结果下载暂时失败", Retryable: true}
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return result, &PaddleError{Message: "识别结果下载失败", Retryable: response.StatusCode >= 500}
		}
		raw, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
		if err != nil || len(raw) > 8<<20 {
			return result, errors.New("识别结果超限或读取失败")
		}
		recognition, err := ParsePaddleJSONL(raw, p.Model)
		if err != nil {
			return result, err
		}
		result.Recognition = &recognition
		return result, nil
	default:
		return result, errors.New("识别服务返回未知任务状态")
	}
}

func (p *PaddleClient) resultURLAllowed(u *url.URL) bool {
	if u == nil || u.User != nil {
		return false
	}
	if p.allowLocal {
		base, _ := url.Parse(p.base)
		return u.Host == base.Host && u.Scheme == base.Scheme
	}
	host := strings.ToLower(u.Hostname())
	return u.Scheme == "https" && (u.Port() == "" || u.Port() == "443") && (host == "paddleocr.aistudio-app.com" || strings.HasSuffix(host, ".bcebos.com") || strings.HasSuffix(host, ".baidubce.com"))
}

func ParsePaddleJSONL(raw []byte, model string) (domain.ReceiptRecognition, error) {
	result := domain.ReceiptRecognition{Provider: "paddleocr", Model: model, Warnings: []string{}}
	chunks, seen := []string{}, map[string]bool{}
	var collect func(any)
	collect = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if markdown, ok := v["markdown"].(map[string]any); ok {
				if text, ok := markdown["text"].(string); ok && !seen[text] {
					chunks = append(chunks, text)
					seen[text] = true
				}
			}
			if texts, ok := v["rec_texts"].([]any); ok {
				for _, item := range texts {
					if text, ok := item.(string); ok && !seen[text] {
						chunks = append(chunks, text)
						seen[text] = true
					}
				}
			}
			for key, item := range v {
				if key != "markdown" && key != "rec_texts" && key != "outputImages" && key != "ocrImage" {
					collect(item)
				}
			}
		case []any:
			for _, item := range v {
				collect(item)
			}
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 8<<20)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		var row map[string]any
		if json.Unmarshal(scanner.Bytes(), &row) != nil || row["result"] == nil {
			return result, errors.New("识别结果JSONL格式异常")
		}
		collect(row["result"])
		result.Pages++
	}
	if scanner.Err() != nil {
		return result, errors.New("识别结果读取失败")
	}
	result.Text = strings.Join(chunks, "\n")
	if len(strings.TrimSpace(result.Text)) == 0 || len(result.Text) > 2<<20 {
		return result, errors.New("没有识别到有效文字或结果过长")
	}
	result.Fields, result.Warnings = ExtractReceiptFields(result.Text)
	result.Warnings = append(result.Warnings, "识别结果仅供填写参考，提交前请核对原始票据")
	return result, nil
}

func paddleErrorText(err error) string {
	var provider *PaddleError
	if errors.As(err, &provider) {
		return provider.Message
	}
	return fmt.Sprint(err)
}
