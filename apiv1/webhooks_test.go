package apiv1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookClientRoundTrips(t *testing.T) {
	var calls []string
	var created apiv1.CreateWebhookRequest
	var updated apiv1.UpdateWebhookRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/webhooks":
			_ = json.NewEncoder(w).Encode(apiv1.WebhookList{Data: []apiv1.Webhook{{ID: "wh1", URL: "https://example.com/hook", Enabled: true}}})
		case "POST /v1/webhooks":
			_ = json.NewDecoder(r.Body).Decode(&created)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(apiv1.Webhook{ID: "wh1", URL: created.URL, Events: created.Events, Secret: "whsec_abc"})
		case "PATCH /v1/webhooks/wh1":
			_ = json.NewDecoder(r.Body).Decode(&updated)
			_ = json.NewEncoder(w).Encode(apiv1.Webhook{ID: "wh1", Enabled: *updated.Enabled})
		case "DELETE /v1/webhooks/wh1":
			_ = json.NewEncoder(w).Encode(apiv1.DeleteWebhookResponse{ID: "wh1", Deleted: true})
		case "POST /v1/webhooks/wh1/test":
			_ = json.NewEncoder(w).Encode(apiv1.WebhookTestResult{DeliveryID: "whd_1", Delivered: true, StatusCode: 204})
		case "POST /v1/webhooks/wh1/rotate-secret":
			_ = json.NewEncoder(w).Encode(apiv1.Webhook{ID: "wh1", Secret: "whsec_new"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := apiv1.NewClient(server.URL, "ptn_test")
	ctx := context.Background()

	list, err := client.ListWebhooks(ctx)
	require.NoError(t, err)
	assert.Equal(t, "wh1", list.Data[0].ID)

	hook, err := client.CreateWebhook(ctx, apiv1.CreateWebhookRequest{URL: "https://example.com/hook", Events: []string{apiv1.WebhookPostFailed}})
	require.NoError(t, err)
	assert.Equal(t, "whsec_abc", hook.Secret)
	assert.Equal(t, []string{"post.failed"}, created.Events)

	off := false
	hook, err = client.UpdateWebhook(ctx, "wh1", apiv1.UpdateWebhookRequest{Enabled: &off})
	require.NoError(t, err)
	assert.False(t, hook.Enabled)
	assert.Nil(t, updated.URL, "a field left out is not sent")

	result, err := client.TestWebhook(ctx, "wh1")
	require.NoError(t, err)
	assert.True(t, result.Delivered)

	hook, err = client.RotateWebhookSecret(ctx, "wh1")
	require.NoError(t, err)
	assert.Equal(t, "whsec_new", hook.Secret)

	deleted, err := client.DeleteWebhook(ctx, "wh1")
	require.NoError(t, err)
	assert.True(t, deleted.Deleted)
	assert.Len(t, calls, 6)
}

func TestCreatePostSendsTheIdempotencyKeyAsAHeader(t *testing.T) {
	var header string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(apiv1.Post{ID: "p1"})
	}))
	defer server.Close()
	client := apiv1.NewClient(server.URL, "ptn_test")

	_, err := client.CreatePost(context.Background(), apiv1.CreatePostRequest{Content: "hi", Platforms: []string{"x"}, IdempotencyKey: "order-42"})
	require.NoError(t, err)
	assert.Equal(t, "order-42", header)
	assert.NotContains(t, body, "idempotency_key", "the key goes in the header, never the body")

	_, err = client.CreatePost(context.Background(), apiv1.CreatePostRequest{Content: "hi", Platforms: []string{"x"}})
	require.NoError(t, err)
	assert.Empty(t, header, "no key, no header")
}

func TestVerifyWebhook(t *testing.T) {
	const secret = "whsec_test"
	body := []byte(`{"id":"whd_1","type":"post.published","created_at":"2026-11-01T09:00:00Z","data":{}}`)
	now := time.Unix(1793000000, 0)
	t0 := now.Add(-time.Minute).Unix()
	sig := apiv1.SignWebhook(secret, t0, body)
	header := "t=" + strconv.FormatInt(t0, 10) + ",v1=" + sig

	assert.NoError(t, apiv1.VerifyWebhook(secret, header, body, 0, now))
	assert.NoError(t, apiv1.VerifyWebhook(secret, "t="+strconv.FormatInt(t0, 10)+",v1=deadbeef,v1="+sig, body, 0, now),
		"while a rotated secret is still signing, either v1 may match")
	assert.ErrorIs(t, apiv1.VerifyWebhook("whsec_other", header, body, 0, now), apiv1.ErrWebhookSignatureInvalid)
	assert.ErrorIs(t, apiv1.VerifyWebhook(secret, header, append(body, ' '), 0, now), apiv1.ErrWebhookSignatureInvalid, "the body is signed byte for byte")
	assert.ErrorIs(t, apiv1.VerifyWebhook(secret, header, body, 0, now.Add(time.Hour)), apiv1.ErrWebhookSignatureExpired)
	assert.ErrorIs(t, apiv1.VerifyWebhook(secret, "", body, 0, now), apiv1.ErrWebhookSignatureMissing)
	assert.ErrorIs(t, apiv1.VerifyWebhook(secret, "v1="+sig, body, 0, now), apiv1.ErrWebhookSignatureMissing)
}
