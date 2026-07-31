package scim

import (
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

const (
	ScimSchemaError        = "urn:ietf:params:scim:api:messages:2.0:Error"
	ScimSchemaGroup        = "urn:ietf:params:scim:schemas:core:2.0:Group"
	ScimSchemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	ScimSchemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	ScimSchemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
)

type ScimEmail struct {
	Primary bool   `json:"primary,omitempty"`
	Type    string `json:"type,omitempty"`
	Value   string `json:"value"`
}

type ScimError struct {
	Detail   string   `json:"detail"`
	Schemas  []string `json:"schemas"`
	ScimType string   `json:"scimType,omitempty"`
	Status   string   `json:"status"`
}

type ScimListResponse struct {
	ItemsPerPage int         `json:"itemsPerPage"`
	Resources    interface{} `json:"Resources"`
	Schemas      []string    `json:"schemas"`
	StartIndex   int         `json:"startIndex"`
	TotalResults int         `json:"totalResults"`
}

type ScimMeta struct {
	Location     string `json:"location"`
	ResourceType string `json:"resourceType"`
}

type ScimName struct {
	FamilyName string `json:"familyName,omitempty"`
	Formatted  string `json:"formatted,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

type ScimOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path,omitempty"`
	Value interface{} `json:"value"`
}

type ScimPatchOp struct {
	Operations []ScimOperation `json:"Operations"`
	Schemas    []string        `json:"schemas"`
}

type ScimUser struct {
	Active     bool        `json:"active"`
	Emails     []ScimEmail `json:"emails,omitempty"`
	ExternalID string      `json:"externalId,omitempty"`
	ID         string      `json:"id"`
	Meta       ScimMeta    `json:"meta"`
	Name       ScimName    `json:"name,omitempty"`
	Schemas    []string    `json:"schemas"`
	UserName   string      `json:"userName"`
}

// DomainUserToScimUser converts a domain User and its workspace Member to a SCIM 2.0 User resource.
func DomainUserToScimUser(u *user.User, member workspace.Member, baseURL string) ScimUser {
	return ScimUser{
		Active:     !member.Disabled,
		Emails:     []ScimEmail{{Primary: true, Type: "work", Value: u.Email()}},
		ExternalID: member.ExternalID,
		ID:         u.ID().String(),
		Meta: ScimMeta{
			Location:     baseURL + "/scim/v2/Users/" + u.ID().String(),
			ResourceType: "User",
		},
		Name:     ScimName{Formatted: u.Name()},
		Schemas:  []string{ScimSchemaUser},
		UserName: u.Email(),
	}
}
