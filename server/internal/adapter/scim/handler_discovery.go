package scim

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// DiscoveryHandler serves the static SCIM 2.0 RFC 7643 §5 discovery endpoints.
// These endpoints are public (no authentication required).
type DiscoveryHandler struct{}

// NewDiscoveryHandler constructs a DiscoveryHandler.
func NewDiscoveryHandler() *DiscoveryHandler {
	return &DiscoveryHandler{}
}

// ResourceTypes handles GET /scim/v2/ResourceTypes.
func (h *DiscoveryHandler) ResourceTypes(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		"totalResults": 1,
		"startIndex":   1,
		"itemsPerPage": 1,
		"Resources": []map[string]interface{}{
			{
				"schemas":          []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"},
				"id":               "User",
				"name":             "User",
				"endpoint":         "/Users",
				"description":      "User Account",
				"schema":           ScimSchemaUser,
				"schemaExtensions": []interface{}{},
				"meta": map[string]interface{}{
					"location":     "/scim/v2/ResourceTypes/User",
					"resourceType": "ResourceType",
				},
			},
		},
	})
}

// Schemas handles GET /scim/v2/Schemas.
func (h *DiscoveryHandler) Schemas(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"schemas":      []string{"urn:ietf:params:scim:api:messages:2.0:ListResponse"},
		"totalResults": 1,
		"startIndex":   1,
		"itemsPerPage": 1,
		"Resources": []map[string]interface{}{
			{
				"id":          ScimSchemaUser,
				"name":        "User",
				"description": "User Account",
				"attributes": []map[string]interface{}{
					{
						"name":        "userName",
						"type":        "string",
						"multiValued": false,
						"required":    true,
						"caseExact":   false,
					},
					{
						"name":        "name",
						"type":        "complex",
						"multiValued": false,
						"required":    false,
					},
					{
						"name":        "emails",
						"type":        "complex",
						"multiValued": true,
						"required":    false,
					},
					{
						"name":        "active",
						"type":        "boolean",
						"multiValued": false,
						"required":    false,
					},
					{
						"name":        "externalId",
						"type":        "string",
						"multiValued": false,
						"required":    false,
					},
				},
				"meta": map[string]interface{}{
					"location":     "/scim/v2/Schemas/" + ScimSchemaUser,
					"resourceType": "Schema",
				},
			},
		},
	})
}

// ServiceProviderConfig handles GET /scim/v2/ServiceProviderConfig.
func (h *DiscoveryHandler) ServiceProviderConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"},
		"documentationUri": "",
		"patch": map[string]interface{}{
			"supported": true,
		},
		"bulk": map[string]interface{}{
			"supported":      false,
			"maxOperations":  0,
			"maxPayloadSize": 0,
		},
		"filter": map[string]interface{}{
			"supported":  true,
			"maxResults": 200,
		},
		"changePassword": map[string]interface{}{
			"supported": false,
		},
		"sort": map[string]interface{}{
			"supported": false,
		},
		"etag": map[string]interface{}{
			"supported": false,
		},
		"authenticationSchemes": []map[string]interface{}{
			{
				"type":        "oauthbearertoken",
				"name":        "OAuth Bearer Token",
				"description": "Authentication scheme using the OAuth Bearer Token standard",
			},
		},
		"meta": map[string]interface{}{
			"location":     "/scim/v2/ServiceProviderConfig",
			"resourceType": "ServiceProviderConfig",
		},
	})
}
