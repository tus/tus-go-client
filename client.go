// Package tusgo is a client library for TUS (https://tus.io), the resumable upload protocol over HTTP.
package tusgo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const TUSv1ProtocolVersion = "1.0.0"

// NewClient returns a new [Client] that sends its requests to the given endpoint URL.
func NewClient(endpoint *url.URL) *Client {
	if endpoint == nil {
		panic("endpoint is nil")
	}
	return &Client{
		ProtocolVersion: TUSv1ProtocolVersion,
		ExtraHeaders:    make(http.Header),
		endpoint:        endpoint,
		mu:              &sync.Mutex{},
	}
}

// Client manages the uploads on a TUS server: it creates, deletes and queries them, and concatenates partial uploads.
// It does not transfer the upload data itself, use [ChunkedWriter] or [Stream] for that.
// Client is safe for concurrent use by multiple goroutines.
//
// Common errors returned by the Client methods are:
//
//   - [ErrUnsupportedFeature] -- the requested action needs a TUS extension that the server does not support
//   - [ErrProtocol] -- an otherwise successful server response contains malformed data, e.g. a missing required HTTP header
//   - [ErrUnexpectedResponse] -- the server responded with an unexpected status code
type Client struct {
	// ProtocolVersion is the TUS protocol version sent in the requests. By default, it's equal to [TUSv1ProtocolVersion].
	ProtocolVersion string

	// ExtraHeaders are the additional HTTP headers sent with every request to the server.
	// They may be used for authentication, cache control, etc.
	ExtraHeaders http.Header

	// DoRequest is called on every request, that sends a prepared TUS [http.Request] to the server and returns the
	// [http.Response]. It may be called concurrently if the Client is used by several goroutines.
	// If nil, the library uses the Client field or [http.DefaultClient].
	//
	// This function may be used for testing, logging, collecting the metrics, shaping the connection speed,
	// modifying the request before sending it, etc.
	DoRequest DoRequestFunc

	// Client is the HTTP client used to send requests to the server. If nil, [http.DefaultClient] is used.
	// This field is ignored if DoRequest is set.
	Client *http.Client

	endpoint             *url.URL
	advertisedExtensions []string
	mu                   *sync.Mutex
}

// DoRequestFunc is the callback type that can be set on Client and will be called on every request to the server.
// Function receives a prepared TUS [http.Request] and returns the [http.Response] or an error.
// It must not close the response body, it will be closed by the library after processing.
// The function should be safe for concurrent use if the Client is used by several goroutines.
type DoRequestFunc func(req *http.Request) (*http.Response, error)

// GetUpload requests the server for the upload information by its location.
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) GetUpload(location string) (Upload, error) {
	res := Upload{Location: location}

	loc, err := url.Parse(location)
	if err != nil {
		return res, fmt.Errorf("parse location: %w", err)
	}
	loc = c.endpoint.ResolveReference(loc)

	req, err := c.createRequest(http.MethodHead, loc.String())
	if err != nil {
		return res, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return res, fmt.Errorf("tus request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		res.Partial = resp.Header.Get("Upload-Concat") == "partial"
		uploadOffset := resp.Header.Get("Upload-Offset")
		// Upload-Offset may not be present if final upload concatenation still in progress on server side
		if uploadOffset == "" {
			if resp.Header.Get("Upload-Concat") != "final" {
				return res, newTusError(ErrProtocol, resp, errors.New("lack of Upload-Offset required header in response"))
			}
			res.RemoteOffset = OffsetUnknown
		} else {
			if res.RemoteOffset, err = strconv.ParseInt(uploadOffset, 10, 64); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Offset header %q: %w", uploadOffset, err))
			}
		}
		// Responses for final concatenated upload may contain Upload-Length header
		if v := resp.Header.Get("Upload-Length"); v != "" {
			if res.RemoteSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Length header %q: %w", v, err))
			}
		}
		if v := resp.Header.Get("Upload-Metadata"); v != "" {
			if res.Metadata, err = DecodeMetadata(v); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Upload-Metadata header %q: %w", v, err))
			}
		}
		return res, nil
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		return res, newTusError(ErrUploadDoesNotExist, resp, nil)
	}

	return res, newTusError(ErrUnexpectedResponse, resp, nil)
}

