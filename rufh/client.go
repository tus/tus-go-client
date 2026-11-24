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
	ExtraHeaders http.Header
	DoRequest    DoRequestFunc
	Client       HTTPClient

	endpoint *url.URL
}

type DoRequestFunc func(req *http.Request, on1xxResponse func(code int, header textproto.MIMEHeader) error) (*http.Response, error)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func (c Client) SubmitUpload(data []byte, contentType, contentEncoding string) (*Upload, error) {
	resp, upload, err := c.createUpload(int64(len(data)), data, contentType, contentEncoding, "?1")
	defer closeResponseBody(resp)
	if err != nil {
		return nil, err
	}

	return upload, nil
}

func (c Client) InitiateUpload(length int64, data []byte, contentType, contentEncoding string) (*Upload, error) {
	if length < -1 {
		return nil, fmt.Errorf("invalid length: %d", length)
	}

	resp, upload, err := c.createUpload(int64(len(data)), data, contentType, contentEncoding, "?0")
	defer closeResponseBody(resp)
	if err != nil {
		return nil, err
	}

	return upload, nil
}

func (c Client) DeleteUpload(location string) error {
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

func (c Client) GetUpload(location string) (*Upload, error) {
	req, err := c.createRequest(http.MethodHead, location, dummy1xxCb)
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

	res := Upload{client: c}
	headers := []string{"Upload-Complete", "Upload-Length", "Upload-Limit", "Upload-Offset"}
	if err := processResponseHeaders(resp, headers, []string{"Upload-Length", "Upload-Offset"}, &res); err != nil {
		return nil, newTusError()
	}

	return &res, nil
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
	limits := resp.Header.Get("Upload-Limit")
	if limits == "" {
		return Limits{}, newTusError()
	}
	if err := res.UnmarshalText([]byte(limits)); err != nil {
		return Limits{}, fmt.Errorf("parse Upload-Limit header: %w", err)
	}

	return res, nil
}

func (c Client) createUpload(length int64, data []byte, contentType, contentEncoding, uploadComplete string) (*http.Response, *Upload, error) {
	res := Upload{length: length, client: c}

	req, err := c.createRequest(http.MethodPost, c.endpoint.String(), res.on1xxResponse)
	if err != nil {
		return nil, &res, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Upload-Complete", uploadComplete)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if contentEncoding != "" {
		req.Header.Set("Content-Encoding", contentEncoding)
	}

	resp, err := c.sendData(req, &res, length, data)
	if err != nil {
		return nil, &res, err
	}

	headers := []string{"Upload-Complete", "Upload-Limit", "Location"}
	if err := processResponseHeaders(resp, headers, []string{"Upload-Limit", "Location"}, &res); err != nil {
		return resp, &res, newTusError()
	}

	return resp, &res, nil
}

func (c Client) sendData(req *http.Request, upload *Upload, length int64, data []byte) (*http.Response, error) {
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

	if resp.StatusCode >= 400 {
		return nil, newTusError()
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
