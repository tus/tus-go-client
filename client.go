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
		GetRequest:      newRequest,
		client:          client,
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

	// GetRequest is called by the library every time a new request object is needed. Replace it to adjust the
	// outgoing requests, e.g. to add the authentication headers. By default, it returns a bare http.Request.
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
func (c *Client) GetUpload(u *Upload, location string) (response *http.Response, err error) {
	if u == nil {
		panic("u is nil")
	}

	var loc *url.URL
	if loc, err = url.Parse(location); err != nil {
		return
	}
	ref := c.BaseURL.ResolveReference(loc).String()

	var req *http.Request
	if req, err = c.GetRequest(http.MethodHead, ref, nil, c, c.client); err != nil {
		return
	}
	if response, err = c.tusRequest(c.ctx, req); err != nil {
		return
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		u2 := Upload{}
		u2.Location = location
		u2.Partial = response.Header.Get("Upload-Concat") == "partial"

		uploadOffset := response.Header.Get("Upload-Offset")
		// Upload-Offset may not be present if final upload concatenation still in progress on server side
		if uploadOffset == "" {
			if response.Header.Get("Upload-Concat") != "final" {
				err = newTusErrorWithErr(ErrProtocol, errors.New("lack of Upload-Offset required header in response"))
				return
			}
			u2.RemoteOffset = OffsetUnknown
		} else {
			if u2.RemoteOffset, err = strconv.ParseInt(uploadOffset, 10, 64); err != nil {
				err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Upload-Offset header %q: %w", uploadOffset, err))
				return
			}
		}
		// Responses for final concatenated upload may contain Upload-Length header
		if v := response.Header.Get("Upload-Length"); v != "" {
			if u2.RemoteSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Upload-Length header %q: %w", v, err))
				return
			}
		}
		if v := response.Header.Get("Upload-Metadata"); v != "" {
			if u2.Metadata, err = DecodeMetadata(v); err != nil {
				err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Upload-Metadata header %q: %w", v, err))
			}
		}
		*u = u2
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		err = newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	default:
		err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
	}
	return
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
func (c *Client) CreateUpload(u *Upload, remoteSize int64, partial bool, meta map[string]string) (response *http.Response, err error) {
	if u == nil {
		panic("u is nil")
	}
	if err = c.ensureExtension("creation"); err != nil {
		return
	}

	var req *http.Request
	if req, err = c.GetRequest(http.MethodPost, c.BaseURL.String(), nil, c, c.client); err != nil {
		return
	}

	req.Header.Set("Content-Length", strconv.FormatInt(0, 10))
	if partial {
		req.Header.Set("Upload-Concat", "partial")
	}
	switch {
	case remoteSize == SizeUnknown:
		if err = c.ensureExtension("creation-defer-length"); err != nil {
			return
		}
		req.Header.Set("Upload-Defer-Length", "1")
	case remoteSize > 0:
		req.Header.Set("Upload-Length", strconv.FormatInt(remoteSize, 10))
	default:
		panic(fmt.Sprintf("remoteSize is negative: %d", remoteSize))
	}

	if len(meta) > 0 {
		var m string
		if m, err = EncodeMetadata(meta); err != nil {
			return
		}
		req.Header.Set("Upload-Metadata", m)
	}

	if response, err = c.tusRequest(c.ctx, req); err != nil {
		return
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusCreated:
		u2 := Upload{}
		u2.Location = response.Header.Get("Location")
		u2.Metadata = meta
		u2.Partial = partial
		u2.RemoteSize = remoteSize
		if v := response.Header.Get("Upload-Expires"); v != "" {
			var t time.Time
			if t, err = time.Parse(time.RFC1123, v); err != nil {
				err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("cannot parse Upload-Expires RFC1123 header %q: %w", v, err))
				return
			}
			u2.UploadExpired = &t
		}
		*u = u2
	case http.StatusRequestEntityTooLarge:
		err = newTusErrorWithResponse(ErrUploadTooLarge, response)
	default:
		err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
	}

	return
}

// CreateUploadWithData creates an upload on the server and transfers data in the same HTTP request. The created
// upload is stored in u. It returns the number of bytes uploaded, the server response and an error (if any).
// The whole data is sent in a single request, so the server must support the "creation-with-upload" extension.
//
// The remoteSize, partial and meta parameters have the same meaning as in CreateUpload.
func (c *Client) CreateUploadWithData(u *Upload, data []byte, remoteSize int64, partial bool, meta map[string]string) (uploadedBytes int64, response *http.Response, err error) {
	if err = c.ensureExtension("creation-with-upload"); err != nil {
		return
	}
	u2 := Upload{}
	s := NewUploadStream(c, &u2)
	s.ChunkSize = int64(len(data)) // Data must be uploaded in one request
	s.uploadMethod = http.MethodPost
	headers := map[string]string{"Upload-Length": strconv.Itoa(int(remoteSize)), "Upload-Offset": ""}
	if partial {
		headers["Upload-Concat"] = "partial"
	}
	if len(meta) > 0 {
		var m string
		if m, err = EncodeMetadata(meta); err != nil {
			return
		}
		headers["Upload-Metadata"] = m
	}
	u2.RemoteSize = remoteSize
	u2.Partial = partial
	u2.Metadata = meta

	rd := bytes.NewReader(data)
	s.setupDirtyBuffer()
	uploadedBytes, _, response, err = s.uploadChunkImpl(c.BaseURL.String(), rd, headers) // Upload in one request
	if err == nil {
		u2.Location = response.Header.Get("Location")
		u2.RemoteOffset = uploadedBytes
		*u = u2
	}

	return
}

