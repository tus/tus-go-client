// Package tusgo is a client library for TUS (https://tus.io), the resumable upload protocol over HTTP.
//
// The work is split between two types. [Client] manages the uploads on a server: it creates, deletes and queries them,
// concatenates the partial ones and reports the server capabilities. [UploadStream] transfers the data of a single
// upload; it implements [io.Writer], [io.ReaderFrom] and [io.Seeker].
// [Upload] represents a particular upload object on the server and its state.
//
// The basic approach is following: first, create a fixed-size upload object on the server by Client,
// and then transfer the data to the returned Upload using the UploadStream. To resume an interrupted transfer,
// just call the same method again.
//
// The protocol also defines a number of extensions -- optional features that customize this process:
// deleting an upload, concatenating several uploads into a single one or verifying the transferred data with a
// checksum, etc. A server supports any subset of them, so the features backed by an extension fail with
// [ErrUnsupportedFeature] unless the server announces it, see [Client.Capabilities].
package tusgo

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// NewClient returns a new [Client] that sends its requests to baseURL using the given HTTP client.
//
// If client is nil, [http.DefaultClient] is used.
func NewClient(client *http.Client, baseURL *url.URL) *Client {
	c := &Client{
		ProtocolVersion: "1.0.0",
		client:          client,
		ExtraHeaders:    make(http.Header),
		BaseURL:         baseURL,
	}
	if client == nil {
		c.client = http.DefaultClient
	}
	if baseURL == nil {
		c.BaseURL, _ = url.Parse("http://example.com/files")
	}
	return c
}

// Client manages the uploads on a TUS server: it creates, deletes and queries them, and concatenates partial uploads.
// It does not transfer the upload data itself, use [UploadStream] for that.
// Client is safe for concurrent use by multiple goroutines.
//
// Common errors returned by the Client methods are:
//
//   - [ErrUnsupportedFeature] -- the requested action needs a TUS extension that the server does not support
//   - [ErrProtocol] -- an otherwise successful server response contains malformed data, e.g. a missing required HTTP header
//   - [ErrUnexpectedResponse] -- the server responded with an unexpected status code
type Client struct {
	// BaseURL is the base URL the client sends its requests to.
	BaseURL *url.URL

	// ProtocolVersion is the TUS protocol version sent in the requests. Default is "1.0.0".
	ProtocolVersion string

	// Capabilities are the features and limits of the server. Use [Client.UpdateCapabilities] to query them from the server.
	Capabilities *ServerCapabilities

	// ExtraHeaders are the additional HTTP headers the client sends with every request. Ignored if [Client.GetRequest] is set.
	ExtraHeaders http.Header

	// GetRequest is called by the library to construct a new HTTP request. If nil, the library uses [http.NewRequest].
	GetRequest GetRequestFunc

	client *http.Client
	ctx    context.Context
}

// GetRequestFunc returns a request object the library will send. It receives the request method, the target URL and
// the request body, along with the Client and the [net/http.Client] on whose behalf the request is made.
type GetRequestFunc func(method, url string, body io.Reader, tusClient *Client, httpClient *http.Client) (*http.Request, error)

// WithContext returns a copy of the client that sends its requests with the given context.
func (c *Client) WithContext(ctx context.Context) *Client {
	res := *c
	res.ctx = ctx
	return &res
}

// GetUpload requests the information about the upload at the given location and stores it in u. It returns the server
// response and an error (if any).
//
// For a regular upload, it fills u with the remote offset, the Partial flag and the metadata. For a final concatenated
// upload it also fills in the upload size, if the server reports one; if the server is still concatenating the upload,
// the remote offset is set to [OffsetUnknown].
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) GetUpload(location string) (Upload, *http.Response, error) {
	res := Upload{Location: location}

	loc, err := url.Parse(location)
	if err != nil {
		return res, nil, fmt.Errorf("parse location: %w", err)
	}
	ref := c.BaseURL.ResolveReference(loc)

	req, err := c.createRequest(http.MethodHead, ref.String(), nil)
	if err != nil {
		return res, nil, fmt.Errorf("create request: %w", err)
	}
	response, err := c.tusRequest(c.ctx, req)
	if err != nil {
		return res, response, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		res.Partial = response.Header.Get("Upload-Concat") == "partial"
		uploadOffset := response.Header.Get("Upload-Offset")
		// Upload-Offset may not be present if final upload concatenation still in progress on server side
		if uploadOffset == "" {
			if response.Header.Get("Upload-Concat") != "final" {
				return res, response, newTusErrorWithErr(ErrProtocol, errors.New("lack of Upload-Offset required header in response"))
			}
			res.RemoteOffset = OffsetUnknown
		} else {
			if res.RemoteOffset, err = strconv.ParseInt(uploadOffset, 10, 64); err != nil {
				return res, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("parse Upload-Offset header %q: %w", uploadOffset, err))
			}
		}
		// Responses for final concatenated upload may contain Upload-Length header
		if v := response.Header.Get("Upload-Length"); v != "" {
			if res.RemoteSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				return res, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("parse Upload-Length header %q: %w", v, err))
			}
		}
		if v := response.Header.Get("Upload-Metadata"); v != "" {
			if res.Metadata, err = DecodeMetadata(v); err != nil {
				return res, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("parse Upload-Metadata header %q: %w", v, err))
			}
		}
		return res, response, nil
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		return res, response, newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	}

	return res, response, newTusErrorWithResponse(ErrUnexpectedResponse, response)
}

