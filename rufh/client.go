package rufh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"slices"
)

const (
	uploadResumptionSupportedStatusCode = 104
)

var dummy1xxCb = func(code int, header textproto.MIMEHeader) error { return nil }

func NewClient(endpoint *url.URL) (*Client, error) {
	if endpoint == nil {
		panic("endpoint is nil")
	}
	return &Client{
		endpoint: endpoint,
	}, nil
}

type Client struct {
	ExtraHeaders       http.Header
	DoRequest          DoRequestFunc
	Client             HTTPClient
	CreateUploadMethod string
	GetUploadMethod    string

	endpoint *url.URL
}

type DoRequestFunc func(req *http.Request, on1xxResponse func(code int, header textproto.MIMEHeader) error) (*http.Response, error)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func (c Client) GetUpload(location string) (*Upload, error) {
	method := c.GetUploadMethod
	if method == "" {
		method = http.MethodHead
	}
	req, err := c.createRequest(method, location, dummy1xxCb)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.rufhRequest(req, dummy1xxCb)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer closeResponseBody(resp)

	if resp.StatusCode >= 400 {
		return nil, newTusError()
	}

	loc, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse location: %w", err)
	}

	upload := Upload{client: c, location: loc}
	headers := []string{"Upload-Complete", "Upload-Length", "Upload-Limit", "Upload-Offset"}
	if err := processResponseHeaders(resp, headers, []string{"Upload-Offset"}, &upload); err != nil {
		return nil, newTusError()
	}
	if resp.Header.Get("Upload-Length") == "" {
		upload.length = -1 // length is unknown or has not been set yet
	}
	if upload.complete {
		upload.finalizedLocation = upload.location
	}

	return &upload, nil
}

func (c Client) SubmitUpload(data []byte, contentType, contentEncoding string) (*Upload, error) {
	resp, upload, validUpload, err := c.createUpload(int64(len(data)), data, contentType, contentEncoding, "?1")
	defer closeResponseBody(resp)
	if err != nil {
		return validUpload, err
	}

	/* 2xx, 3xx response */

	// Upload is complete regardless the presence of the Upload-Complete header in the response
	upload.complete = true
	// Assume all data has been confirmed even if the server doesn't support the interim responses carrying the offset changes.
	upload.Offset = int64(len(data))

	// Response may contain the optional Location header, treat it as the finalized location
	if v := resp.Header.Get("Location"); v != "" {
		loc, err := url.Parse(v)
		if err != nil {
			return validUpload, newTusError() // TODO: protocol error
		}
		upload.finalizedLocation = loc
		// Response contains the Location header, so the upload is valid even if the server does not support interim responses
		validUpload = &upload
	} else {
		upload.finalizedLocation = upload.location
	}

	// Upload-Limit is optional header as well, read it if present
	if err := processResponseHeaders(resp, []string{"Upload-Limit"}, nil, &upload); err != nil {
		return validUpload, newTusError()
	}

	return validUpload, err
}

func (c Client) InitiateUpload(length int64, data []byte, contentType, contentEncoding string) (*Upload, error) {
	if length < -1 {
		return nil, fmt.Errorf("invalid length: %d", length)
	}
	if length < int64(len(data)) {
		return nil, fmt.Errorf("length %d is less than data length %d", length, len(data))
	}

	resp, upload, validUpload, err := c.createUpload(length, data, contentType, contentEncoding, "?0")
	defer closeResponseBody(resp)
	if err != nil {
		return validUpload, err
	}

	/* 2xx, 3xx response */

	// Assume all data has been confirmed even if the server doesn't support the interim responses carrying the offset changes.
	upload.Offset = int64(len(data))

	headers := []string{"Upload-Limit", "Location"}
	if err := processResponseHeaders(resp, headers, []string{"Location"}, &upload); err != nil {
		return validUpload, newTusError()
	}
	// Response contains the Location header, so the upload is valid even if the server does not support interim responses
	validUpload = &upload

	// On success, server is required to set the upload limits either in the interim responses or in the final response.
	// This is less strict than the protocol spec requires, where both interim and final responses have to contain the upload limits.
	if upload.limits == nil {
		return validUpload, newTusError() // TODO: protocol error
	}

	if resp.Header.Get("Upload-Complete") == "?1" {
		return validUpload, newTusError() // TODO: protocol error
	}

	return validUpload, err
}