// DeleteUpload deletes the upload u from the server. It returns the server response and an error (if any).
// The server must support the "termination" extension.
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) DeleteUpload(u Upload) (response *http.Response, err error) {
	if err = c.ensureExtension("termination"); err != nil {
		return
	}

	var req *http.Request
	var loc *url.URL
	if loc, err = url.Parse(u.Location); err != nil {
		return
	}
	ref := c.BaseURL.ResolveReference(loc).String()
	if req, err = c.GetRequest(http.MethodDelete, ref, nil, c, c.client); err != nil {
		return
	}
	if response, err = c.tusRequest(c.ctx, req); err != nil {
		return
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusNoContent:
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		err = newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	default:
		err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
	}

	return
}

// ConcatenateUploads requests the server to concatenate the previously created partial uploads into a single final
// upload, which is stored in final. It returns the server response and an error (if any). The server must support the
// "concatenation" extension.
//
// The partial uploads are concatenated in the order they are given. Typically, they must be fully transferred to the
// server beforehand, unless the server supports the "concatenation-unfinished" extension and accepts unfinished ones.
func (c *Client) ConcatenateUploads(final *Upload, partials []Upload, meta map[string]string) (response *http.Response, err error) {
	if final == nil {
		panic("final is nil")
	}
	if len(partials) == 0 {
		panic("must be at least one partial upload to concatenate")
	}
	if err = c.ensureExtension("concatenation"); err != nil {
		return
	}

	var req *http.Request
	if req, err = c.GetRequest(http.MethodPost, c.BaseURL.String(), nil, c, c.client); err != nil {
		return
	}

	locations := make([]string, 0)
	for _, f := range partials {
		if !f.Partial {
			return nil, fmt.Errorf("upload %q is not partial", f.Location)
		}
		locations = append(locations, f.Location)
	}
	req.Header.Set("Upload-Concat", "final;"+strings.Join(locations, " "))

	if len(meta) > 0 {
		var m string
		if m, err = EncodeMetadata(meta); err != nil {
			return
		}
		req.Header.Set("Upload-Metadata", m)
	}

	if response, err = c.tusRequest(c.ctx, req); err != nil {
		return
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusCreated:
		u2 := Upload{}
		u2.Location = response.Header.Get("Location")
		u2.Metadata = meta
		*final = u2
	case http.StatusNotFound, http.StatusGone:
		err = newTusErrorWithResponse(ErrUploadDoesNotExist, response)
	default:
		err = newTusErrorWithResponse(ErrUnexpectedResponse, response)
	}
	return
}

// ConcatenateStreams requests the server to concatenate the streams into a single final upload, which is stored in final upload.
//
// If all streams are finished at the moment, then functions the same as [Client.ConcatenateUploads].
//
// If some streams are unfinished, the server is asked to concatenate them automatically once they will be finished.
// The server must support the "concatenation-unfinished" extension for that.
func (c *Client) ConcatenateStreams(final *Upload, streams []*UploadStream, meta map[string]string) (response *http.Response, err error) {
	if len(streams) == 0 {
		panic("must be at least one stream to concatenate")
	}

	uploads := make([]Upload, 0)
	for i, s := range streams {
		if s.Tell() < s.Len() {
			if err = c.ensureExtension("concatenation-unfinished"); err != nil {
				return nil, fmt.Errorf("stream #%d is not finished: %w", i, err)
			}
		}
		uploads = append(uploads, *s.Upload)
	}

	return c.ConcatenateUploads(final, uploads, meta)
}

// UpdateCapabilities queries the server for its features and limits and stores them in the [Client.Capabilities] field.
func (c *Client) UpdateCapabilities() (response *http.Response, err error) {
	var req *http.Request
	if req, err = c.GetRequest(http.MethodOptions, c.BaseURL.String(), nil, c, c.client); err != nil {
		return
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

func (c *Client) tusRequest(ctx context.Context, req *http.Request) (response *http.Response, err error) {
	if req.Method != http.MethodOptions && req.Header.Get("Tus-Resumable") == "" {
		req.Header.Set("Tus-Resumable", c.ProtocolVersion)
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	response, err = c.client.Do(req)
	if err == nil && response.StatusCode == http.StatusPreconditionFailed {
		versions := response.Header.Get("Tus-Version")
		err = newTusErrorWithErr(ErrProtocol, fmt.Errorf("request protocol version %q, server supported versions are %q", c.ProtocolVersion, versions))
		return
	}
	if response != nil && response.Body != nil {
		var bodyBytes []byte
		bodyBytes, err = io.ReadAll(response.Body)
		if err != nil {
			return
		}
		response.Body.Close() // Close the original body
		response.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return
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

// EncodeMetadata encodes a metadata map into the TUS `Upload-Metadata` header format.
func EncodeMetadata(metadata map[string]string) (string, error) {
	var encoded []string

	for k, v := range metadata {
		if strings.Contains(k, " ") {
			return "", fmt.Errorf("key %q contains spaces", k)
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

func newRequest(method, url string, body io.Reader, tusClient *Client, _ *http.Client) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}
