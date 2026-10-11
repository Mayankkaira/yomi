package consent

import (
	"errors"
	"fmt"

	"github.com/yomi/yomi/internal/models"
)

type Scope struct {
	Key    string                   `json:"key"`
	Label  string                   `json:"label"`
	Detail string                   `json:"detail"`
	Roles  []models.InstitutionRole `json:"-"`
}

var scopeCatalog = []Scope{
	{
		Key:    "identity.basic",
		Label:  "Name and address",
		Detail: "Your name, date of birth and registered address",
		Roles:  []models.InstitutionRole{models.InstitutionRoleViewer, models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin},
	},
	{
		Key:    "identity.contact",
		Label:  "Phone and email",
		Detail: "The phone number and email address on your record",
		Roles:  []models.InstitutionRole{models.InstitutionRoleViewer, models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin},
	},
	{
		Key:    "financial.accounts",
		Label:  "Account numbers and balances",
		Detail: "Bank account numbers, balances and standing instructions",
		Roles:  []models.InstitutionRole{models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin},
	},
	{
		Key:    "financial.transactions",
		Label:  "Transaction history",
		Detail: "Where your money was sent and received, and when",
		Roles:  []models.InstitutionRole{models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin},
	},
	{
		Key:    "financial.credit",
		Label:  "Credit score and report",
		Detail: "Your credit score, open loans and repayment record",
		Roles:  []models.InstitutionRole{models.InstitutionRoleAdmin},
	},
	{
		Key:    "health.records",
		Label:  "Medical records",
		Detail: "Diagnoses, prescriptions and hospital visits",
		Roles:  []models.InstitutionRole{models.InstitutionRoleAdmin},
	},
	{
		Key:    "telecom.usage",
		Label:  "Call and data usage",
		Detail: "Call detail records, data usage and recharge history",
		Roles:  []models.InstitutionRole{models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin},
	},
}

var (
	ErrUnknownScope    = errors.New("unknown data scope")
	ErrScopeNotAllowed = errors.New("institution role cannot request this data scope")
	ErrUnknownRole     = errors.New("unknown institution role")
)

func ScopeCatalog() []Scope {
	out := make([]Scope, len(scopeCatalog))
	copy(out, scopeCatalog)
	return out
}

func scopeByKey(key string) (Scope, bool) {
	for _, s := range scopeCatalog {
		if s.Key == key {
			return s, true
		}
	}
	return Scope{}, false
}

func roleAllows(role models.InstitutionRole, s Scope) bool {
	for _, r := range s.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func ValidRole(role models.InstitutionRole) bool {
	switch role {
	case models.InstitutionRoleViewer, models.InstitutionRoleAnalyst, models.InstitutionRoleAdmin:
		return true
	}
	return false
}

func NormalizeScope(role models.InstitutionRole, keys []string) ([]string, error) {
	if !ValidRole(role) {
		return nil, fmt.Errorf("%w: %s", ErrUnknownRole, role)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: no scope requested", ErrUnknownScope)
	}
	requested := make(map[string]bool, len(keys))
	for _, k := range keys {
		s, ok := scopeByKey(k)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownScope, k)
		}
		if !roleAllows(role, s) {
			return nil, fmt.Errorf("%w: %s at role %s", ErrScopeNotAllowed, k, role)
		}
		requested[k] = true
	}
	out := make([]string, 0, len(requested))
	for _, s := range scopeCatalog {
		if requested[s.Key] {
			out = append(out, s.Key)
		}
	}
	return out, nil
}

type ScopeSummary struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

func DescribeScopes(keys []string) []ScopeSummary {
	out := make([]ScopeSummary, 0, len(keys))
	for _, k := range keys {
		if s, ok := scopeByKey(k); ok {
			out = append(out, ScopeSummary{Key: s.Key, Label: s.Label, Detail: s.Detail})
			continue
		}
		out = append(out, ScopeSummary{Key: k, Label: k, Detail: "Unrecognised scope key"})
	}
	return out
}

func CatalogForRole(role models.InstitutionRole) []ScopeSummary {
	out := make([]ScopeSummary, 0, len(scopeCatalog))
	for _, s := range scopeCatalog {
		if roleAllows(role, s) {
			out = append(out, ScopeSummary{Key: s.Key, Label: s.Label, Detail: s.Detail})
		}
	}
	return out
}
