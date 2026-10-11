package models

import (
	"encoding/json"
	"time"
)

type Role string

const (
	RoleCitizen     Role = "citizen"
	RoleInstitution Role = "institution"
	RoleAdmin       Role = "admin"
)

type InstitutionRole string

const (
	InstitutionRoleViewer  InstitutionRole = "viewer"
	InstitutionRoleAnalyst InstitutionRole = "analyst"
	InstitutionRoleAdmin   InstitutionRole = "admin"
)

type Citizen struct {
	ID           string    `json:"id"`
	AadhaarID    string    `json:"aadhaarId"`
	FullName     string    `json:"fullName"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ConsentStatus string

const (
	StatusPending  ConsentStatus = "pending"
	StatusApproved ConsentStatus = "approved"
	StatusRejected ConsentStatus = "rejected"
	StatusRevoked  ConsentStatus = "revoked"
	StatusExpired  ConsentStatus = "expired"
)

type ConsentRequest struct {
	ID              string          `json:"id"`
	CitizenID       string          `json:"citizenId"`
	InstitutionID   string          `json:"institutionId"`
	InstitutionRole InstitutionRole `json:"institutionRole,omitempty"`
	DataScope       []string        `json:"dataScope"`
	Purpose         string          `json:"purpose"`
	Status          string          `json:"status"`
	AccessTokenHash string          `json:"accessTokenHash,omitempty"`
	ExpiresAt       time.Time       `json:"expiresAt"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type Institution struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Sector     string          `json:"sector"`
	Role       InstitutionRole `json:"role"`
	APIKeyHash string          `json:"-"`
	IsActive   bool            `json:"isActive"`
	CreatedAt  time.Time       `json:"createdAt"`
}

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

type ConsentEvent struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	CitizenID     string          `json:"citizen_id"`
	InstitutionID string          `json:"institution_id"`
	RequestID     string          `json:"request_id"`
	Timestamp     time.Time       `json:"timestamp"`
	Metadata      json.RawMessage `json:"metadata,omitempty"`
}

const (
	EventConsentRequested = "consent_requested"
	EventConsentApproved  = "consent_approved"
	EventConsentRejected  = "consent_rejected"
	EventConsentRevoked   = "consent_revoked"
	EventDataAccessed     = "data_accessed"
)
