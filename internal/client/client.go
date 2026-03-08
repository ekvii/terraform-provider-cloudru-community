// Package client provides a thin HTTP wrapper used by all provider components.
//
// It encapsulates:
//   - Bearer-token authorization on every request
//   - Transparent page-token pagination via [GetPaged]
//   - Consistent error handling (transport errors, non-2xx status codes, API
//     error fields in the JSON body, and JSON decode failures)
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// IsNotFound reports whether err originates from an HTTP 404 response as
// surfaced by the client methods.
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), fmt.Sprintf("unexpected HTTP status %d", http.StatusNotFound))
}

// CloudRuHttpClient is the shared HTTP client for all provider resources and
// data sources. Create one via [NewCloudRuHttpClient] and store it in the
// provider's configure response so every resource/datasource can retrieve it
// through ProviderData.
type CloudRuHttpClient struct {
	httpClient *http.Client
	token      string

	// ProjectID is the default Cloud.ru project used when callers do not
	// supply one explicitly.
	ProjectID string

	// VpcEndpoint is the base URL for the VPC API (e.g. "https://vpc.api.cloud.ru").
	VpcEndpoint string

	// DnsEndpoint is the base URL for the DNS API (e.g. "https://dns.api.cloud.ru").
	DnsEndpoint string

	// ComputeEndpoint is the base URL for the Compute API (e.g. "https://compute.api.cloud.ru").
	// It hosts the subnet, VM, and other compute resources.
	ComputeEndpoint string
}

// NewCloudRuHttpClient builds a fully initialised [CloudRuHttpClient].
// It constructs an internal retryable HTTP client, fetches a bearer token via
// the OAuth2 client_credentials grant, and stores all configuration so callers
// never need to handle auth or endpoint details directly.
func NewCloudRuHttpClient(ctx context.Context, keyID, secret, projectID, vpcEndpoint, dnsEndpoint, computeEndpoint string) (*CloudRuHttpClient, error) {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.HTTPClient.Timeout = 3 * time.Minute
	retryClient.Logger = nil
	httpClient := retryClient.StandardClient()

	token, err := fetchToken(ctx, httpClient, keyID, secret)
	if err != nil {
		return nil, err
	}

	return &CloudRuHttpClient{
		httpClient:      httpClient,
		token:           token,
		ProjectID:       projectID,
		VpcEndpoint:     vpcEndpoint,
		DnsEndpoint:     dnsEndpoint,
		ComputeEndpoint: computeEndpoint,
	}, nil
}

