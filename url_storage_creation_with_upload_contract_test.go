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
	generatedTusCreationWithUploadContent           = "hello world"
	generatedTusCreationWithUploadContentType       = "application/offset+octet-stream"
	generatedTusCreationWithUploadContentTypeHeader = "Content-Type"
	generatedTusCreationWithUploadEndpointPath      = "/uploads"
	generatedTusCreationWithUploadEventPolicy       = "exact-except-allowed-extra-events"
	generatedTusCreationWithUploadExpectedRequests  = 1
	generatedTusCreationWithUploadLength            = "11"
	generatedTusCreationWithUploadLengthHeader      = "Upload-Length"
	generatedTusCreationWithUploadMetadataHeader    = "Upload-Metadata"
	generatedTusCreationWithUploadOffset            = "11"
	generatedTusCreationWithUploadOffsetHeader      = "Upload-Offset"
	generatedTusCreationWithUploadPath              = "/uploads/creation-with-upload-contract"
)

var generatedTusCreationWithUploadExtraEventPrefixes = []string{"progress:"}
var generatedTusCreationWithUploadExpectedEvents = []string{"progress:0:11", "progress:11:11", "upload-url-available", "success", "source-close"}
var generatedTusCreationWithUploadMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedURLStorageCreationWithUpload(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusCreationWithUploadMetadata)
	if err != nil {
		t.Fatal(err)
	}

	requestCount := 0
	requestErrs := make(chan error, 4)
	recordRequestErr := func(err error) {
		if err != nil {
			requestErrs <- err
		}
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != generatedTusCreationWithUploadEndpointPath ||
			request.Method != createOperation.Method {
			recordRequestErr(fmt.Errorf("unexpected request %s %s", request.Method, request.URL.Path))
			responseWriter.WriteHeader(http.StatusNotFound)
			return
		}
		requestCount += 1
		body, err := io.ReadAll(request.Body)
		recordRequestErr(err)
		if string(body) != generatedTusCreationWithUploadContent {
			recordRequestErr(fmt.Errorf(
				"expected creation-with-upload body %q, got %q",
				generatedTusCreationWithUploadContent,
				string(body),
			))
		}
		if actual := request.Header.Get(generatedTusCreationWithUploadContentTypeHeader); actual != generatedTusCreationWithUploadContentType {
			recordRequestErr(fmt.Errorf(
				"expected creation-with-upload content type %s, got %s",
				generatedTusCreationWithUploadContentType,
				actual,
			))
		}
		recordRequestErr(generatedAssertTusCreationWithUploadRequestHeaders(
			request,
			createOperation,
			map[string]string{
				"Content-Type":    generatedTusCreationWithUploadContentType,
				"Tus-Resumable":   "1.0.0",
				"Upload-Length":   generatedTusCreationWithUploadLength,
				"Upload-Metadata": encodedMetadata,
			},
		))
		createResponse := generatedResponseFor(createOperation, 201)
		generatedWriteTusCreationWithUploadResponseHeaders(
			responseWriter,
			createResponse,
			map[string]string{
				"Location":      server.URL + generatedTusCreationWithUploadPath,
				"Tus-Resumable": "1.0.0",
				"Upload-Offset": generatedTusCreationWithUploadOffset,
			},
		)
		responseWriter.WriteHeader(createResponse.StatusCode)
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusCreationWithUploadEndpointPath)
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
	source := &generatedTusCreationWithUploadSource{
		Reader: strings.NewReader(generatedTusCreationWithUploadContent),
		events: &events,
	}
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:                  storage,
		Source:                   source,
		Fingerprint:              "contract-creation-with-upload-fingerprint",
		Size:                     int64(len(generatedTusCreationWithUploadContent)),
		Metadata:                 generatedTusCreationWithUploadMetadata,
		UploadDataDuringCreation: true,
		EventHooks: UploadEventHooks{
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				events = append(events, generatedTusEventKeyProgress(
					generatedTusEventKeyNumber(bytesSent),
					generatedTusCreationWithUploadBytesTotalString(bytesTotal),
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
	if upload.Location != server.URL+generatedTusCreationWithUploadPath {
		t.Fatalf("expected upload URL %s, got %s", server.URL+generatedTusCreationWithUploadPath, upload.Location)
	}
	if upload.RemoteOffset != int64(len(generatedTusCreationWithUploadContent)) {
		t.Fatalf("expected upload offset %d, got %d", len(generatedTusCreationWithUploadContent), upload.RemoteOffset)
	}
	if requestCount != generatedTusCreationWithUploadExpectedRequests {
		t.Fatalf("expected %d creation-with-upload request(s), got %d", generatedTusCreationWithUploadExpectedRequests, requestCount)
	}
	select {
	case err := <-requestErrs:
		t.Fatal(err)
	default:
	}
	generatedTusAssertEvents(t, "creationWithUpload", generatedTusCreationWithUploadEventPolicy, generatedTusCreationWithUploadExtraEventPrefixes, generatedTusCreationWithUploadExpectedEvents, events)

	storedUploads, err := storage.FindUploadsByFingerprint("contract-creation-with-upload-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 1 {
		t.Fatalf("expected creation-with-upload URL to be stored once, got %#v", storedUploads)
	}
}

type generatedTusCreationWithUploadSource struct {
	*strings.Reader
	events *[]string
}

func (source *generatedTusCreationWithUploadSource) Close() error {
	*source.events = append(*source.events, generatedTusEventKeySourceClose())
	return nil
}

func generatedTusCreationWithUploadBytesTotalString(bytesTotal *int64) string {
	if bytesTotal == nil {
		return "null"
	}

	return fmt.Sprintf("%d", *bytesTotal)
}

func generatedAssertTusCreationWithUploadRequestHeaders(
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

func generatedWriteTusCreationWithUploadResponseHeaders(
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
	if value := values[generatedTusCreationWithUploadOffsetHeader]; value != "" {
		responseWriter.Header().Set(generatedTusCreationWithUploadOffsetHeader, value)
	}
}