// CreateUpload creates an upload of the given size and metadata on the server and stores it in u. It returns the
// server response and an error (if any). The server must support the "creation" extension.
//
// If partial is true, the created upload is a partial one, suitable for a further concatenation. The keys of the
// metadata map must not contain spaces.
//
// If remoteSize is [SizeUnknown], the upload is created with a deferred size, i.e. with a size that is not known yet,
// but has to be known by the time the data transfer begins. The server must also support the
// "creation-defer-length" extension for this feature, and [UploadStream.SetUploadSize] must be set before the transfer.
//
// The method returns [ErrUploadTooLarge] if the upload size exceeds [ServerCapabilities.MaxSize] the server is able to
// accept.
func (c *Client) CreateUpload(remoteSize int64, partial bool, metadata map[string]string) (Upload, *http.Response, error) {
	var res Upload

	if err := c.ensureExtension("creation"); err != nil {
		return res, nil, err
	}

	req, err := c.createRequest(http.MethodPost, c.BaseURL.String(), nil)
	if err != nil {
		return res, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Length", strconv.FormatInt(0, 10))
	if partial {
		req.Header.Set("Upload-Concat", "partial")
	}
	switch {
	case remoteSize == SizeUnknown:
		if err = c.ensureExtension("creation-defer-length"); err != nil {
			return res, nil, err
		}
		req.Header.Set("Upload-Defer-Length", "1")
	case remoteSize > 0:
		req.Header.Set("Upload-Length", strconv.FormatInt(remoteSize, 10))
	default:
		panic(fmt.Sprintf("remoteSize is negative: %d", remoteSize))
	}

	if len(metadata) > 0 {
		var m string
		if m, err = EncodeMetadata(metadata); err != nil {
			return res, nil, fmt.Errorf("encode metadata: %w", err)
		}
		req.Header.Set("Upload-Metadata", m)
	}

	response, err := c.tusRequest(c.ctx, req)
	if err != nil {
		return res, response, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusCreated:
		res.Location = response.Header.Get("Location")
		res.Metadata = metadata
		res.Partial = partial
		res.RemoteSize = remoteSize
		if v := response.Header.Get("Upload-Expires"); v != "" {
			t, err := time.Parse(time.RFC1123, v)
			if err != nil {
				return res, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("parse RFC1123 datetime in Upload-Expires header %q: %w", v, err))
			}
			res.UploadExpired = &t
		}
		return res, response, nil
	case http.StatusRequestEntityTooLarge:
		return res, response, newTusErrorWithResponse(ErrUploadTooLarge, response)
	}

	return res, response, newTusErrorWithResponse(ErrUnexpectedResponse, response)
}

