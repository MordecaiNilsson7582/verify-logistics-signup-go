package logistics

import (
	"context"
	"testing"
)

type recordingUsers struct{ calls int }

func (r *recordingUsers) CreateUser(context.Context, Signup) error { r.calls++; return nil }

type recordingEmails struct{ calls int }

func (r *recordingEmails) SendVerification(context.Context, Signup) (string, error) {
	r.calls++
	return "msg-42", nil
}

func TestStartSignupStopsAtUnresolvedShipmentException(t *testing.T) {
	tests := []struct {
		name      string
		exception *Exception
		wantError bool
		wantSends int
	}{
		{name: "delivered shipment sends verification", wantSends: 1},
		{name: "customs exception holds verification", exception: &Exception{Code: "CUSTOMS_REVIEW"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &recordingUsers{}
			emails := &recordingEmails{}
			service := Service{Users: users, Emails: emails}
			result, err := service.StartSignup(context.Background(), Signup{
				Email: "receiver@example.com", Password: "pipeline-secret", ShipmentID: "SHP-204",
				LatestEvent: ShipmentEvent{Kind: "delivered"}, Exception: tt.exception,
			})
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, want error %v", err, tt.wantError)
			}
			if emails.calls != tt.wantSends {
				t.Fatalf("email sends = %d, want %d", emails.calls, tt.wantSends)
			}
			if users.calls != 0 {
				t.Fatalf("auth users created = %d, want 0", users.calls)
			}
			if !tt.wantError && result.MessageID != "msg-42" {
				t.Fatalf("message id = %q", result.MessageID)
			}
		})
	}
}
