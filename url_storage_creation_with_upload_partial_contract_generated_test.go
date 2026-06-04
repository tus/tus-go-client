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
	generatedTusCreationPartialContent           = "hello world"
	generatedTusCreationPartialContentType       = "application/offset+octet-stream"
	generatedTusCreationPartialContentTypeHeader = "Content-Type"
	generatedTusCreationPartialCreateBodySize    = 5
	generatedTusCreationPartialEndpointPath      = "/uploads"
	generatedTusCreationPartialEventPolicy       = "exact-except-extra-progress"
	generatedTusCreationPartialLength            = "11"
	generatedTusCreationPartialLengthHeader      = "Upload-Length"
	generatedTusCreationPartialMetadataHeader    = "Upload-Metadata"
	generatedTusCreationPartialOffset            = "5"
	generatedTusCreationPartialOffsetHeader      = "Upload-Offset"
	generatedTusCreationPartialFirstPatchBody    = 5
	generatedTusCreationPartialFirstPatchOffset  = "5"
	generatedTusCreationPartialFirstPatchResult  = "10"
	generatedTusCreationPartialSecondPatchBody   = 1
	generatedTusCreationPartialSecondPatchOffset = "10"
	generatedTusCreationPartialPath              = "/uploads/creation-with-upload-partial-contract"
	generatedTusCreationPartialFinalOffset       = "11"
	generatedTusCreationPartialChunkSize         = 5
)