// CreateUploadWithData creates an upload on the server and transfers data in the same HTTP request. The created
// upload is stored in u. It returns the number of bytes uploaded, the server response and an error (if any).
// The whole data is sent in a single request, so the server must support the "creation-with-upload" extension.
//
// The remoteSize, partial and meta parameters have the same meaning as in CreateUpload.
//
// The method returns [ErrZeroProgress] if the server did not accept any data by some reason, more likely when it is
// busy and temporarily cannot accept the data. In that case, use [UploadStream] to transfer the remaining data.
func (c *Client) CreateUploadWithData(data []byte, remoteSize int64, partial bool, metadata map[string]string) (int64, Upload, *http.Response, error) {
	var res Upload

	if err := c.ensureExtension("creation-with-upload"); err != nil {
		return 0, res, nil, err
	}

	s := NewUploadStream(c, &res)
	s.ChunkSize = int64(len(data)) // Data must be uploaded in one request as single chunk
	s.uploadMethod = http.MethodPost
	headers := map[string]string{"Upload-Length": strconv.Itoa(int(remoteSize)), "Upload-Offset": ""}
	if partial {
		headers["Upload-Concat"] = "partial"
	}
	if len(metadata) > 0 {
		m, err := EncodeMetadata(metadata)
		if err != nil {
			return 0, res, nil, fmt.Errorf("encode metadata: %w", err)
		}
		headers["Upload-Metadata"] = m
	}
	res.RemoteSize = remoteSize
	res.Partial = partial
	res.Metadata = metadata

	uploadedBytes, response, err := s.doRequest(c.BaseURL.String(), data, nil, headers) // Upload in one request
	res.RemoteOffset = uploadedBytes
	if err != nil {
		return uploadedBytes, res, response, fmt.Errorf("send request: %w", err)
	}
	res.Location = response.Header.Get("Location")

	switch {
	case uploadedBytes == 0 && len(data) > 0:
		return uploadedBytes, res, response, newTusErrorWithErr(ErrZeroProgress, fmt.Errorf("offset=%d", res.RemoteOffset))
	case uploadedBytes < 0:
		return uploadedBytes, res, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("server offset has gone backwards, offset=%d, transferredDataSize=%d", uploadedBytes, len(data)))
	case uploadedBytes > int64(len(data)):
		return uploadedBytes, res, response, newTusErrorWithErr(ErrProtocol, fmt.Errorf("server offset has been advanced too far: offset=%d, transferredDataSize=%d", uploadedBytes, len(data)))
	}

	return uploadedBytes, res, response, nil
}

// DeleteUpload deletes the upload u from the server. It returns the server response and an error (if any).
// The server must support the "termination" extension.
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) DeleteUpload(u Upload) (*http.Response, error) {
	if err := c.ensureExtension("termination"); err != nil {
		return nil, err
	}

	loc, err := url.Parse(u.Location)
	if err != nil {
		return nil, fmt.Errorf("parse location: %w", err)
	}
	ref := c.BaseURL.ResolveReference(loc).String()
	req, err := c.createRequest(http.MethodDelete, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	response, err := c.tusRequest(c.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusNoContent:
		return response, nil
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		return response, newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	}
	return response, newTusErrorWithResponse(ErrUnexpectedResponse, response)
}

// ConcatenateUploads requests the server to concatenate the previously created partial uploads into a single final
// upload, which is stored in final. It returns the server response and an error (if any). The server must support the
// "concatenation" extension.
//
// The partial uploads are concatenated in the order they are given. Typically, they must be fully transferred to the
// server beforehand, unless the server supports the "concatenation-unfinished" extension and accepts unfinished ones.
func (c *Client) ConcatenateUploads(partials []Upload, meta map[string]string) (Upload, *http.Response, error) {
	var res Upload

	if len(partials) == 0 {
		panic("must be at least one partial upload to concatenate")
	}
	if err := c.ensureExtension("concatenation"); err != nil {
		return res, nil, err
	}

	req, err := c.createRequest(http.MethodPost, c.BaseURL.String(), nil)
	if err != nil {
		return res, nil, fmt.Errorf("create request: %w", err)
	}

	locations := make([]string, 0)
	for _, f := range partials {
		if !f.Partial {
			return res, nil, fmt.Errorf("upload %q is not partial", f.Location)
		}
		locations = append(locations, f.Location)
	}
	req.Header.Set("Upload-Concat", "final;"+strings.Join(locations, " "))

	if len(meta) > 0 {
		m, err := EncodeMetadata(meta)
		if err != nil {
			return res, nil, fmt.Errorf("encode metadata: %w", err)
		}
		req.Header.Set("Upload-Metadata", m)
	}

	response, err := c.tusRequest(c.ctx, req)
	if err != nil {
		return res, nil, fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusCreated:
		res.Location = response.Header.Get("Location")
		res.Metadata = meta
		return res, response, nil
	case http.StatusNotFound, http.StatusGone:
		return res, response, newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	}

	return res, response, newTusErrorWithResponse(ErrUnexpectedResponse, response)
}

// ConcatenateStreams requests the server to concatenate the streams into a single final upload, which is stored in final upload.
//
// If all streams are finished at the moment, then functions the same as [Client.ConcatenateUploads].
//
// If some streams are unfinished, the server is asked to concatenate them automatically once they will be finished.
// The server must support the "concatenation-unfinished" extension for that.
func (c *Client) ConcatenateStreams(streams []*UploadStream, meta map[string]string) (Upload, *http.Response, error) {
	if len(streams) == 0 {
		panic("must be at least one stream to concatenate")
	}

	uploads := make([]Upload, 0)
	for i, s := range streams {
		if s.Tell() < s.Len() {
			if err := c.ensureExtension("concatenation-unfinished"); err != nil {
				return Upload{}, nil, fmt.Errorf("stream #%d is not finished: %w", i, err)
			}
		}
		uploads = append(uploads, *s.Upload)
	}

	return c.ConcatenateUploads(uploads, meta)
}

// UpdateCapabilities queries the server for its features and limits and stores them in the [Client.Capabilities] field.
func (c *Client) UpdateCapabilities() (response *http.Response, err error) {
	var req *http.Request
	if req, err = c.createRequest(http.MethodOptions, c.BaseURL.String(), nil); err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if response, err = c.tusRequest(c.ctx, req); err != nil {
		return
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		c.Capabilities = &ServerCapabilities{}
		if v := response.Header.Get("Tus-Max-Size"); v != "" {
			if c.Capabilities.MaxSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Tus-Max-Size integer value %q: %w", v, err))
				return
			}
		}
		if v := response.Header.Get("Tus-Extension"); v != "" {
			c.Capabilities.Extensions = strings.Split(v, ",")
		}
		if v := response.Header.Get("Tus-Version"); v != "" {
			c.Capabilities.ProtocolVersions = strings.Split(v, ",")
		}
		if v := response.Header.Get("Tus-Checksum-Algorithm"); v != "" {
			c.Capabilities.ChecksumAlgorithms = strings.Split(v, ",")
		}
	default:
		err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
	}
	return
}

