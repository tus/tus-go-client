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
	generatedTusDeferredLengthAcceptedOffset    = "11"
	generatedTusDeferredLengthContent           = "hello world"
	generatedTusDeferredLengthContentTypeHeader = "Content-Type"
	generatedTusDeferredLengthCreateDeferHeader = "Upload-Defer-Length"
	generatedTusDeferredLengthCreateDeferValue  = "1"
	generatedTusDeferredLengthEndpointPath      = "/uploads"
	generatedTusDeferredLengthEventPolicy       = "exact-except-allowed-extra-events"
	generatedTusDeferredLengthExpectedCreates   = 1
	generatedTusDeferredLengthExpectedPatches   = 1
	generatedTusDeferredLengthMetadataHeader    = "Upload-Metadata"
	generatedTusDeferredLengthPatchLength       = "11"
	generatedTusDeferredLengthPatchLengthHeader = "Upload-Length"
	generatedTusDeferredLengthPatchOffset       = "0"
	generatedTusDeferredLengthPatchOffsetHeader = "Upload-Offset"
	generatedTusDeferredLengthUploadPath        = "/uploads/deferred-contract"
)

var generatedTusDeferredLengthCreateAbsentHeaders = []string{"Upload-Length"}
var generatedTusDeferredLengthExtraEventPrefixes = []string{"progress:"}
var generatedTusDeferredLengthExpectedEvents = []string{"upload-url-available", "progress:0:11", "progress:11:11", "chunk-complete:11:11:11", "success", "source-close"}
var generatedTusDeferredLengthMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedURLStorageDeferredLengthUpload(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusDeferredLengthMetadata)
	if err != nil {
		t.Fatal(err)
	}

	createCount := 0
	patchCount := 0
	requestErrs := make(chan error, 6)
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == generatedTusDeferredLengthEndpointPath &&
			request.Method == createOperation.Method:
			createCount += 1
			recordRequestErr(generatedAssertTusDeferredLengthAbsentHeaders(
				request,
				generatedTusDeferredLengthCreateAbsentHeaders,
			))
			recordRequestErr(generatedAssertTusDeferredLengthRequestHeaders(
				request,
				createOperation,
				map[string]string{
					"Tus-Resumable":       "1.0.0",
					"Upload-Defer-Length": generatedTusDeferredLengthCreateDeferValue,
					"Upload-Metadata":     encodedMetadata,
				},
			))
			createResponse := generatedResponseFor(createOperation, 201)
			generatedWriteTusDeferredLengthResponseHeaders(
				responseWriter,
				createResponse,
				map[string]string{
					"Location":      server.URL + generatedTusDeferredLengthUploadPath,
					"Tus-Resumable": "1.0.0",
				},
			)
			responseWriter.WriteHeader(createResponse.StatusCode)

		case request.URL.Path == generatedTusDeferredLengthUploadPath &&
			request.Method == patchOperation.Method:
			patchCount += 1
			body, err := io.ReadAll(request.Body)
			recordRequestErr(err)
			if string(body) != generatedTusDeferredLengthContent {
				recordRequestErr(fmt.Errorf(
					"expected deferred upload body %q, got %q",
					generatedTusDeferredLengthContent,
					string(body),
				))
			}
			recordRequestErr(generatedAssertTusDeferredLengthRequestHeaders(
				request,
				patchOperation,
				map[string]string{
					"Content-Type":  patchOperation.Request.ContentType,
					"Tus-Resumable": "1.0.0",
					"Upload-Length": generatedTusDeferredLengthPatchLength,
					"Upload-Offset": generatedTusDeferredLengthPatchOffset,
				},
			))
			patchResponse := generatedResponseFor(patchOperation, 204)
			generatedWriteTusDeferredLengthResponseHeaders(
				responseWriter,
				patchResponse,
				map[string]string{
					"Tus-Resumable": "1.0.0",
					"Upload-Offset": generatedTusDeferredLengthAcceptedOffset,
				},
			)
			responseWriter.WriteHeader(patchResponse.StatusCode)

		default:
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusDeferredLengthEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role, generatedTusDeferredLengthExtension},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	events := []string{}
	source := &generatedTusDeferredLengthSource{
		Reader: strings.NewReader(generatedTusDeferredLengthContent),
		events: &events,
	}
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:              storage,
		Source:               source,
		Fingerprint:          "contract-deferred-length-fingerprint",
		Size:                 int64(len(generatedTusDeferredLengthContent)),
		Metadata:             generatedTusDeferredLengthMetadata,
		ChunkSize:            100,
		UploadLengthDeferred: true,
		EventHooks: UploadEventHooks{
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				events = append(events, generatedTusEventKeyProgress(
					generatedTusEventKeyNumber(bytesSent),
					generatedTusDeferredLengthBytesTotalString(bytesTotal),
				))
				return nil
			},
			OnChunkComplete: func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error {
				events = append(events, generatedTusEventKeyChunkComplete(
					generatedTusEventKeyNumber(chunkSize),
					generatedTusEventKeyNumber(bytesAccepted),
					generatedTusDeferredLengthBytesTotalString(bytesTotal),
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
		t.Fatal(err)
	}
	if upload.Location != server.URL+generatedTusDeferredLengthUploadPath {
		t.Fatalf("expected upload URL %s, got %s", server.URL+generatedTusDeferredLengthUploadPath, upload.Location)
	}
	if createCount != generatedTusDeferredLengthExpectedCreates || patchCount != generatedTusDeferredLengthExpectedPatches {
		t.Fatalf(
			"expected create=%d patch=%d, got create=%d patch=%d",
			generatedTusDeferredLengthExpectedCreates,
			generatedTusDeferredLengthExpectedPatches,
			createCount,
			patchCount,
		)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
	generatedTusAssertEvents(t, "deferredLengthUpload", generatedTusDeferredLengthEventPolicy, generatedTusDeferredLengthExtraEventPrefixes, generatedTusDeferredLengthExpectedEvents, events)

	storedUploads, err := storage.FindUploadsByFingerprint("contract-deferred-length-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 1 {
		t.Fatalf("expected deferred upload URL to be stored once, got %#v", storedUploads)
	}
}

type generatedTusDeferredLengthSource struct {
	*strings.Reader
	events *[]string
}

func (source *generatedTusDeferredLengthSource) Close() error {
	*source.events = append(*source.events, generatedTusEventKeySourceClose())
	return nil
}

func generatedTusDeferredLengthBytesTotalString(bytesTotal *int64) string {
	if bytesTotal == nil {
		return "null"
	}

	return fmt.Sprintf("%d", *bytesTotal)
}

func generatedAssertTusDeferredLengthAbsentHeaders(
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

func generatedAssertTusDeferredLengthRequestHeaders(
	request *http.Request,
	operation generatedTusProtocolOperation,
	values map[string]string,
) error {
	failures := []string{}
	for _, variant := range operation.Request.HeaderVariants {
		if err := generatedAssertTusDeferredLengthRequestHeaderVariant(request, variant, values); err != nil {
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

func generatedAssertTusDeferredLengthRequestHeaderVariant(
	request *http.Request,
	variant generatedTusHeaderVariant,
	values map[string]string,
) error {
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

func generatedWriteTusDeferredLengthResponseHeaders(
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
}