// SizeUnknown is passed to Client.CreateUpload in remoteSize arg to create an upload with deferred size.
// Such upload is created without size and accepts as much data as the server is able to accept.
// The actual size is reported later by a separate request, see DeclareSize stream method.
//
// Server must support the "creation-defer-length" extension for this feature.
const SizeUnknown = -1

// CreateUpload creates an upload of the given size, and returns an [Upload] object.
// The server must advertise the "creation" extension.
//
// If remoteSize is [SizeUnknown], the upload is created with a deferred size, i.e. with a size that is not known yet
// and may be set later by DeclareSize method in stream.
// The server must also advertise the "creation-defer-length" extension for this feature.
//
// If partial arg is true, the upload marks as "partial", which may be used to upload data in parallel and meant
// to be concatenated to "final" upload further, see [Client.ConcatenatePartialUploads].
// The given metadata will be attached to the upload and can be retrieved later.
//
// The method returns [ErrTooLarge] if the upload size exceeds [ServerCapabilities.MaxSize] the server is able to
// accept.
func (c *Client) CreateUpload(remoteSize int64, partial bool, metadata map[string]string) (Upload, error) {
	var res Upload

	if err := c.ensureExtension("creation"); err != nil {
		return res, err
	}

	req, err := c.createRequest(http.MethodPost, c.endpoint.String())
	if err != nil {
		return res, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Length", strconv.FormatInt(0, 10))
	if partial {
		req.Header.Set("Upload-Concat", "partial")
	}
	switch {
	case remoteSize == SizeUnknown:
		if err = c.ensureExtension("creation-defer-length"); err != nil {
			return res, err
		}
		req.Header.Set("Upload-Defer-Length", "1")
	case remoteSize > 0:
		req.Header.Set("Upload-Length", strconv.FormatInt(remoteSize, 10))
	default:
		return res, fmt.Errorf("remoteSize is negative: %d", remoteSize)
	}

	if len(metadata) > 0 {
		var m string
		if m, err = EncodeMetadata(metadata); err != nil {
			return res, fmt.Errorf("encode metadata: %w", err)
		}
		req.Header.Set("Upload-Metadata", m)
	}

	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return res, fmt.Errorf("tus request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		res.Location = resp.Header.Get("Location")
		res.Metadata = metadata
		res.Partial = partial
		res.RemoteSize = remoteSize
		if v := resp.Header.Get("Upload-Expires"); v != "" {
			t, err := time.Parse(time.RFC1123, v)
			if err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse RFC1123 datetime in Upload-Expires header %q: %w", v, err))
			}
			res.UploadExpired = &t
		}
		return res, nil
	case http.StatusRequestEntityTooLarge:
		return res, newTusError(ErrTooLarge, resp, nil)
	}

	return res, newTusError(ErrUnexpectedResponse, resp, nil)
}

