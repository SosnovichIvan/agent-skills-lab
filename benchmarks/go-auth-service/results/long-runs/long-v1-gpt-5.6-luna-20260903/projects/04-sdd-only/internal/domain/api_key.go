package domain

import "time"

// APIKey stores only a digest of the credential and permissions scoped to one
// organization.
type APIKey struct {
	ID             ID           `json:"id"`
	OrganizationID ID           `json:"organization_id"`
	Name           string       `json:"name"`
	KeyHash        []byte       `json:"-"`
	Scopes         []Permission `json:"scopes"`
	CreatedAt      time.Time    `json:"created_at"`
	Revoked        bool         `json:"revoked"`
}

type APIKeyProfile struct {
	ID             ID           `json:"id"`
	OrganizationID ID           `json:"organization_id"`
	Name           string       `json:"name"`
	Scopes         []Permission `json:"scopes"`
	CreatedAt      time.Time    `json:"created_at"`
	Revoked        bool         `json:"revoked"`
}

func (key APIKey) PublicProfile() APIKeyProfile {
	return APIKeyProfile{
		ID: key.ID, OrganizationID: key.OrganizationID, Name: key.Name,
		Scopes: append([]Permission(nil), key.Scopes...), CreatedAt: key.CreatedAt, Revoked: key.Revoked,
	}
}
