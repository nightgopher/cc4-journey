package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nightgopher/cc4-journey/app/internal/domain"
)

var receiptTags = regexp.MustCompile(`<[^>]*>`)
var receiptNumber = regexp.MustCompile(`(?i)(?:发票号码|发票号|票据号码|收据编号|Invoice\s*(?:No\.?|Number))[\s:：#]*([A-Z0-9][A-Z0-9-]{3,63})`)
var receiptCode = regexp.MustCompile(`(?:发票代码|票据代码)[\s:：]*([0-9]{8,20})`)
var receiptDate = regexp.MustCompile(`(20[0-9]{2})[-/年]([0-9]{1,2})[-/月]([0-9]{1,2})日?`)
var receiptMerchant = regexp.MustCompile(`(?i)(?:Merchant|Vendor|Seller(?:\s*Name)?|销售方名称|销方名称|收款单位|商户名称|商家名称|开票单位|店名)[\s:：]+(.+)`)
var receiptTotal = regexp.MustCompile(`(?i)(?:价税合计|合计金额|总金额|^\s*(?:GRAND\s*)?TOTAL(?:\s*AMOUNT)?|^\s*AMOUNT\s*PAID)[^0-9\n]{0,35}([0-9][0-9,]*\.[0-9]{2})(?:[^0-9]|$)`)
var receiptForeign = regexp.MustCompile(`(?i)\b(USD|EUR|HKD|JPY|GBP)\b`)

func ExtractReceiptFields(text string) (domain.ReceiptFields, []string) {
	text = receiptTags.ReplaceAllString(strings.NewReplacer("|", " ", "**", "", "<br>", "\n", "<br/>", "\n", "\u00a0", " ").Replace(text), "")
	f := domain.ReceiptFields{Currency: "CNY", Category: "other"}
	warnings := []string{}
	numbers := receiptNumber.FindAllStringSubmatch(text, -1)
	if len(numbers) > 0 {
		f.InvoiceNumber = strings.ToUpper(numbers[0][1])
		for _, n := range numbers[1:] {
			if strings.ToUpper(n[1]) != f.InvoiceNumber {
				warnings = append(warnings, "发现多个票据号码，请确认一单一票或拆分上传")
			}
		}
	}
	if m := receiptCode.FindStringSubmatch(text); len(m) > 1 {
		f.InvoiceCode = m[1]
	}
	seller := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "购买方") {
			seller = false
		}
		if strings.Contains(line, "销售方") {
			seller = true
		}
		if m := receiptMerchant.FindStringSubmatch(line); len(m) > 1 && f.Merchant == "" {
			f.Merchant = strings.TrimSpace(m[1])
		} else if seller && f.Merchant == "" {
			for _, prefix := range []string{"名称：", "名称:"} {
				if _, value, ok := strings.Cut(line, prefix); ok {
					f.Merchant = strings.TrimSpace(value)
				}
			}
		}
		if f.InvoiceDate == "" {
			if m := receiptDate.FindStringSubmatch(line); len(m) == 4 {
				y, _ := strconv.Atoi(m[1])
				month, _ := strconv.Atoi(m[2])
				day, _ := strconv.Atoi(m[3])
				candidate := fmt.Sprintf("%04d-%02d-%02d", y, month, day)
				if _, err := time.Parse("2006-01-02", candidate); err == nil {
					f.InvoiceDate = candidate
				}
			}
		}
	}
	amounts := map[float64]bool{}
	for _, line := range strings.Split(text, "\n") {
		if m := receiptTotal.FindStringSubmatch(line); len(m) > 1 {
			if value, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64); err == nil && value > 0 && value < 1e10 {
				amounts[value] = true
			}
		}
	}
	if len(amounts) == 1 {
		for value := range amounts {
			n := value
			f.Total = &n
		}
	}
	if len(amounts) > 1 {
		warnings = append(warnings, "发现不同合计金额，未自动选择，请人工核对")
	}
	if m := receiptForeign.FindString(text); m != "" {
		f.Currency = strings.ToUpper(m)
		warnings = append(warnings, "当前报销仅支持人民币，请核对票据币种")
	}
	lower := strings.ToLower(text)
	for _, group := range []struct {
		category string
		words    []string
	}{
		{"office", []string{"办公", "stationery", "office"}}, {"transport", []string{"出租", "停车", "taxi", "transport"}},
		{"travel", []string{"酒店", "机票", "hotel", "flight"}}, {"meals", []string{"餐饮", "restaurant", "meal"}},
	} {
		for _, word := range group.words {
			if strings.Contains(lower, word) && f.Category == "other" {
				f.Category = group.category
			}
		}
	}
	for _, field := range []struct{ label, value string }{{"商家", f.Merchant}, {"票据号码", f.InvoiceNumber}, {"票据日期", f.InvoiceDate}} {
		if field.value == "" {
			warnings = append(warnings, "未可靠识别"+field.label+"，请补充")
		}
	}
	if f.Total == nil {
		warnings = append(warnings, "未可靠识别合计金额，请补充")
	}
	if len([]rune(f.Merchant)) > 200 {
		f.Merchant = ""
		warnings = append(warnings, "商家识别内容过长，请人工补充名称")
	}
	return f, warnings
}
