package invoice

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingGateway struct {
	pdfs   int
	emails int
}

func (g *recordingGateway) GeneratePDF(context.Context, string, string) (json.RawMessage, error) {
	g.pdfs++
	return json.RawMessage(`{}`), nil
}

func (g *recordingGateway) SendEmail(context.Context, string, string, string, string) (json.RawMessage, error) {
	g.emails++
	return json.RawMessage(`{"message_id":"msg_123"}`), nil
}

func TestCreateInvoiceEmailDecision(t *testing.T) {
	tests := []struct {
		name     string
		status   DispatchStatus
		followUp bool
		wantSent bool
	}{
		{name: "completed follow-up", status: DispatchCompleted, followUp: true, wantSent: true},
		{name: "completed without follow-up", status: DispatchCompleted, followUp: false, wantSent: false},
		{name: "active follow-up", status: DispatchInProgress, followUp: true, wantSent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &recordingGateway{}
			service := NewService(gateway)
			result, err := service.CreateInvoice(context.Background(), WorkOrder{
				OrderID: "WO-1042", CustomerEmail: "customer@example.com",
				DispatchStatus: tt.status, FollowUpRequired: tt.followUp,
				Items: []LineItem{{Description: "Pump inspection", Quantity: 1, UnitCents: 12500}},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.EmailSent != tt.wantSent {
				t.Fatalf("EmailSent = %v, want %v", result.EmailSent, tt.wantSent)
			}
			if gateway.emails != boolCount(tt.wantSent) {
				t.Fatalf("email calls = %d", gateway.emails)
			}
			if gateway.pdfs != 1 {
				t.Fatalf("PDF calls = %d, want 1", gateway.pdfs)
			}
		})
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
