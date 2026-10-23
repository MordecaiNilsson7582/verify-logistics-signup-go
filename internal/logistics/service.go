package logistics

import (
	"context"
	"fmt"
	"html"
)

// Signup is the signup record collected beside a shipment event.
type Signup struct {
	Email           string          `json:"email"`
	Password        string          `json:"password"`
	Name            string          `json:"name"`
	ShipmentID      string          `json:"shipment_id"`
	LatestEvent     ShipmentEvent   `json:"latest_event"`
	ProofOfDelivery ProofOfDelivery `json:"proof_of_delivery"`
	Exception       *Exception      `json:"exception,omitempty"`
}

type ShipmentEvent struct {
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurred_at"`
	Location   string `json:"location"`
}

type ProofOfDelivery struct {
	DocumentURL string `json:"document_url"`
	SignedBy    string `json:"signed_by"`
}

type Exception struct {
	Code    string `json:"code"`
	Details string `json:"details"`
}

type UserCreator interface {
	CreateUser(context.Context, Signup) error
}

type VerificationSender interface {
	SendVerification(context.Context, Signup) (string, error)
}

// Service keeps the operational decision close to the transport boundary.
type Service struct {
	Users  UserCreator
	Emails VerificationSender
}

type Result struct {
	MessageID string `json:"message_id"`
}

// StartSignup publishes verification without creating a persistent auth user.
func (s Service) StartSignup(ctx context.Context, signup Signup) (Result, error) {
	if signup.Email == "" || signup.Password == "" || signup.ShipmentID == "" {
		return Result{}, fmt.Errorf("email, password, and shipment_id are required")
	}
	if signup.Exception != nil && signup.Exception.Code != "" {
		return Result{}, fmt.Errorf("shipment exception must be resolved before verification")
	}
	messageID, err := s.Emails.SendVerification(ctx, signup)
	if err != nil {
		return Result{}, err
	}
	return Result{MessageID: messageID}, nil
}

func verificationHTML(signup Signup) string {
	shipment := html.EscapeString(signup.ShipmentID)
	event := html.EscapeString(signup.LatestEvent.Kind)
	return fmt.Sprintf("<p>Confirm the email for shipment <b>%s</b>.</p><p>Latest event: %s.</p><p><a href=\"https://portal.example/verify?shipment_id=%s\">Verify email</a></p>", shipment, event, shipment)
}
