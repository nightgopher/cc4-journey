package domain

// ReceiptFields are suggestions. A human must confirm them before submission.
type ReceiptFields struct {
	Merchant      string   `json:"merchant"`
	InvoiceNumber string   `json:"invoiceNumber"`
	InvoiceCode   string   `json:"invoiceCode"`
	InvoiceDate   string   `json:"invoiceDate"`
	Currency      string   `json:"currency"`
	Total         *float64 `json:"total"`
	Category      string   `json:"category"`
}

type ReceiptRecognition struct {
	Provider   string        `json:"provider"`
	Model      string        `json:"model"`
	Text       string        `json:"text"`
	Pages      int           `json:"pages"`
	Confidence *float64      `json:"confidence"`
	Fields     ReceiptFields `json:"fields"`
	Warnings   []string      `json:"warnings"`
}
