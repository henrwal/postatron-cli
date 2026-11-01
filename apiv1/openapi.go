package apiv1

import _ "embed"

// OpenAPI is the OpenAPI 3.1 description of the REST API: every /v1 route,
// its parameters, bodies, errors and headers, and the webhooks the API sends,
// as JSON. The API serves the same document at OpenAPIPath, without a key.
//
// It is written by hand, and the API's tests hold it to the routes the API
// serves and to the JSON fields of the types in this package, so a new route
// or field needs a line here too.
//
//go:embed openapi.json
var OpenAPI []byte

// OpenAPIPath is where the API serves OpenAPI.
const OpenAPIPath = "/v1/openapi.json"