func (c Client) CancelUpload(location string) error {
	req, err := c.createRequest(http.MethodDelete, location, dummy1xxCb)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.rufhRequest(req, dummy1xxCb)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer closeResponseBody(resp)

	if resp.StatusCode >= 400 {
		return newTusError()
	}
	return nil
}

func (c Client) GetServerLimits() (Limits, error) {
	req, err := c.createRequest(http.MethodOptions, c.endpoint.String(), dummy1xxCb)
	if err != nil {
		return Limits{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.rufhRequest(req, dummy1xxCb)
	if err != nil {
		return Limits{}, fmt.Errorf("request: %w", err)
	}
	defer closeResponseBody(resp)

	if resp.StatusCode >= 400 {
		return Limits{}, newTusError()
	}

	var res Limits
	if v := resp.Header.Get("Upload-Limit"); v == "" {
		return Limits{}, newTusError() // TODO: protocol error, server does not support upload creation or this is not a tus server
	} else if err := res.UnmarshalText([]byte(v)); err != nil {
		return Limits{}, newTusError() // TODO: protocol error
	}

	return res, nil
}

func (c Client) createUpload(length int64, data []byte, contentType, contentEncoding, uploadComplete string) (*http.Response, Upload, *Upload, error) {
	method := c.CreateUploadMethod
	if method == "" {
		method = http.MethodPost
	}

	upload := Upload{length: length, client: c}
	req, err := c.createRequest(method, c.endpoint.String(), upload.on1xxResponse)
	if err != nil {
		return nil, upload, nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Upload-Complete", uploadComplete)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}

	resp, err := c.sendData(data, length, req, &upload)
	var validUpload *Upload  // "Valid" means the upload has location or finalized location
	if upload.location != nil {
		validUpload = &upload // Location was received in the interim responses
		if err != nil {
			return nil, upload, validUpload, newTusError() // TODO: incomplete write
		}
	}
	if err != nil {
		return nil, upload, validUpload, err
	}

	// Server can explicitly mark the upload as complete in the error response.
	// Otherwise, assume that the upload remains incomplete until handling the 2xx response.
	if resp.Header.Get("Upload-Complete") == "?1" {
		upload.complete = true
	}

	switch {
	case resp.StatusCode == http.StatusRequestEntityTooLarge:
		return resp, upload, validUpload, newTusError() // TODO: too large
	case resp.StatusCode >= 500 && upload.location != nil:
		return resp, upload, validUpload, newTusError() // TODO: incomplete write
	case resp.StatusCode >= 400:
		return resp, upload, validUpload, newTusError() // TODO: unexpected error
	}

	return resp, upload, validUpload, nil
}

func (c Client) sendData(data []byte, length int64, req *http.Request, upload *Upload) (*http.Response, error) {
	if length >= 0 {
		req.Header.Set("Upload-Length", fmt.Sprintf("%d", length))
	}
	req.ContentLength = int64(len(data)) // TODO: remove if len(data) == 0?
	if len(data) > 0 {
		req.Body = io.NopCloser(bytes.NewReader(data))
	} else {
		req.Body = http.NoBody
	}

	resp, err := c.rufhRequest(req, upload.on1xxResponse)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	return resp, nil
}

func (c Client) createRequest(method, url string, on1xx func(code int, header textproto.MIMEHeader) error) (*http.Request, error) {
	ctx := httptrace.WithClientTrace(context.Background(), &httptrace.ClientTrace{
		Got1xxResponse: on1xx,
	})
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}

	if len(c.ExtraHeaders) > 0 {
		req.Header = c.ExtraHeaders.Clone()
	}

	return req, nil
}

func (c Client) rufhRequest(req *http.Request, on1xx func(code int, header textproto.MIMEHeader) error) (*http.Response, error) {
	var resp *http.Response
	var err error

	if c.DoRequest != nil {
		resp, err = c.DoRequest(req, on1xx)
	} else {
		cl := c.Client
		if cl == nil {
			cl = http.DefaultClient
		}
		resp, err = cl.Do(req)
	}
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

func processResponseHeaders(resp *http.Response, headers, requiredHeaders []string, upload *Upload) error {
	for _, h := range headers {
		v := resp.Header.Get(h)
		// Check required headers
		if slices.Contains(requiredHeaders, h) && v == "" {
			return fmt.Errorf("missing required header: %s", h)
		}
		if err := upload.processResponseHeader(h, v); err != nil {
			return fmt.Errorf("header %q: %w", h, err)
		}
	}
	return nil
}

func closeResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}
