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
	generatedTusCustomHeadersContent           = "hello world"
	generatedTusCustomHeadersContentType       = "application/offset+octet-stream"
	generatedTusCustomHeadersContentTypeHeader = "Content-Type"
	generatedTusCustomHeadersEndpointPath      = "/uploads"
	generatedTusCustomHeadersLength            = "11"
	generatedTusCustomHeadersLengthHeader      = "Upload-Length"
	generatedTusCustomHeadersMetadataHeader    = "Upload-Metadata"
	generatedTusCustomHeadersOffset            = "0"
	generatedTusCustomHeadersOffsetHeader      = "Upload-Offset"
	generatedTusCustomHeadersPath              = "/uploads/custom-headers-contract"
	generatedTusCustomHeadersAcceptedOffset    = "11"
)

var generatedTusCustomHeaders = map[string]string{"X-Tus-Contract": "custom-header", "X-Tus-Trace": "trace-123"}
var generatedTusCustomHeadersMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedURLStorageCustomRequestHeaders(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusCustomHeadersMetadata)
	if err != nil {
		t.Fatal(err)
	}

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
		case request.URL.Path == generatedTusCustomHeadersEndpointPath &&
			request.Method == createOperation.Method:
			requestCount += 1
			recordRequestErr(generatedAssertTusCustomRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Tus-Resumable":   "1.0.0",
					"Upload-Length":   generatedTusCustomHeadersLength,
					"Upload-Metadata": encodedMetadata,
					"X-Tus-Contract":  "custom-header",
					"X-Tus-Trace":     "trace-123",
				},
			))
			recordRequestErr(generatedAssertTusCustomHeaderValues(request, generatedTusCustomHeaders))
			createResponse := generatedResponseFor(createOperation, 201)
			generatedWriteTusCustomResponseHeaders(
				responseWriter,
				createResponse,
				map[string]string{
					"Location":      server.URL + generatedTusCustomHeadersPath,
					"Tus-Resumable": "1.0.0",
				},
			)
			responseWriter.WriteHeader(createResponse.StatusCode)

		case request.URL.Path == generatedTusCustomHeadersPath &&
			request.Method == patchOperation.Method:
			requestCount += 1
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			if string(body) != generatedTusCustomHeadersContent {
				recordRequestErr(fmt.Errorf(
					"expected custom-header upload body %q, got %q",
					generatedTusCustomHeadersContent,
					string(body),
				))
			}
			recordRequestErr(generatedAssertTusCustomRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					"Content-Type":   generatedTusCustomHeadersContentType,
					"Tus-Resumable":  "1.0.0",
					"Upload-Offset":  generatedTusCustomHeadersOffset,
					"X-Tus-Contract": "custom-header",
					"X-Tus-Trace":    "trace-123",
				},
			))
			recordRequestErr(generatedAssertTusCustomHeaderValues(request, generatedTusCustomHeaders))
			patchResponse := generatedResponseFor(patchOperation, 204)
			generatedWriteTusCustomResponseHeaders(
				responseWriter,
				patchResponse,
				map[string]string{
					"Tus-Resumable": "1.0.0",
					"Upload-Offset": generatedTusCustomHeadersAcceptedOffset,
				},
			)
			responseWriter.WriteHeader(patchResponse.StatusCode)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusCustomHeadersEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:     storage,
		Source:      strings.NewReader(generatedTusCustomHeadersContent),
		Fingerprint: "contract-custom-headers-fingerprint",
		Size:        11,
		Headers:     generatedTusCustomHeaders,
		Metadata:    generatedTusCustomHeadersMetadata,
	})
	if err != nil {
		select {
		case requestErr := <-requestErrs:
			t.Fatalf("%v: %v", err, requestErr)
		default:
			t.Fatal(err)
		}
	}
	if upload.Location != server.URL+generatedTusCustomHeadersPath {
		t.Fatalf("expected upload URL %s, got %s", server.URL+generatedTusCustomHeadersPath, upload.Location)
	}
	if upload.RemoteOffset != 11 {
		t.Fatalf("expected upload offset 11, got %d", upload.RemoteOffset)
	}
	if requestCount != 2 {
		t.Fatalf("expected custom-header create and patch requests, got %d", requestCount)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
}

func generatedAssertTusCustomHeaderValues(
	request *http.Request,
	expected map[string]string,
) error {
	for key, value := range expected {
		if actual := request.Header.Get(key); actual != value {
			return fmt.Errorf("expected custom header %s=%s, got %s", key, value, actual)
		}
	}

	return nil
}

func generatedAssertTusCustomRequestHeaders(
	request *http.Request,
	operation generatedTusProtocolOperation,
	values map[string]string,
) error {
	variant := operation.Request.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		expected := generatedTusRequestHeaderValue(values, field.DisplayName)
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

func generatedWriteTusCustomResponseHeaders(
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
		value := generatedTusResponseHeaderValue(values, field.DisplayName)
		responseWriter.Header().Set(field.DisplayName, value)
	}
	if value := values[generatedTusCustomHeadersOffsetHeader]; value != "" {
		responseWriter.Header().Set(generatedTusCustomHeadersOffsetHeader, value)
	}
}
