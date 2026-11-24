package rufh

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
)

func NewUpload(location string, client Client) (*Upload, error) {
	loc, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse location: %w", err)
	}

	return &Upload{
		location: loc,
		client:   client,
	}, nil
}

type Upload struct {
	Offset int64

	limits            *Limits
	location          *url.URL
	finalizedLocation *url.URL
	length            int64
	complete          bool

	client Client
}

func (u *Upload) Write(p []byte) (n int, err error) {
	if u.complete {
		return 0, newTusError() // TODO: return a specific error indicating that the upload is complete
	}
	oldOffset := u.Offset
	// TODO: check limits (and size if limits are valid?)

	resp, err := u.appendUpload(p, "?0")
	defer closeResponseBody(resp)
	if err != nil {
		return 0, err
	}

	switch resp.Header.Get("Upload-Complete") {
	case "":
		return int(u.Offset - oldOffset), newTusError()
	case "?1":
		return int(u.Offset - oldOffset), newTusError()
	}

	return int(u.Offset - oldOffset), nil
}

func (u *Upload) Finalize(p []byte) (n int, err error) {
	if u.complete {
		return 0, newTusError() // TODO: return a specific error indicating that the upload is complete
	}
	oldOffset := u.Offset
	// TODO: check limits (and size if limits are valid?)

	resp, err := u.appendUpload(p, "?1")
	defer closeResponseBody(resp)
	if err != nil {
		return 0, err
	}

	// TODO: is check for "?0" needed?
	switch resp.Header.Get("Upload-Complete") {
	case "":
		return int(u.Offset - oldOffset), newTusError()
	case "?0":
		return int(u.Offset - oldOffset), newTusError()
	}

	return int(u.Offset - oldOffset), nil
}

func (u *Upload) Reload() (int64, error) {
	res, err := u.client.GetUpload(u.location.String())
	if err != nil {
		return 0, err
	}

	u.Offset = res.Offset
	u.length = res.length
	u.complete = res.complete
	u.limits = res.limits
	u.finalizedLocation = res.finalizedLocation

	return u.Offset, nil
}

func (u Upload) Loaded() bool {
	return u.limits != nil
}

func (u *Upload) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64

	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = u.Offset + offset
	case io.SeekEnd:
		if u.length == -1 {
			return u.Offset, fmt.Errorf("cannot seek from end of upload with unknown size")
		}
		newOffset = u.length + offset
	default:
		return u.Offset, fmt.Errorf("unknown whence value: %d", whence)
	}
	if u.length != -1 && newOffset >= u.length {
		newOffset = u.length
	}
	if newOffset < 0 {
		return newOffset, fmt.Errorf("offset %d is negative", newOffset)
	}

	u.Offset = newOffset
	return newOffset, nil
}

func (u Upload) Length() int64 {
	return u.length
}

func (u *Upload) SetLength(size int64) error {
	if size < -1 {
		return fmt.Errorf("invalid length: %d", size)
	}
	if u.length != -1 {
		return fmt.Errorf("length is already set to %d", u.length)
	}

	u.length = size
	return nil
}

func (u Upload) Location() string {
	if u.location == nil {
		return ""
	}
	return u.location.String()
}

func (u Upload) FinalizedLocation() string {
	if u.finalizedLocation == nil {
		return ""
	}
	return u.finalizedLocation.String()
}

func (u Upload) Limits() (Limits, bool) {
	if u.limits == nil {
		return Limits{}, false
	}
	return *u.limits, true
}

func (u Upload) IsComplete() bool {
	return u.complete
}

func (u *Upload) appendUpload(p []byte, uploadComplete string) (*http.Response, error) {
	// TODO: check limits (and size if limits are valid?)

	loc := u.client.endpoint.ResolveReference(u.location)
	req, err := u.client.createRequest(http.MethodPatch, loc.String(), u.on1xxResponse)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/partial-upload")
	req.Header.Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	req.Header.Set("Upload-Complete", uploadComplete)

	resp, err := u.client.sendData(req, u, int64(len(p)), p)
	defer closeResponseBody(resp)
	if err != nil {
		return nil, err
	}
	// TODO: 409

	return resp, nil
}

func (u *Upload) on1xxResponse(code int, header textproto.MIMEHeader) error {
	if code != uploadResumptionSupportedStatusCode {
		return nil
	}

	seen := make(map[string]struct{})
	for k, v := range header {
		// TODO: white list of headers, other to ignore
		if _, ok := seen[k]; ok {
			return newTusError() // TODO: protocol error
		}
		seen[k] = struct{}{}
		if err := u.processResponseHeader(k, v[0]); err != nil {
			return fmt.Errorf("header %q: %w", k, err)
		}
	}

	return nil
}

func (u *Upload) processResponseHeader(key, value string) error {
	if value == "" {
		return errors.New("empty value")
	}

	switch key {
	case "Upload-Offset":
		offset, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		if offset < 0 {
			return fmt.Errorf("value is negative: %q", value)
		}
		u.Offset = offset
	case "Upload-Length":
		length, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		if length < 0 {
			return fmt.Errorf("value is negative: %q", value)
		}
		u.length = length
	case "Upload-Limit":
		var limits Limits
		if err := limits.UnmarshalText([]byte(value)); err != nil {
			return fmt.Errorf("parse: %w", err)
		}
		u.limits = &limits
	case "Location":
		loc, err := url.Parse(value)
		if err != nil {
			return fmt.Errorf("parse: %w", err)
		}
		u.location = loc
	case "Upload-Complete":
		switch value {
		case "?0":
			u.complete = false
		case "?1":
			u.complete = true
		default:
			return fmt.Errorf("unknown value: %q", value)
		}
	}

	return nil
}
