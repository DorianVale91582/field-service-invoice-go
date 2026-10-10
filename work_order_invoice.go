package invoice

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
)

type DispatchStatus string

const (
	DispatchScheduled  DispatchStatus = "scheduled"
	DispatchInProgress DispatchStatus = "in_progress"
	DispatchCompleted  DispatchStatus = "completed"
)

type WorkOrderPhoto struct {
	Caption string `json:"caption"`
	URL     string `json:"url"`
}

type LineItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitCents   int    `json:"unit_cents"`
}

type WorkOrder struct {
	OrderID          string           `json:"order_id"`
	CustomerName     string           `json:"customer_name"`
	CustomerEmail    string           `json:"customer_email"`
	ServiceAddress   string           `json:"service_address"`
	TechnicianName   string           `json:"technician_name"`
	DispatchStatus   DispatchStatus   `json:"dispatch_status"`
	FollowUpRequired bool             `json:"follow_up_required"`
	Photos           []WorkOrderPhoto `json:"photos"`
	Items            []LineItem       `json:"items"`
}

type Result struct {
	PDF       json.RawMessage `json:"pdf"`
	Email     json.RawMessage `json:"email,omitempty"`
	EmailSent bool            `json:"email_sent"`
}

type gateway interface {
	GeneratePDF(context.Context, string, string) (json.RawMessage, error)
	SendEmail(context.Context, string, string, string, string) (json.RawMessage, error)
}

type Service struct{ gateway gateway }

func NewService(g gateway) *Service { return &Service{gateway: g} }

func (s *Service) CreateInvoice(ctx context.Context, order WorkOrder) (Result, error) {
	if err := validate(order); err != nil {
		return Result{}, err
	}
	html, err := renderInvoice(order)
	if err != nil {
		return Result{}, err
	}
	pdf, err := s.gateway.GeneratePDF(ctx, html, "invoice-pdf-"+order.OrderID)
	if err != nil {
		return Result{}, err
	}
	result := Result{PDF: pdf}
	if order.DispatchStatus == DispatchCompleted && order.FollowUpRequired {
		email, err := s.gateway.SendEmail(ctx, order.CustomerEmail, "Invoice for work order "+order.OrderID, html, "invoice-email-"+order.OrderID)
		if err != nil {
			return Result{}, err
		}
		result.Email = email
		result.EmailSent = true
	}
	return result, nil
}

func validate(order WorkOrder) error {
	if strings.TrimSpace(order.OrderID) == "" || strings.TrimSpace(order.CustomerEmail) == "" {
		return fmt.Errorf("order_id and customer_email are required")
	}
	if len(order.Items) == 0 {
		return fmt.Errorf("at least one line item is required")
	}
	return nil
}

var invoiceTemplate = template.Must(template.New("invoice").Funcs(template.FuncMap{
	"money":     func(cents int) string { return fmt.Sprintf("$%.2f", float64(cents)/100) },
	"lineTotal": func(item LineItem) int { return item.Quantity * item.UnitCents },
}).Parse(`<!doctype html><html><head><meta charset="utf-8"><style>body{font:14px sans-serif;color:#17202a}table{width:100%;border-collapse:collapse}th,td{padding:8px;border-bottom:1px solid #ccd1d1;text-align:left}.photos img{max-width:180px;max-height:120px;margin:6px}</style></head><body><h1>Invoice {{.OrderID}}</h1><p>{{.CustomerName}}<br>{{.ServiceAddress}}</p><p>Dispatch: {{.DispatchStatus}}<br>Technician: {{.TechnicianName}}</p><table><tr><th>Service</th><th>Qty</th><th>Unit</th><th>Total</th></tr>{{range .Items}}<tr><td>{{.Description}}</td><td>{{.Quantity}}</td><td>{{money .UnitCents}}</td><td>{{money (lineTotal .)}}</td></tr>{{end}}</table>{{if .Photos}}<h2>Work photos</h2><div class="photos">{{range .Photos}}<figure><img src="{{.URL}}" alt="{{.Caption}}"><figcaption>{{.Caption}}</figcaption></figure>{{end}}</div>{{end}}{{if .FollowUpRequired}}<p>Technician follow-up requested.</p>{{end}}</body></html>`))

func renderInvoice(order WorkOrder) (string, error) {
	var out strings.Builder
	if err := invoiceTemplate.Execute(&out, order); err != nil {
		return "", fmt.Errorf("render invoice: %w", err)
	}
	return out.String(), nil
}
