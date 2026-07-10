package tusgo

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

const (
	generatedTusCreateFlowContent                    = "hello world"
	generatedTusCreateFlowCreatedUploadPath          = "/uploads/generated-contract"
	generatedTusCreateFlowEndpointPath               = "/uploads"
	generatedTusCreateFlowFingerprint                = "contract-single-fingerprint"
	generatedTusCreateFlowPatchAcceptedOffset        = "11"
	generatedTusCreateFlowPatchBody                  = "hello world"
	generatedTusCreateFlowPatchOffset                = "0"
	generatedTusCreateFlowRemoveFingerprintOnSuccess = false
	generatedTusCreateFlowUploadLength               = "11"
)

var generatedTusCreateFlowMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedURLStorageCreateFlow(t *testing.T) {
	srvMock := mocha.New(t)
	srvMock.Start()
	defer func() {
		if err := srvMock.Close(); err != nil {
			t.Fatal(err)
		}
		srvMock.AssertCalled(t)
	}()

	baseURL, err := url.Parse(srvMock.URL() + generatedTusCreateFlowEndpointPath)
	if err != nil {
		t.Fatal(err)
	}

	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	createdUploadURL := srvMock.URL() + generatedTusCreateFlowCreatedUploadPath
	encodedMetadata, err := EncodeMetadata(generatedTusCreateFlowMetadata)
	if err != nil {
		t.Fatal(err)
	}

	createResponse := generatedResponseFor(createOperation, 201)
	createReply := generatedURLStorageCreateResponseHeaders(
		reply.Status(createResponse.StatusCode),
		createResponse,
		map[string]string{
			"Location":      createdUploadURL,
			"Tus-Resumable": "1.0.0",
		},
	)
	srvMock.AddMocks(
		generatedURLStorageCreateRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusCreateFlowEndpointPath)).
				Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Tus-Resumable":   "1.0.0",
				"Upload-Length":   generatedTusCreateFlowUploadLength,
				"Upload-Metadata": encodedMetadata,
			},
		).Reply(createReply),
	)

	patchResponse := generatedResponseFor(patchOperation, 204)
	patchReply := generatedURLStorageCreateResponseHeaders(
		reply.Status(patchResponse.StatusCode),
		patchResponse,
		map[string]string{
			"Tus-Resumable": "1.0.0",
			"Upload-Offset": generatedTusCreateFlowPatchAcceptedOffset,
		},
	)
	patchRequest := generatedURLStorageCreateRequestHeaders(
		mocha.Request().
			URL(expect.URLPath(generatedTusCreateFlowCreatedUploadPath)).
			Method(patchOperation.Method).
			Body(expect.ToEqual([]byte(generatedTusCreateFlowPatchBody))),
		patchOperation,
		map[string]string{
			"Content-Type":  patchOperation.Request.ContentType,
			"Tus-Resumable": "1.0.0",
			"Upload-Offset": generatedTusCreateFlowPatchOffset,
		},
	)
	srvMock.AddMocks(patchRequest.Reply(patchReply))

	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:                    storage,
		Source:                     strings.NewReader(generatedTusCreateFlowContent),
		Fingerprint:                generatedTusCreateFlowFingerprint,
		Size:                       11,
		Metadata:                   generatedTusCreateFlowMetadata,
		RemoveFingerprintOnSuccess: generatedTusCreateFlowRemoveFingerprintOnSuccess,
	})
	if err != nil {
		t.Fatal(err)
	}
	if upload.Location != createdUploadURL {
		t.Fatalf("expected created upload URL %s, got %s", createdUploadURL, upload.Location)
	}
	if upload.RemoteOffset != 11 {
		t.Fatalf("expected upload offset 11, got %d", upload.RemoteOffset)
	}

	storedUploads, err := storage.FindUploadsByFingerprint(generatedTusCreateFlowFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	shouldRemoveStoredUpload, shouldRemoveStoredUploadErr := generatedTusShouldRemoveStoredUploadOnSuccess(
		generatedTusCreateFlowRemoveFingerprintOnSuccess,
	)
	if shouldRemoveStoredUploadErr != nil {
		t.Fatal(shouldRemoveStoredUploadErr)
	}
	if shouldRemoveStoredUpload {
		if len(storedUploads) != 0 {
			t.Fatalf("expected successful create flow to remove stored upload, got %#v", storedUploads)
		}
		return
	}
	if len(storedUploads) != 1 {
		t.Fatalf("expected successful create flow to store one upload, got %#v", storedUploads)
	}
	storedUploadURL, ok := stringFromURLStorageUpload(storedUploads[0], "uploadUrl")
	if !ok || storedUploadURL != createdUploadURL {
		t.Fatalf("expected stored upload URL %s, got %#v", createdUploadURL, storedUploads[0])
	}
}

func generatedURLStorageCreateRequestHeaders(
	builder *mocha.MockBuilder,
	operation generatedTusProtocolOperation,
	values map[string]string,
) *mocha.MockBuilder {
	variant := operation.Request.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := generatedTusRequestHeaderValue(values, field.DisplayName)
		builder = builder.Header(field.DisplayName, expect.ToEqual(value))
	}

	return builder
}

func generatedURLStorageCreateResponseHeaders(
	response *reply.StdReply,
	contract generatedTusResponseContract,
	values map[string]string,
) *reply.StdReply {
	variant := contract.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := generatedTusResponseHeaderValue(values, field.DisplayName)
		response = response.Header(field.DisplayName, value)
	}

	return response
}
