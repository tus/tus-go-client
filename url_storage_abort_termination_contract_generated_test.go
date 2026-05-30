// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	generatedTusAbortTerminationContent      = "hello world"
	generatedTusAbortTerminationEndpointPath = "/uploads"
	generatedTusAbortTerminationFingerprint  = "contract-abort-terminate-fingerprint"
	generatedTusAbortTerminationPatchBody    = "hello world"
	generatedTusAbortTerminationPatchOffset  = "0"
	generatedTusAbortTerminationUploadLength = "11"
	generatedTusAbortTerminationUploadPath   = "/uploads/abort-terminate-contract"
)

var generatedTusAbortTerminationExpectedEvents = []string{"request-abort:1"}
var generatedTusAbortTerminationMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedAbortTerminatesKnownUpload(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	terminateOperation := generatedProtocolOperation("terminateTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusAbortTerminationMetadata)
	if err != nil {
		t.Fatal(err)
	}

	patchStarted := make(chan struct{})
	patchDone := make(chan struct{})
	terminationDone := make(chan struct{})
	requestErrs := make(chan error, 8)
	events := []string{}
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == generatedTusAbortTerminationEndpointPath && request.Method == createOperation.Method:
			recordRequestErr(generatedAssertTusAbortTerminationRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Upload-Metadata": encodedMetadata,
					"Upload-Length":   generatedTusAbortTerminationUploadLength,
				},
			))
			createResponse := generatedResponseFor(createOperation, 201)
			generatedWriteTusAbortTerminationResponseHeaders(
				responseWriter,
				createResponse,
				map[string]string{
					"Location": server.URL + generatedTusAbortTerminationUploadPath,
				},
			)
			responseWriter.WriteHeader(201)

		case request.URL.Path == generatedTusAbortTerminationUploadPath && request.Method == patchOperation.Method:
			defer close(patchDone)
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			if string(body) != generatedTusAbortTerminationPatchBody {
				recordRequestErr(fmt.Errorf("expected abort patch body %q, got %q", generatedTusAbortTerminationPatchBody, string(body)))
			}
			recordRequestErr(generatedAssertTusAbortTerminationRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					"Content-Type":  patchOperation.Request.ContentType,
					"Upload-Offset": generatedTusAbortTerminationPatchOffset,
				},
			))
			events = append(events, "request-abort:1")
			close(patchStarted)
			<-request.Context().Done()

		case request.URL.Path == generatedTusAbortTerminationUploadPath && request.Method == terminateOperation.Method:
			recordRequestErr(generatedAssertTusAbortTerminationRequestHeaders(
				request,
				terminateOperation,
				map[string]string{},
			))
			terminateResponse := generatedResponseFor(terminateOperation, 204)
			generatedWriteTusAbortTerminationResponseHeaders(
				responseWriter,
				terminateResponse,
				map[string]string{},
			)
			responseWriter.WriteHeader(204)
			close(terminationDone)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusAbortTerminationEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role, terminateOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	storage := NewMemoryURLStorage()
	go func() {
		_, err := client.UploadWithURLStorage(URLStorageUploadOptions{
			Context:                ctx,
			Storage:                storage,
			Source:                 strings.NewReader(generatedTusAbortTerminationContent),
			Fingerprint:            generatedTusAbortTerminationFingerprint,
			Size:                   11,
			Metadata:               generatedTusAbortTerminationMetadata,
			TerminateUploadOnAbort: true,
		})
		result <- err
	}()

	select {
	case <-patchStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for abort patch request")
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for abort termination result")
	}
	select {
	case <-patchDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server to observe abort")
	}
	select {
	case <-terminationDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for termination request")
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
	if !reflect.DeepEqual(events, generatedTusAbortTerminationExpectedEvents) {
		t.Fatalf("expected abort termination events %#v, got %#v", generatedTusAbortTerminationExpectedEvents, events)
	}

	storedUploads, err := storage.FindAllUploads()
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 0 {
		t.Fatalf("expected terminated abort to remove stored uploads, got %#v", storedUploads)
	}
}

func generatedAssertTusAbortTerminationRequestHeaders(
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

func generatedWriteTusAbortTerminationResponseHeaders(
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
}
