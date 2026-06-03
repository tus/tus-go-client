// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	generatedTusOverrideContent           = "hello world"
	generatedTusOverrideContentType       = "application/offset+octet-stream"
	generatedTusOverrideContentTypeHeader = "Content-Type"
	generatedTusOverrideHeaderName        = "X-HTTP-Method-Override"
	generatedTusOverrideHeaderValue       = "PATCH"
	generatedTusOverrideMethod            = "POST"
	generatedTusOverrideOffset            = "3"
	generatedTusOverrideOffsetHeader      = "Upload-Offset"
	generatedTusOverridePath              = "/uploads/override-contract"
	generatedTusOverrideUploadLength      = "11"
	generatedTusOverrideLengthHeader      = "Upload-Length"
	generatedTusOverrideFinalOffset       = "11"
	generatedTusOverridePatchBody         = "lo world"
)

func TestGeneratedURLStorageOverridePatchMethod(t *testing.T) {
	getOperation := generatedProtocolOperation("getTusUploadOffset")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	requestCount := 0
	requestErrs := make(chan error, 8)
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == generatedTusOverridePath &&
			request.Method == getOperation.Method:
			requestCount += 1
			if actual := request.Header.Get(generatedTusOverrideHeaderName); actual != "" {
				recordRequestErr(fmt.Errorf("expected no override header on offset request, got %s", actual))
			}
			getResponse := generatedResponseFor(getOperation, 200)
			generatedWriteTusOverrideResponseHeaders(
				responseWriter,
				getResponse,
				map[string]string{
					generatedTusOverrideLengthHeader: generatedTusOverrideUploadLength,
					generatedTusOverrideOffsetHeader: generatedTusOverrideOffset,
				},
			)
			responseWriter.WriteHeader(getResponse.StatusCode)

		case request.URL.Path == generatedTusOverridePath &&
			request.Method == generatedTusOverrideMethod:
			requestCount += 1
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			if string(body) != generatedTusOverridePatchBody {
				recordRequestErr(fmt.Errorf(
					"expected override patch body %q, got %q",
					generatedTusOverridePatchBody,
					string(body),
				))
			}
			recordRequestErr(generatedAssertTusOverrideRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					generatedTusOverrideContentTypeHeader: generatedTusOverrideContentType,
					generatedTusOverrideHeaderName:        generatedTusOverrideHeaderValue,
					generatedTusOverrideOffsetHeader:      generatedTusOverrideOffset,
				},
			))
			patchResponse := generatedResponseFor(patchOperation, 204)
			generatedWriteTusOverrideResponseHeaders(
				responseWriter,
				patchResponse,
				map[string]string{
					generatedTusOverrideOffsetHeader: generatedTusOverrideFinalOffset,
				},
			)
			responseWriter.WriteHeader(patchResponse.StatusCode)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		ProtocolVersions: []string{DefaultProtocolVersion},
	}
	storage := NewMemoryURLStorage()
	if _, err := storage.AddUpload(
		"contract-override-fingerprint",
		URLStorageUpload{"uploadUrl": server.URL + generatedTusOverridePath},
	); err != nil {
		t.Fatal(err)
	}

	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:             storage,
		Source:              strings.NewReader(generatedTusOverrideContent),
		Fingerprint:         "contract-override-fingerprint",
		Size:                11,
		OverridePatchMethod: true,
	})
	if err != nil {
		select {
		case requestErr := <-requestErrs:
			t.Fatalf("%v: %v", err, requestErr)
		default:
			t.Fatal(err)
		}
	}
	if upload.Location != server.URL+generatedTusOverridePath {
		t.Fatalf("expected upload URL %s, got %s", server.URL+generatedTusOverridePath, upload.Location)
	}
	if upload.RemoteOffset != 11 {
		t.Fatalf("expected upload offset 11, got %d", upload.RemoteOffset)
	}
	if requestCount != 2 {
		t.Fatalf("expected one offset request and one overridden patch request, got %d", requestCount)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
}

func generatedAssertTusOverrideRequestHeaders(
	request *http.Request,
	operation generatedTusProtocolOperation,
	values map[string]string,
) error {
	variant := operation.Request.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		expected := values[field.DisplayName]
		if expected == "" {
			expected = DefaultProtocolVersion
		}
		if actual := request.Header.Get(field.DisplayName); actual != expected {
			return fmt.Errorf(
				"expected request header %s=%s, got %s",
				field.DisplayName,
				expected,
				actual,
			)
		}
	}

	return nil
}

func generatedWriteTusOverrideResponseHeaders(
	responseWriter http.ResponseWriter,
	contract generatedTusResponseContract,
	values map[string]string,
) {
	if len(contract.HeaderVariants) == 0 {
		return
	}
	variant := contract.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := values[field.DisplayName]
		if value == "" {
			value = DefaultProtocolVersion
		}
		responseWriter.Header().Set(field.DisplayName, value)
	}
	if value := values[generatedTusOverrideOffsetHeader]; value != "" {
		responseWriter.Header().Set(generatedTusOverrideOffsetHeader, value)
	}
}