func (c *Client) tusRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodOptions && req.Header.Get("Tus-Resumable") == "" {
		req.Header.Set("Tus-Resumable", c.ProtocolVersion)
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var bodyBytes []byte
	bodyBytes, err = io.ReadAll(response.Body)
	if err != nil {
		return response, fmt.Errorf("read body: %w", err)
	}
	response.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if response.StatusCode == http.StatusPreconditionFailed {
		versions := response.Header.Get("Tus-Version")
		err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("request protocol version %q, server supported versions are %q", c.ProtocolVersion, versions))
		return response, err
	}

	return response, nil
}

func (c *Client) ensureExtension(extension string) error {
	if c.Capabilities == nil {
		if _, err := c.UpdateCapabilities(); err != nil {
			return fmt.Errorf("cannot obtain server capabilities: %w", err)
		}
	}
	for _, e := range c.Capabilities.Extensions {
		if extension == e {
			return nil
		}
	}
	return newTusErrorWithErr(ErrUnsupportedFeature, errors.New(extension))
}

func (c *Client) createRequest(method, url string, body io.Reader) (*http.Request, error) {
	if c.GetRequest != nil {
		return c.GetRequest(method, url, body, c, c.client)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if len(c.ExtraHeaders) > 0 {
		req.Header = c.ExtraHeaders.Clone()
	}
	return req, nil
}

// EncodeMetadata encodes a metadata map into the TUS `Upload-Metadata` header format.
func EncodeMetadata(metadata map[string]string) (string, error) {
	var encoded []string

	for k, v := range metadata {
		switch {
		case k == "":
			return "", fmt.Errorf("empty key")
		case strings.Contains(k, " "):
			return "", fmt.Errorf("key %q contains spaces", k)
		case strings.Contains(k, ","):
			return "", fmt.Errorf("key %q contains commas", k)
		}
		encoded = append(encoded, fmt.Sprintf("%s %s", k, base64.StdEncoding.EncodeToString([]byte(v))))
	}

	return strings.Join(encoded, ","), nil
}

// DecodeMetadata decodes a metadata map from the TUS `Upload-Metadata` header format.
func DecodeMetadata(raw string) (map[string]string, error) {
	res := make(map[string]string)
	for _, item := range strings.Split(raw, ",") {
		kv := strings.SplitN(item, " ", 2)
		if len(kv) <= 1 {
			return res, fmt.Errorf("metadata item %q has bad format", item)
		}
		val, err := base64.StdEncoding.DecodeString(kv[1])
		if err != nil {
			return res, err
		}
		res[kv[0]] = string(val)
	}

	return res, nil
}