// CreateUploadWithData creates an upload on the server and transfers data in a single HTTP request.
// It returns the number of bytes been uploaded and the [Upload] object.
// Server must advertise the "creation-with-upload" extension.
//
// The remoteSize, partial and metadata parameters have the same meaning as in CreateUpload.
//
// If the server successfully create the upload, but did not accept all data, the method returns a correct Upload
// and [ErrIncompleteWrite] error. In that case, use [ChunkedWriter] or [Stream] to transfer the remaining data.
func (c *Client) CreateUploadWithData(data []byte, remoteSize int64, partial bool, metadata map[string]string) (int64, Upload, error) {
	var res Upload

	if err := c.ensureExtension("creation-with-upload"); err != nil {
		return 0, res, err
	}

	s, err := NewStream(c, res)
	if err != nil {
		return 0, res, fmt.Errorf("create upload stream: %w", err)
	}
	headers := map[string]string{"Upload-Length": strconv.Itoa(int(remoteSize)), "Upload-Offset": ""}
	if partial {
		headers["Upload-Concat"] = "partial"
	}
	if len(metadata) > 0 {
		m, err := EncodeMetadata(metadata)
		if err != nil {
			return 0, res, fmt.Errorf("encode metadata: %w", err)
		}
		headers["Upload-Metadata"] = m
	}
	res.RemoteSize = remoteSize
	res.Partial = partial
	res.Metadata = metadata

	newOffset, expires, resp, err := s.doRequest(http.MethodPost, c.endpoint.String(), data, nil, 0, headers) // Upload in one request
	res.RemoteOffset = newOffset
	res.UploadExpired = expires
	defer closeResponseBody(resp)
	if err != nil {
		return newOffset, res, fmt.Errorf("request: %w", err)
	}
	res.Location = resp.Header.Get("Location")

	switch {
	case newOffset < 0:
		return newOffset, res, newTusError(ErrProtocol, resp, fmt.Errorf("server offset is negative, offset=%d, transferredDataSize=%d", newOffset, len(data)))
	case newOffset > int64(len(data)):
		return newOffset, res, newTusError(ErrProtocol, resp, fmt.Errorf("server offset has been advanced too far, offset=%d, transferredDataSize=%d", newOffset, len(data)))
	case newOffset < int64(len(data)) && len(data) > 0:
		return newOffset, res, newTusError(ErrIncompleteWrite, resp, fmt.Errorf("server did not accepted some data, accepted=%d, offset=%d", newOffset, res.RemoteOffset))
	}

	return newOffset, res, nil
}

// DeleteUpload deletes the upload at the given location from the server. The server must advertise the
// "termination" extension.
//
// The method returns [ErrUploadDoesNotExist] if no such upload has been found on the server.
func (c *Client) DeleteUpload(location string) error {
	if err := c.ensureExtension("termination"); err != nil {
		return err
	}

	loc, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("parse location: %w", err)
	}
	loc = c.endpoint.ResolveReference(loc)
	req, err := c.createRequest(http.MethodDelete, loc.String())
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return fmt.Errorf("tus request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound, http.StatusGone, http.StatusForbidden:
		return newTusError(ErrUploadDoesNotExist, resp, nil)
	}
	return newTusError(ErrUnexpectedResponse, resp, nil)
}

// ConcatenatePartialUploads creates a "final" upload whose data is the concatenation of the "partial" uploads at
// partialLocations, in the given order, and returns its location. The optional metadata is attached to the final
// upload. The metadata of the partial uploads, if any, is discarded. The final upload will not be able to accept any data.
//
// Partial uploads are typically used to transfer the data in parallel. The approach is following: create several
// "partial" uploads on the server, transfer in parallel the data split by chunks equal to these uploads,
// and then concatenate them into a single "final" upload.
//
// The same "partial" upload may be used in multiple concatenations and may be used multiple times in the same
// concatenation. Also, the server may delete the partial uploads after the concatenation. All of this depends
// on server settings.
//
// If all partial uploads are full, the server may start the concatenation immediately. In this case, only
// the "concatenation" extension is required to be advertised by the server.
//
// If transferring to any of the partial uploads is unfinished, the server starts the concatenation automatically
// once all of them are finished. In this case, the server must advertise the "concatenation-unfinished" extension.
//
// If server does not support "concatenation" nor "concatenation-unfinished" extension, the method returns [ErrUnsupportedFeature].
//
// The method returns [ErrUploadDoesNotExist] if any of the partial uploads has not been found on the server.
func (c *Client) ConcatenatePartialUploads(partialLocations []string, metadata map[string]string) (string, error) {
	if len(partialLocations) == 0 {
		return "", fmt.Errorf("locations list is empty")
	}

	err1 := c.ensureExtension("concatenation")
	err2 := c.ensureExtension("concatenation-unfinished")
	if err1 != nil && err2 != nil {
		return "", newTusError(ErrUnsupportedFeature, nil, fmt.Errorf("server should support either concatenation or concatenation-unfinished extension"))
	}

	req, err := c.createRequest(http.MethodPost, c.endpoint.String())
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Upload-Concat", "final;"+strings.Join(partialLocations, " "))
	if len(metadata) > 0 {
		m, err := EncodeMetadata(metadata)
		if err != nil {
			return "", fmt.Errorf("encode metadata: %w", err)
		}
		req.Header.Set("Upload-Metadata", m)
	}

	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return "", fmt.Errorf("tus request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		location := resp.Header.Get("Location")
		if location == "" {
			return "", newTusError(ErrProtocol, resp, errors.New("empty location header"))
		}
		return location, nil
	case http.StatusNotFound, http.StatusGone:
		return "", newTusError(ErrUploadDoesNotExist, resp, nil)
	}

	return "", newTusError(ErrUnexpectedResponse, resp, nil)
}

