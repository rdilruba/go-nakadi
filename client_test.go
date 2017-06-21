// Copyright (c) 2017, A. Stoewer <adrian.stoewer@rz.ifi.lmu.de>
// All rights reserved.

package nakadi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stoewer/go-nakadi/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/jarcoal/httpmock.v1"
)

func TestTokenProvider_Authorize(t *testing.T) {
	t.Run("fail nil provider", func(t *testing.T) {
		var provider TokenProvider
		req := httptest.NewRequest("GET", "/", nil)

		err := provider.Authorize(req)
		require.Error(t, err)
		assert.Regexp(t, "no token func", err)
	})

	t.Run("fail retrieve token", func(t *testing.T) {
		provider := TokenProvider(func() (string, error) { return "", assert.AnError })
		req := httptest.NewRequest("GET", "/", nil)

		err := provider.Authorize(req)
		require.Error(t, err)
		assert.Regexp(t, assert.AnError, err)
	})

	t.Run("successfully add token", func(t *testing.T) {
		provider := TokenProvider(func() (string, error) { return "token", nil })
		req := httptest.NewRequest("GET", "/", nil)

		err := provider.Authorize(req)
		require.NoError(t, err)
		assert.Equal(t, "Bearer token", req.Header.Get("Authorization"))
	})
}

func TestNew(t *testing.T) {
	t.Run("no options", func(t *testing.T) {
		client := New()

		require.NotNil(t, client)
		assert.Equal(t, client.nakadiURL, defaultNakadiURL)
		assert.Equal(t, client.timeout, defaultTimeOut)
		assert.NotNil(t, client.httpClient)
		assert.Equal(t, defaultTimeOut, client.httpClient.Timeout)
		assert.NotNil(t, client.httpStream)
		assert.Nil(t, client.tokenProvider)
	})

	t.Run("timeout option", func(t *testing.T) {
		timeout := 5 * time.Second
		client := New(Timeout(timeout))

		require.NotNil(t, client)
		assert.Equal(t, client.nakadiURL, defaultNakadiURL)
		assert.Equal(t, client.timeout, timeout)
		assert.NotNil(t, client.httpClient)
		assert.Equal(t, timeout, client.httpClient.Timeout)
		assert.NotNil(t, client.httpStream)
		assert.Nil(t, client.tokenProvider)
	})

	t.Run("tokens option", func(t *testing.T) {
		provider := TokenProvider(func() (string, error) { return "token", nil })
		client := New(Tokens(provider))

		require.NotNil(t, client)
		assert.Equal(t, client.nakadiURL, defaultNakadiURL)
		assert.Equal(t, client.timeout, defaultTimeOut)
		assert.NotNil(t, client.httpClient)
		assert.Equal(t, defaultTimeOut, client.httpClient.Timeout)
		assert.NotNil(t, client.httpStream)
		assert.NotNil(t, client.tokenProvider)
	})

	t.Run("url option", func(t *testing.T) {
		url := "https://example.com/nakadi"
		client := New(URL(url))

		require.NotNil(t, client)
		assert.Equal(t, client.nakadiURL, url)
		assert.Equal(t, client.timeout, defaultTimeOut)
		assert.NotNil(t, client.httpClient)
		assert.Equal(t, defaultTimeOut, client.httpClient.Timeout)
		assert.NotNil(t, client.httpStream)
		assert.Nil(t, client.tokenProvider)
	})
}

func TestClient_Publish(t *testing.T) {
	client := &Client{}
	// TODO implement actual test
	err := client.Publish("", event.DataChange{})
	require.Nil(t, err)
}

func TestClient_Subscribe(t *testing.T) {
	url := fmt.Sprintf("%s/subscriptions", defaultNakadiURL)
	setupClient := func(responder httpmock.Responder) *Client {
		httpmock.RegisterResponder("POST", url, responder)
		return &Client{
			nakadiURL:  defaultNakadiURL,
			httpClient: http.DefaultClient}
	}

	t.Run("fail retrieve token", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		client := setupClient(httpmock.NewStringResponder(200, ""))
		client.tokenProvider = func() (string, error) { return "", assert.AnError }

		_, err := client.Subscribe("nakadi-client", "test-data", "")
		require.Error(t, err)
		assert.Regexp(t, assert.AnError, err)
	})

	t.Run("fail connect error", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		client := setupClient(httpmock.NewErrorResponder(assert.AnError))

		_, err := client.Subscribe("nakadi-client", "test-data", "")
		require.Error(t, err)
		assert.Regexp(t, assert.AnError, err)
	})

	t.Run("fail http error", func(t *testing.T) {
		problem := &problemJSON{Detail: "foo problem detail"}
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		responder, _ := httpmock.NewJsonResponder(400, problem)
		client := setupClient(responder)

		_, err := client.Subscribe("nakadi-client", "test-data", "")
		require.Error(t, err)
		assert.Regexp(t, problem.Detail, err)
	})

	t.Run("successful subscription", func(t *testing.T) {
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		sub := &Subscription{
			OwningApplication: "nakadi-client",
			EventTypes:        []string{"test-date"},
			ConsumerGroup:     "default"}
		responder, _ := httpmock.NewJsonResponder(200, sub)
		client := setupClient(responder)

		_, err := client.Subscribe("nakadi-client", "test-data", "")
		require.NoError(t, err)
		assert.Equal(t, "nakadi-client", sub.OwningApplication)
		assert.Equal(t, "test-date", sub.EventTypes[0])
		assert.Equal(t, "default", sub.ConsumerGroup)
	})
}

func TestClient_Stream(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	sub := &Subscription{
		ID:                "4e6f4b42-5459-11e7-8b76-97cbdf1f5274",
		OwningApplication: "nakadi-client",
		EventTypes:        []string{"test"},
		ConsumerGroup:     "default",
		ReadFrom:          "end",
		CreatedAt:         time.Now()}
	client := &Client{
		nakadiURL:  defaultNakadiURL,
		httpClient: http.DefaultClient,
		httpStream: http.DefaultClient}
	url := fmt.Sprintf("%s/subscriptions/%s/events", defaultNakadiURL, sub.ID)

	responder, _ := httpmock.NewJsonResponder(200, sub)
	httpmock.RegisterResponder("GET", url, responder)

	stream, err := client.Stream(sub)
	require.NoError(t, err)
	assert.NotNil(t, stream)
}