// fetchToken obtains a short-lived bearer token from the Cloud.ru identity
// endpoint using the OAuth2 client_credentials grant.
func fetchToken(ctx context.Context, httpClient *http.Client, keyID, secret string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", keyID)
	form.Set("client_secret", secret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://id.cloud.ru/auth/system/openid/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected token response status: %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	return body.AccessToken, nil
}

// apiError is the common error envelope returned by Cloud.ru APIs.
type apiError struct {
	Error string `json:"error"`
}

// execute performs an HTTP request with the Authorization header set, asserts
// the expected HTTP status code, reads the full body, checks for an "error"
// field in the JSON response, and — when dest is non-nil — decodes the body
// into dest.
func (c *CloudRuHttpClient) execute(req *http.Request, expectedStatus int, dest interface{}) error {
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != expectedStatus {
		// Try to surface the API error message if present.
		var apiErr apiError
		if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, apiErr.Error)
		}
		return fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, string(body))
	}

	// Even on a success status some APIs embed an error field.
	var apiErr apiError
	if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error != "" {
		return fmt.Errorf("API error: %s", apiErr.Error)
	}

	if dest != nil {
		if err := json.Unmarshal(body, dest); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// GetJSON performs a GET request, asserts HTTP 200, checks for an API error
// field, and JSON-decodes the response body into dest.
func (c *CloudRuHttpClient) GetJSON(ctx context.Context, url string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	return c.execute(req, http.StatusOK, dest)
}

// PostJSON marshals body as JSON, performs a POST request, asserts HTTP 200,
// checks for an API error field, and JSON-decodes the response into dest.
// Pass nil for dest to discard the response body.
func (c *CloudRuHttpClient) PostJSON(ctx context.Context, url string, body interface{}, dest interface{}) error {
	return c.postJSON(ctx, url, body, http.StatusOK, dest)
}

// PostJSONCreated is like [PostJSON] but asserts HTTP 201 instead of 200.
// Use this for endpoints that return 201 Created on success.
func (c *CloudRuHttpClient) PostJSONCreated(ctx context.Context, url string, body interface{}, dest interface{}) error {
	return c.postJSON(ctx, url, body, http.StatusCreated, dest)
}

func (c *CloudRuHttpClient) postJSON(ctx context.Context, url string, body interface{}, expectedStatus int, dest interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.execute(req, expectedStatus, dest)
}

// PutJSON marshals body as JSON, performs a PUT request, asserts HTTP 200,
// checks for an API error field, and JSON-decodes the response into dest.
// Pass nil for dest to discard the response body.
func (c *CloudRuHttpClient) PutJSON(ctx context.Context, url string, body interface{}, dest interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.execute(req, http.StatusOK, dest)
}

// Delete performs a DELETE request, asserts HTTP 200, and checks for an API
// error field in the response body.
func (c *CloudRuHttpClient) Delete(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	return c.execute(req, http.StatusOK, nil)
}

// DeleteJSON performs a DELETE request, asserts HTTP 200, checks for an API
// error field, and JSON-decodes the response body into dest.
// Use this for endpoints that return a body (e.g. an async Operation) on deletion.
// Pass nil for dest to discard the response body.
func (c *CloudRuHttpClient) DeleteJSON(ctx context.Context, url string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	return c.execute(req, http.StatusOK, dest)
}

// DeleteNoContent performs a DELETE request and asserts HTTP 204 (no body).
// Use this for endpoints that return 204 No Content on successful deletion.
func (c *CloudRuHttpClient) DeleteNoContent(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	return c.execute(req, http.StatusNoContent, nil)
}

// operationError mirrors the error field embedded in an Operation (google.rpc.Status).
type operationError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Operation is the common async operation envelope returned by Cloud.ru APIs
// for mutating requests (create, update, delete). Poll the operation URL until
// Done is true, then inspect ResourceId to get the created/modified resource ID.
type Operation struct {
	Id         string          `json:"id"`
	Done       bool            `json:"done"`
	ResourceId string          `json:"resourceId"`
	Error      *operationError `json:"error"`
}

// WaitForOperation polls the given operationURL every 2 seconds until the
// operation reports Done == true, then returns the final Operation value.
// If the completed operation carries a non-nil error field, an error is returned.
// The context deadline is respected — if the context is cancelled or times out,
// the function returns the context error immediately.
func (c *CloudRuHttpClient) WaitForOperation(ctx context.Context, operationURL string) (*Operation, error) {
	var op Operation
	if err := c.GetJSON(ctx, operationURL, &op); err != nil {
		return nil, err
	}
	for !op.Done {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
		if err := c.GetJSON(ctx, operationURL, &op); err != nil {
			return nil, err
		}
	}
	if op.Error != nil {
		return nil, fmt.Errorf("operation %s failed (code %d): %s", op.Id, op.Error.Code, op.Error.Message)
	}
	return &op, nil
}

// PagedResponse is the minimal interface a paginated list response must
// satisfy so [GetPaged] can extract items and advance the cursor.
//
// T is the item type returned per page.
type PagedResponse[T any] interface {
	// Items returns the slice of items from this page.
	Items() []T
	// NextToken returns the opaque page token for the following page, or an
	// empty string when this is the last page.
	NextToken() string
}

// GetPaged fetches all pages from a paginated list endpoint and returns the
// deduplicated items. The endpoint must accept an optional "pageToken" query
// parameter appended to baseURL.
//
// newResp is a factory that returns a fresh, zeroed *R (where R implements
// [PagedResponse][T]) for each page decode. idOf extracts a stable unique
// string key from each item for deduplication.
//
// Example:
//
//	type vpcPage struct { ... }
//	func (p *vpcPage) Items() []apiVpc      { return p.Vpcs }
//	func (p *vpcPage) NextToken() string    { return p.NextPageToken }
//
//	vpcs, err := client.GetPaged(ctx, c, baseURL,
//	    func() *vpcPage { return &vpcPage{} },
//	    func(v apiVpc) string { return v.ID },
//	)
func GetPaged[T any, R PagedResponse[T]](
	ctx context.Context,
	c *CloudRuHttpClient,
	baseURL string,
	newResp func() R,
	idOf func(T) string,
) ([]T, error) {
	var (
		all    []T
		seen   = make(map[string]struct{})
		cursor = ""
	)

	for {
		reqURL := baseURL
		if cursor != "" {
			reqURL += "&pageToken=" + cursor
		}

		page := newResp()
		if err := c.GetJSON(ctx, reqURL, page); err != nil {
			return nil, err
		}

		for _, item := range page.Items() {
			id := idOf(item)
			if _, dup := seen[id]; !dup {
				seen[id] = struct{}{}
				all = append(all, item)
			}
		}

		next := page.NextToken()
		if next == "" || next == cursor {
			break
		}
		cursor = next
	}

	return all, nil
}