// FetchCapabilities queries the server for its capabilities. See [ServerCapabilities] for the details.
func (c *Client) FetchCapabilities() (ServerCapabilities, error) {
	return c.fetchCapabilities()
}

func (c *Client) fetchCapabilities() (ServerCapabilities, error) {
	var res ServerCapabilities

	req, err := c.createRequest(http.MethodOptions, c.endpoint.String())
	if err != nil {
		return res, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.tusRequest(req)
	defer closeResponseBody(resp)
	if err != nil {
		return res, fmt.Errorf("tus request: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		if v := resp.Header.Get("Tus-Max-Size"); v != "" {
			if res.MaxSize, err = strconv.ParseInt(v, 10, 64); err != nil {
				return res, newTusError(ErrProtocol, resp, fmt.Errorf("parse Tus-Max-Size integer value %q: %w", v, err))
			}
		}
		if v := resp.Header.Get("Tus-Extension"); v != "" {
			res.Extensions = strings.Split(v, ",")
		}
		if v := resp.Header.Get("Tus-Version"); v != "" {
			res.ProtocolVersions = strings.Split(v, ",")
		}
		if v := resp.Header.Get("Tus-Checksum-Algorithm"); v != "" {
			res.ChecksumAlgorithms = strings.Split(v, ",")
		}
		return res, nil
	}

	return res, newTusError(ErrUnexpectedResponse, resp, nil)
}

func (c *Client) tusRequest(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodOptions && req.Header.Get("Tus-Resumable") == "" {
		req.Header.Set("Tus-Resumable", c.ProtocolVersion)
	}

	f := c.DoRequest
	if f == nil {
		cl := c.Client
		if cl == nil {
			cl = http.DefaultClient
		}
		f = cl.Do
	}
	resp, err := f(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}

	if resp.StatusCode == http.StatusPreconditionFailed {
		versions := resp.Header.Get("Tus-Version")
		return resp, newTusError(ErrProtocol, resp, fmt.Errorf("server does not support protocol version %q, supported versions: %s", c.ProtocolVersion, versions))
	}

	return resp, nil
}

func (c *Client) ensureExtension(extension string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.advertisedExtensions == nil {
		caps, err := c.fetchCapabilities()
		if err != nil {
			return fmt.Errorf("fetch capabilities: %w", err)
		}
		c.advertisedExtensions = caps.Extensions
		if c.advertisedExtensions == nil {
			c.advertisedExtensions = make([]string, 0)
		}
	}
	if !slices.Contains(c.advertisedExtensions, extension) {
		return newTusError(ErrUnsupportedFeature, nil, errors.New(extension))
	}

	return nil
}

func (c *Client) createRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
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
		// The space separating the key and the value may be omitted if the value is empty
		key, encoded, _ := strings.Cut(item, " ")
		if key == "" {
			return res, fmt.Errorf("metadata item %q has bad format", item)
		}
		val, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return res, err
		}
		res[key] = string(val)
	}

	return res, nil
}

func closeResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
}
