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
	"reflect"
	"strings"
	"testing"
)

const (
	generatedTusParallelConcatExtension        = "concatenation"
	generatedTusParallelContent                = "hello world"
	generatedTusParallelEndpointPath           = "/uploads"
	generatedTusParallelFinalConcatPrefix      = "final;"
	generatedTusParallelFinalPath              = "/uploads/parallel-final"
	generatedTusParallelUploadURLSeparator     = " "
	generatedTusParallelConformanceUploadCount = 2
)

var generatedTusParallelExpectedEvents = []string{"progress:5:11", "chunk-complete:5:5:11", "progress:11:11", "chunk-complete:6:11:11"}
var generatedTusParallelFinalAbsentHeaders = []string{"Upload-Length"}
var generatedTusParallelMetadata = map[string]string{"foo": "hello"}
var generatedTusParallelMetadataForPartialUploads = map[string]string{"test": "world"}
var generatedTusParallelPartPatchAcceptedOffsets = []string{"5", "6"}
var generatedTusParallelPartPatchBodies = []string{"hello", " world"}
var generatedTusParallelPartPatchOffsets = []string{"0", "0"}
var generatedTusParallelPartUploadLengths = []string{"5", "6"}
var generatedTusParallelPartUploadPaths = []string{"/uploads/parallel-part-1", "/uploads/parallel-part-2"}