var generatedTusCreationPartialExpectedEvents = []string{"progress:0:11", "progress:5:11", "upload-url-available", "chunk-complete:5:5:11", "progress:5:11", "progress:10:11", "chunk-complete:5:10:11", "progress:10:11", "progress:11:11", "chunk-complete:1:11:11", "success", "source-close"}
var generatedTusCreationPartialMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedURLStorageCreationWithUploadPartialChunk(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusCreationPartialMetadata)
	if err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	patchRequestCount := 0
	requestErrs := make(chan error, 8)
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == generatedTusCreationPartialEndpointPath &&
			request.Method == createOperation.Method:
			requestCount += 1
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			expectedBody := generatedTusCreationPartialContent[:generatedTusCreationPartialCreateBodySize]
			if string(body) != expectedBody {
				recordRequestErr(fmt.Errorf(
					"expected partial creation body %q, got %q",
					expectedBody,
					string(body),
				))
			}
			if actual := request.Header.Get(generatedTusCreationPartialContentTypeHeader); actual != generatedTusCreationPartialContentType {
				recordRequestErr(fmt.Errorf(
					"expected partial creation content type %s, got %s",
					generatedTusCreationPartialContentType,
					actual,
				))
			}
			recordRequestErr(generatedAssertTusCreationPartialRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Content-Type":    generatedTusCreationPartialContentType,
					"Tus-Resumable":   "1.0.0",
					"Upload-Length":   generatedTusCreationPartialLength,
					"Upload-Metadata": encodedMetadata,
				},
			))
			createResponse := generatedResponseFor(createOperation, 201)
			generatedWriteTusCreationPartialResponseHeaders(
				responseWriter,
				createResponse,
				map[string]string{
					"Location":      server.URL + generatedTusCreationPartialPath,
					"Tus-Resumable": "1.0.0",
					"Upload-Offset": generatedTusCreationPartialOffset,
				},
			)
			responseWriter.WriteHeader(createResponse.StatusCode)

		case request.URL.Path == generatedTusCreationPartialPath &&
			request.Method == patchOperation.Method:
			requestCount += 1
			patchRequestCount += 1
			expectedBodyStart := generatedTusCreationPartialCreateBodySize
			expectedBodySize := generatedTusCreationPartialFirstPatchBody
			expectedOffset := generatedTusCreationPartialFirstPatchOffset
			responseOffset := generatedTusCreationPartialFirstPatchResult
			responseStatus := 204
			if patchRequestCount == 2 {
				expectedBodyStart += generatedTusCreationPartialFirstPatchBody
				expectedBodySize = generatedTusCreationPartialSecondPatchBody
				expectedOffset = generatedTusCreationPartialSecondPatchOffset
				responseOffset = generatedTusCreationPartialFinalOffset
				responseStatus = 204
			} else if patchRequestCount > 2 {
				recordRequestErr(fmt.Errorf("unexpected continuation request %d", patchRequestCount))
				responseWriter.WriteHeader(http.StatusNotFound)
				return
			}
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			expectedBodyEnd := expectedBodyStart + expectedBodySize
			expectedBody := generatedTusCreationPartialContent[expectedBodyStart:expectedBodyEnd]
			if len(expectedBody) != expectedBodySize {
				recordRequestErr(fmt.Errorf(
					"expected configured patch body size %d, got %d",
					expectedBodySize,
					len(expectedBody),
				))
			}
			if string(body) != expectedBody {
				recordRequestErr(fmt.Errorf(
					"expected continuation body %q, got %q",
					expectedBody,
					string(body),
				))
			}
			if actual := request.Header.Get(generatedTusCreationPartialContentTypeHeader); actual != generatedTusCreationPartialContentType {
				recordRequestErr(fmt.Errorf(
					"expected continuation content type %s, got %s",
					generatedTusCreationPartialContentType,
					actual,
				))
			}
			recordRequestErr(generatedAssertTusCreationPartialRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					"Content-Type":  generatedTusCreationPartialContentType,
					"Tus-Resumable": "1.0.0",
					"Upload-Offset": expectedOffset,
				},
			))
			patchResponse := generatedResponseFor(patchOperation, responseStatus)
			generatedWriteTusCreationPartialResponseHeaders(
				responseWriter,
				patchResponse,
				map[string]string{
					"Tus-Resumable": "1.0.0",
					"Upload-Offset": responseOffset,
				},
			)
			responseWriter.WriteHeader(patchResponse.StatusCode)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusCreationPartialEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role, generatedTusCreationWithUploadExtension},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	events := []string{}
	source := &generatedTusCreationPartialSource{
		Reader: strings.NewReader(generatedTusCreationPartialContent),
		events: &events,
	}
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:                  storage,
		Source:                   source,
		Fingerprint:              "contract-creation-with-upload-partial-fingerprint",
		Size:                     int64(len(generatedTusCreationPartialContent)),
		Metadata:                 generatedTusCreationPartialMetadata,
		ChunkSize:                int64(generatedTusCreationPartialChunkSize),
		UploadDataDuringCreation: true,
		EventHooks: UploadEventHooks{
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				events = append(events, generatedTusEventKeyProgress(
					generatedTusEventKeyNumber(bytesSent),
					generatedTusCreationPartialBytesTotalString(bytesTotal),
				))
				return nil
			},
			OnChunkComplete: func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error {
				events = append(events, generatedTusEventKeyChunkComplete(
					generatedTusEventKeyNumber(chunkSize),
					generatedTusEventKeyNumber(bytesAccepted),
					generatedTusCreationPartialBytesTotalString(bytesTotal),
				))
				return nil
			},
			OnUploadURLAvailable: func() error {
				events = append(events, generatedTusEventKeyUploadUrlAvailable())
				return nil
			},
			OnSuccess: func(UploadSuccessPayload) error {
				events = append(events, generatedTusEventKeySuccess())
				return nil
			},
		},
	})
	if err != nil {
		select {
		case requestErr := <-requestErrs:
			t.Fatalf("%v: %v", err, requestErr)
		default:
			t.Fatal(err)
		}
	}
	if upload.Location != server.URL+generatedTusCreationPartialPath {
		t.Fatalf("expected upload URL %s, got %s", server.URL+generatedTusCreationPartialPath, upload.Location)
	}
	if upload.RemoteOffset != int64(len(generatedTusCreationPartialContent)) {
		t.Fatalf("expected upload offset %d, got %d", len(generatedTusCreationPartialContent), upload.RemoteOffset)
	}
	if requestCount != 3 {
		t.Fatalf("expected one creation request and two continuation requests, got %d", requestCount)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
	generatedTusAssertEvents(t, "creationWithUploadPartialChunk", generatedTusCreationPartialEventPolicy, generatedTusCreationPartialExpectedEvents, events)

	storedUploads, err := storage.FindUploadsByFingerprint("contract-creation-with-upload-partial-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 1 {
		t.Fatalf("expected partial creation URL to be stored once, got %#v", storedUploads)
	}
}

type generatedTusCreationPartialSource struct {
	*strings.Reader
	events *[]string
}

func (source *generatedTusCreationPartialSource) Close() error {
	*source.events = append(*source.events, generatedTusEventKeySourceClose())
	return nil
}

func generatedTusCreationPartialBytesTotalString(bytesTotal *int64) string {
	if bytesTotal == nil {
		return "null"
	}

	return fmt.Sprintf("%d", *bytesTotal)
}

func generatedAssertTusCreationPartialRequestHeaders(
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

func generatedWriteTusCreationPartialResponseHeaders(
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
	if value := values[generatedTusCreationPartialOffsetHeader]; value != "" {
		responseWriter.Header().Set(generatedTusCreationPartialOffsetHeader, value)
	}
}
