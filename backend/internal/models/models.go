// Package models holds the shared domain structs used across services and the
// wire format for Kafka consent events.
package models

import (
	"encoding/json"
	"time"
)

// Role is the authenticated subject type carried inside a JWT.
type Role string

const (
	RoleCitizen     Role = "citizen"
	RoleInstitution Role = "institution"
	RoleAdmin       Role = "admin"
)

// Citizen is the auth record for a registered citizen.
type Citizen struct {
	ID           string    `json:"id"`
	AadhaarID    string    `json:"aadhaarId"`
	FullName     string    `json:"fullName"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ConsentStatus describes the lifecycle of a consent request.
type ConsentStatus string

const (
	StatusPending  ConsentStatus = "pending"
	StatusApproved ConsentStatus = "approved"
	StatusRejected ConsentStatus = "rejected"
	StatusRevoked  ConsentStatus = "revoked"
	StatusExpired  ConsentStatus = "expired"
)

// ConsentRequest is a citizen consent decision record.
type ConsentRequest struct {
	ID            string    `json:"id"`
	CitizenID     string    `json:"citizenId"`
	InstitutionID string    `json:"institutionId"`
	DataScope     []string  `json:"dataScope"`
	Purpose       string    `json:"purpose"`
	Status        string    `json:"status"`
	AuthToken     string    `json:"authToken,omitempty"`
	ExpiresAt     time.Time `json:"expiresAt"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Institution is a registered data-consuming organization.
type Institution struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Sector     string    `json:"sector"`
	APIKeyHash string    `json:"-"`
	IsActive   bool      `json:"isActive"`
	CreatedAt  time.Time `json:"createdAt"`
}

// AuditEntry is one row in the append-only hash-chained ledger.
type AuditEntry struct {
	ID            string          `json:"id"`
	SequenceNo    int64           `json:"sequenceNo"`
	EventType     string          `json:"eventType"`
	CitizenID     *string         `json:"citizenId,omitempty"`
	InstitutionID *string         `json:"institutionId,omitempty"`
	RequestID     *string         `json:"requestId,omitempty"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
	PrevHash      string          `json:"prevHash"`
	EntryHash     string          `json:"entryHash"`
	CreatedAt     time.Time       `json:"createdAt"`
}

// ConsentEvent is the Kafka event schema shared by producer and consumer.
type ConsentEvent struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	CitizenID     string          `json:"citizen_id"`
	InstitutionID string          `json:"institution_id"`
	RequestID     string          `json:"request_id"`
	Timestamp     time.Time       `json:"timestamp"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

// EventType constants for the consent lifecycle.
const (
	EventConsentRequested = "consent_requested"
	EventConsentApproved  = "consent_approved"
	EventConsentRejected  = "consent_rejected"
	EventConsentRevoked   = "consent_revoked"
	EventDataAccessed     = "data_accessed"
)