func TestGeneratedURLStorageParallelUploadConcatFlow(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusParallelMetadata)
	if err != nil {
		t.Fatal(err)
	}
	encodedPartialMetadata, err := EncodeMetadata(generatedTusParallelMetadataForPartialUploads)
	if err != nil {
		t.Fatal(err)
	}

	createIndex := 0
	patchIndex := 0
	requestErrs := make(chan error, 8)
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == generatedTusParallelEndpointPath &&
			request.Method == createOperation.Method &&
			createIndex < len(generatedTusParallelPartUploadPaths):
			partIndex := createIndex
			createIndex += 1
			recordRequestErr(generatedAssertTusParallelRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Upload-Concat":   "partial",
					"Upload-Metadata": encodedPartialMetadata,
					"Upload-Length":   generatedTusParallelPartUploadLengths[partIndex],
				},
			))
			createResponse := generatedResponseFor(createOperation, http.StatusCreated)
			generatedWriteTusParallelResponseHeaders(
				responseWriter,
				createResponse,
				map[string]string{
					"Location": server.URL + generatedTusParallelPartUploadPaths[partIndex],
				},
			)
			responseWriter.WriteHeader(createResponse.StatusCode)

		case request.URL.Path == generatedTusParallelEndpointPath &&
			request.Method == createOperation.Method &&
			createIndex == len(generatedTusParallelPartUploadPaths):
			createIndex += 1
			recordRequestErr(generatedAssertTusParallelAbsentHeaders(
				request,
				generatedTusParallelFinalAbsentHeaders,
			))
			recordRequestErr(generatedAssertTusParallelRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Upload-Concat":   generatedTusParallelFinalConcatHeader(server.URL),
					"Upload-Metadata": encodedMetadata,
				},
			))
			finalResponse := generatedResponseFor(createOperation, 201)
			generatedWriteTusParallelResponseHeaders(
				responseWriter,
				finalResponse,
				map[string]string{
					"Location": server.URL + generatedTusParallelFinalPath,
				},
			)
			responseWriter.WriteHeader(finalResponse.StatusCode)

		case request.Method == patchOperation.Method:
			partIndex := generatedTusParallelPartIndexForPath(request.URL.Path)
			if partIndex < 0 {
				recordRequestErr(fmt.Errorf("unexpected parallel patch path %s", request.URL.Path))
				responseWriter.WriteHeader(http.StatusNotFound)
				return
			}
			patchIndex += 1
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			if string(body) != generatedTusParallelPartPatchBodies[partIndex] {
				recordRequestErr(fmt.Errorf(
					"expected parallel patch body %q, got %q",
					generatedTusParallelPartPatchBodies[partIndex],
					string(body),
				))
			}
			recordRequestErr(generatedAssertTusParallelRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					"Content-Type":  patchOperation.Request.ContentType,
					"Upload-Offset": generatedTusParallelPartPatchOffsets[partIndex],
				},
			))
			patchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
			generatedWriteTusParallelResponseHeaders(
				responseWriter,
				patchResponse,
				map[string]string{
					"Upload-Offset": generatedTusParallelPartPatchAcceptedOffsets[partIndex],
				},
			)
			responseWriter.WriteHeader(patchResponse.StatusCode)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusParallelEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role, generatedTusParallelConcatExtension},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	events := []string{}
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:                   storage,
		Source:                    strings.NewReader(generatedTusParallelContent),
		Fingerprint:               "contract-parallel-fingerprint",
		Size:                      int64(len(generatedTusParallelContent)),
		Metadata:                  generatedTusParallelMetadata,
		MetadataForPartialUploads: generatedTusParallelMetadataForPartialUploads,
		ParallelUploads:           generatedTusParallelConformanceUploadCount,
		EventHooks: UploadEventHooks{
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				events = append(events, fmt.Sprintf(
					"progress:%d:%s",
					bytesSent,
					generatedTusParallelBytesTotalString(bytesTotal),
				))
				return nil
			},
			OnChunkComplete: func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error {
				events = append(events, fmt.Sprintf(
					"chunk-complete:%d:%d:%s",
					chunkSize,
					bytesAccepted,
					generatedTusParallelBytesTotalString(bytesTotal),
				))
				return nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if upload.Location != server.URL+generatedTusParallelFinalPath {
		t.Fatalf("expected final upload URL %s, got %s", server.URL+generatedTusParallelFinalPath, upload.Location)
	}
	if createIndex != len(generatedTusParallelPartUploadPaths)+1 {
		t.Fatalf("expected %d create requests, got %d", len(generatedTusParallelPartUploadPaths)+1, createIndex)
	}
	if patchIndex != len(generatedTusParallelPartUploadPaths) {
		t.Fatalf("expected %d patch requests, got %d", len(generatedTusParallelPartUploadPaths), patchIndex)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
	if !reflect.DeepEqual(events, generatedTusParallelExpectedEvents) {
		t.Fatalf("expected parallel events %#v, got %#v", generatedTusParallelExpectedEvents, events)
	}

	storedUploads, err := storage.FindUploadsByFingerprint("contract-parallel-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 1 {
		t.Fatalf("expected final parallel upload to be stored once, got %#v", storedUploads)
	}
	storedUploadURL, ok := stringFromURLStorageUpload(storedUploads[0], "uploadUrl")
	if !ok || storedUploadURL != upload.Location {
		t.Fatalf("expected stored final upload URL %s, got %#v", upload.Location, storedUploads[0])
	}
}

func generatedTusParallelFinalConcatHeader(serverURL string) string {
	locations := make([]string, 0, len(generatedTusParallelPartUploadPaths))
	for _, path := range generatedTusParallelPartUploadPaths {
		locations = append(locations, serverURL+path)
	}

	return generatedTusParallelFinalConcatPrefix +
		strings.Join(locations, generatedTusParallelUploadURLSeparator)
}

func generatedTusParallelPartIndexForPath(path string) int {
	for index, candidate := range generatedTusParallelPartUploadPaths {
		if path == candidate {
			return index
		}
	}

	return -1
}

func generatedTusParallelBytesTotalString(bytesTotal *int64) string {
	if bytesTotal == nil {
		return "null"
	}

	return fmt.Sprintf("%d", *bytesTotal)
}

func generatedAssertTusParallelAbsentHeaders(
	request *http.Request,
	headers []string,
) error {
	for _, header := range headers {
		if actual := request.Header.Get(header); actual != "" {
			return fmt.Errorf("expected request header %s to be absent, got %s", header, actual)
		}
	}

	return nil
}

func generatedAssertTusParallelRequestHeaders(
	request *http.Request,
	operation generatedTusProtocolOperation,
	values map[string]string,
) error {
	failures := []string{}
	for _, variant := range operation.Request.HeaderVariants {
		if err := generatedAssertTusParallelRequestHeaderVariant(request, variant, values); err != nil {
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

func generatedAssertTusParallelRequestHeaderVariant(
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

func generatedWriteTusParallelResponseHeaders(
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
