// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charmhub

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/juju/errors"
	httprequest "gopkg.in/httprequest.v1"
)

// Transport defines a type for making the actual request.
type Transport interface {
	// Do performs the *http.Request and returns a *http.Response or an error
	// if it fails to construct the transport.
	Do(*http.Request) (*http.Response, error)
}

// DefaultHTTPTransport creates a new HTTPTransport.
func DefaultHTTPTransport() *http.Client {
	return &http.Client{}
}

// APIError represents the error from the charmhub api.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// APIRequester creates a wrapper around the transport to allow for better
// error handling.
type APIRequester struct {
	transport Transport
}

// NewAPIRequester creates a new http.Client for making requests to a server.
func NewAPIRequester(transport Transport) *APIRequester {
	return &APIRequester{
		transport: transport,
	}
}

// Do performs the *http.Request and returns a *http.Response or an error
// if it fails to construct the transport.
func (t *APIRequester) Do(req *http.Request) (*http.Response, error) {
	resp, err := t.transport.Do(req)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusNoContent {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the response error.
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Annotate(err, "cannot read response body")
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != "application/json" {
		return nil, errors.Errorf(`expected "application/json" contentType from server: %v`, contentType)
	}

	var apiError APIError
	if err := json.Unmarshal(data, &apiError); err != nil {
		return nil, errors.Trace(err)
	}

	return resp, errors.Errorf(apiError.Message)
}

// RESTClient defines a type for making requests to a server.
type RESTClient interface {
	// Get performs GET requests to a given Path.
	Get(context.Context, Path, interface{}) error
}

// HTTPRESTClient represents a RESTClient that expects to interact with a
// HTTP transport.
type HTTPRESTClient struct {
	transport Transport
}

// NewHTTPRESTClient creates a new HTTPRESTClient
func NewHTTPRESTClient(transport Transport) *HTTPRESTClient {
	return &HTTPRESTClient{
		transport: transport,
	}
}

// Get makes a GET request to the given path in the charm store (not
// including the host name or version prefix but including a leading /),
// parsing the result as JSON into the given result value, which should
// be a pointer to the expected data, but may be nil if no result is
// desired.
func (c *HTTPRESTClient) Get(ctx context.Context, path Path, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", path.String(), nil)
	if err != nil {
		return errors.Annotate(err, "can not make new request")
	}
	resp, err := c.transport.Do(req)
	if err != nil {
		return errors.Trace(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse the response.
	if err := httprequest.UnmarshalJSONResponse(resp, result); err != nil {
		return errors.Annotate(err, "charm hub client get")
	}
	return nil
}
