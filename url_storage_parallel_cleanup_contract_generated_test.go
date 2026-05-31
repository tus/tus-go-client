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
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	generatedTusParallelCleanupContent           = "hello world"
	generatedTusParallelCleanupContentType       = "application/offset+octet-stream"
	generatedTusParallelCleanupContentTypeHeader = "Content-Type"
	generatedTusParallelCleanupEndpointPath      = "/uploads"
	generatedTusParallelCleanupFailurePartIndex  = 0
	generatedTusParallelCleanupFailureStatus     = 500
	generatedTusParallelCleanupMethod            = "POST"
	generatedTusParallelCleanupOffsetHeader      = "Upload-Offset"
	generatedTusParallelCleanupOverrideHeader    = "X-HTTP-Method-Override"
	generatedTusParallelCleanupOverrideValue     = "PATCH"
	generatedTusParallelCleanupUploadCount       = 2
)

var generatedTusParallelCleanupHeaders = map[string]string{"X-Tus-Contract": "parallel-cleanup-policy", "X-Tus-Trace": "parallel-cleanup-trace-123"}
var generatedTusParallelCleanupMetadataForPartialUploads = map[string]string{"test": "world"}
var generatedTusParallelCleanupPartPatchBodies = []string{"hello", " world"}
var generatedTusParallelCleanupPartPatchOffsets = []string{"0", "0"}
var generatedTusParallelCleanupPartUploadLengths = []string{"5", "6"}
var generatedTusParallelCleanupPartUploadPaths = []string{"/uploads/parallel-cleanup-part-1", "/uploads/parallel-cleanup-part-2"}
var generatedTusParallelCleanupTerminatePaths = []string{"/uploads/parallel-cleanup-part-1", "/uploads/parallel-cleanup-part-2"}

func TestGeneratedURLStorageParallelUploadCleanup(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	terminateOperation := generatedProtocolOperation("terminateTusUpload")
	encodedPartialMetadata, err := EncodeMetadata(generatedTusParallelCleanupMetadataForPartialUploads)
	if err != nil {
		t.Fatal(err)
	}

	var requestMu sync.Mutex
	createIndex := 0
	patchIndex := 0
	terminateIndex := 0
	terminatedParts := map[int]bool{}
	patchArrivals := make(chan int, generatedTusParallelCleanupUploadCount)
	releasePatches := make(chan struct{})
	requestErrs := make(chan error, 12)
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}
	go generatedTusReleaseParallelCleanupPatchesAfterAllStarted(
		patchArrivals,
		releasePatches,
		requestErrs,
	)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == generatedTusParallelCleanupEndpointPath &&
			request.Method == createOperation.Method &&
			request.Header.Get("Upload-Concat") == "partial":
			partIndex := generatedTusParallelCleanupPartIndexForUploadLength(
				request.Header.Get("Upload-Length"),
			)
			if partIndex < 0 {
				recordRequestErr(fmt.Errorf(
					"unexpected cleanup create upload length %s",
					request.Header.Get("Upload-Length"),
				))
				responseWriter.WriteHeader(http.StatusBadRequest)
				return
			}
			requestMu.Lock()
			createIndex += 1
			requestMu.Unlock()
			recordRequestErr(generatedAssertTusParallelCleanupRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Upload-Concat":   "partial",
					"Upload-Metadata": encodedPartialMetadata,
					"Upload-Length":   generatedTusParallelCleanupPartUploadLengths[partIndex],
				},
			))
			recordRequestErr(generatedAssertTusParallelCleanupCustomHeaders(
				request,
				generatedTusParallelCleanupHeaders,
			))
			createResponse := generatedResponseFor(createOperation, http.StatusCreated)
			generatedWriteTusParallelCleanupResponseHeaders(
				responseWriter,
				createResponse,
				map[string]string{
					"Location": server.URL + generatedTusParallelCleanupPartUploadPaths[partIndex],
				},
			)
			responseWriter.WriteHeader(createResponse.StatusCode)

		case request.Method == generatedTusParallelCleanupMethod:
			partIndex := generatedTusParallelCleanupPartIndexForPath(request.URL.Path)
			if partIndex < 0 {
				recordRequestErr(fmt.Errorf("unexpected cleanup patch path %s", request.URL.Path))
				responseWriter.WriteHeader(http.StatusNotFound)
				return
			}
			select {
			case patchArrivals <- partIndex:
			case <-request.Context().Done():
				recordRequestErr(request.Context().Err())
				return
			}
			select {
			case <-releasePatches:
			case <-request.Context().Done():
				recordRequestErr(request.Context().Err())
				return
			}
			requestMu.Lock()
			patchIndex += 1
			requestMu.Unlock()
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			if string(body) != generatedTusParallelCleanupPartPatchBodies[partIndex] {
				recordRequestErr(fmt.Errorf(
					"expected cleanup patch body %q, got %q",
					generatedTusParallelCleanupPartPatchBodies[partIndex],
					string(body),
				))
			}
			recordRequestErr(generatedAssertTusParallelCleanupRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					generatedTusParallelCleanupContentTypeHeader: generatedTusParallelCleanupContentType,
					generatedTusParallelCleanupOffsetHeader:      generatedTusParallelCleanupPartPatchOffsets[partIndex],
					generatedTusParallelCleanupOverrideHeader:    generatedTusParallelCleanupOverrideValue,
				},
			))
			recordRequestErr(generatedAssertTusParallelCleanupCustomHeaders(
				request,
				generatedTusParallelCleanupHeaders,
			))
			if actual := request.Header.Get(generatedTusParallelCleanupOverrideHeader); actual != generatedTusParallelCleanupOverrideValue {
				recordRequestErr(fmt.Errorf("expected override header %s, got %s", generatedTusParallelCleanupOverrideValue, actual))
			}
			if partIndex == generatedTusParallelCleanupFailurePartIndex {
				responseWriter.WriteHeader(generatedTusParallelCleanupFailureStatus)
				return
			}
			select {
			case <-request.Context().Done():
				return
			case <-time.After(2 * time.Second):
				recordRequestErr(fmt.Errorf("expected cleanup patch request to be canceled"))
				responseWriter.WriteHeader(http.StatusInternalServerError)
			}

		case request.Method == terminateOperation.Method:
			partIndex := generatedTusParallelCleanupPartIndexForTerminatePath(request.URL.Path)
			if partIndex < 0 {
				recordRequestErr(fmt.Errorf("unexpected cleanup termination path %s", request.URL.Path))
				responseWriter.WriteHeader(http.StatusNotFound)
				return
			}
			requestMu.Lock()
			terminateIndex += 1
			terminatedParts[partIndex] = true
			requestMu.Unlock()
			recordRequestErr(generatedAssertTusParallelCleanupRequestHeaders(
				request,
				terminateOperation,
				map[string]string{},
			))
			recordRequestErr(generatedAssertTusParallelCleanupCustomHeaders(
				request,
				generatedTusParallelCleanupHeaders,
			))
			if actual := request.Header.Get(generatedTusParallelCleanupOverrideHeader); actual != "" {
				recordRequestErr(fmt.Errorf("expected no override header on cleanup termination request, got %s", actual))
			}
			responseWriter.WriteHeader(http.StatusNoContent)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusParallelCleanupEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role, terminateOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	_, err = client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:                   storage,
		Source:                    strings.NewReader(generatedTusParallelCleanupContent),
		Fingerprint:               "contract-parallel-cleanup-fingerprint",
		Size:                      int64(len(generatedTusParallelCleanupContent)),
		Headers:                   generatedTusParallelCleanupHeaders,
		MetadataForPartialUploads: generatedTusParallelCleanupMetadataForPartialUploads,
		OverridePatchMethod:       true,
		ParallelUploads:           generatedTusParallelCleanupUploadCount,
		TerminateUploadOnAbort:    true,
	})
	if err == nil {
		t.Fatal("expected parallel cleanup upload to fail")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("expected original parallel part failure, got %v", err)
	}

	requestMu.Lock()
	actualCreateIndex := createIndex
	actualPatchIndex := patchIndex
	actualTerminateIndex := terminateIndex
	actualTerminatedParts := len(terminatedParts)
	requestMu.Unlock()
	if actualCreateIndex != generatedTusParallelCleanupUploadCount {
		t.Fatalf("expected %d partial creates, got %d", generatedTusParallelCleanupUploadCount, actualCreateIndex)
	}
	if actualPatchIndex != generatedTusParallelCleanupUploadCount {
		t.Fatalf("expected %d partial patches, got %d", generatedTusParallelCleanupUploadCount, actualPatchIndex)
	}
	if actualTerminateIndex != generatedTusParallelCleanupUploadCount {
		t.Fatalf("expected %d partial terminations, got %d", generatedTusParallelCleanupUploadCount, actualTerminateIndex)
	}
	if actualTerminatedParts != generatedTusParallelCleanupUploadCount {
		t.Fatalf("expected all partial uploads to be terminated, got %#v", terminatedParts)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}

	storedUploads, err := storage.FindUploadsByFingerprint("contract-parallel-cleanup-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 0 {
		t.Fatalf("expected no final parallel upload to be stored, got %#v", storedUploads)
	}
}

