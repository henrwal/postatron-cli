package apiv1

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The API's own tests compare the description with its routes and with these
// types; this only checks the embedded file is a document at all.
func TestOpenAPIIsAnOpenAPI31Document(t *testing.T) {
	var doc struct {
		OpenAPI  string                     `json:"openapi"`
		Paths    map[string]json.RawMessage `json:"paths"`
		Webhooks map[string]json.RawMessage `json:"webhooks"`
	}
	require.NoError(t, json.Unmarshal(OpenAPI, &doc))
	assert.True(t, strings.HasPrefix(doc.OpenAPI, "3.1."), "openapi is %q", doc.OpenAPI)
	assert.Contains(t, doc.Paths, OpenAPIPath)
	for path := range doc.Paths {
		assert.True(t, strings.HasPrefix(path, "/v1/"), "%s is not a /v1 route", path)
	}
	for _, event := range append([]string{WebhookTest}, WebhookEventTypes...) {
		assert.Contains(t, doc.Webhooks, event)
	}
}