func generatedTusParallelCleanupPartIndexForPath(path string) int {
	for index, candidate := range generatedTusParallelCleanupPartUploadPaths {
		if path == candidate {
			return index
		}
	}

	return -1
}

func generatedTusParallelCleanupPartIndexForTerminatePath(path string) int {
	for index, candidate := range generatedTusParallelCleanupTerminatePaths {
		if path == candidate {
			return index
		}
	}

	return -1
}

func generatedTusParallelCleanupPartIndexForUploadLength(uploadLength string) int {
	for index, candidate := range generatedTusParallelCleanupPartUploadLengths {
		if uploadLength == candidate {
			return index
		}
	}

	return -1
}

func generatedTusReleaseParallelCleanupPatchesAfterAllStarted(
	patchArrivals <-chan int,
	releasePatches chan<- struct{},
	requestErrs chan<- error,
) {
	seen := map[int]bool{}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for len(seen) < generatedTusParallelCleanupUploadCount {
		select {
		case partIndex := <-patchArrivals:
			seen[partIndex] = true
		case <-timer.C:
			requestErrs <- fmt.Errorf("expected all cleanup PATCH requests to be in flight")
			close(releasePatches)
			return
		}
	}

	close(releasePatches)
}

func generatedAssertTusParallelCleanupRequestHeaders(
	request *http.Request,
	operation generatedTusProtocolOperation,
	values map[string]string,
) error {
	failures := []string{}
	for _, variant := range operation.Request.HeaderVariants {
		if err := generatedAssertTusParallelCleanupRequestHeaderVariant(request, variant, values); err != nil {
			failures = append(failures, err.Error())
			continue
		}

		return nil
	}

	return fmt.Errorf(
		"no %s request header variant matched: %s",
		operation.OperationID,
		strings.Join(failures, "; "),
	)
}

func generatedAssertTusParallelCleanupRequestHeaderVariant(
	request *http.Request,
	variant generatedTusHeaderVariant,
	values map[string]string,
) error {
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

func generatedAssertTusParallelCleanupCustomHeaders(
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

func generatedWriteTusParallelCleanupResponseHeaders(
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
