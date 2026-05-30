// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

const (
	generatedTusFileFlowContent                    = "hello world"
	generatedTusFileFlowCreatedUploadPath          = "/uploads/node-path-contract"
	generatedTusFileFlowFingerprintExpected        = "node-file-/tmp/tus-contract-file.bin-11-1700000000123-https://tus.io/uploads"
	generatedTusFileFlowFingerprintFixtureEndpoint = "https://tus.io/uploads"
	generatedTusFileFlowFingerprintFixtureMtimeMs  = 1700000000123
	generatedTusFileFlowFingerprintFixturePath     = "/tmp/tus-contract-file.bin"
	generatedTusFileFlowPatchAcceptedOffset        = "11"
	generatedTusFileFlowPatchBody                  = "hello world"
	generatedTusFileFlowPatchOffset                = "0"
	generatedTusFileFlowRemoveFingerprintOnSuccess = false
	generatedTusFileFlowUploadLength               = "11"
)

var generatedTusFileFlowMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedFileFingerprint(t *testing.T) {
	fingerprint := FileFingerprint(FileFingerprintInput{
		AbsolutePath: generatedTusFileFlowFingerprintFixturePath,
		Endpoint:     generatedTusFileFlowFingerprintFixtureEndpoint,
		MtimeMs:      generatedTusFileFlowFingerprintFixtureMtimeMs,
		Size:         11,
	})
	if fingerprint != generatedTusFileFlowFingerprintExpected {
		t.Fatalf("expected file fingerprint %s, got %s", generatedTusFileFlowFingerprintExpected, fingerprint)
	}
}

func TestGeneratedURLStorageFileFlow(t *testing.T) {
	srvMock := mocha.New(t)
	srvMock.Start()
	defer func() {
		if err := srvMock.Close(); err != nil {
			t.Fatal(err)
		}
		srvMock.AssertCalled(t)
	}()

	baseURL, err := url.Parse(srvMock.URL() + "/uploads")
	if err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(t.TempDir(), "tus-contract-input.bin")
	if err := os.WriteFile(filePath, []byte(generatedTusFileFlowContent), 0o600); err != nil {
		t.Fatal(err)
	}
	mtime := time.Unix(0, generatedTusFileFlowFingerprintFixtureMtimeMs*int64(time.Millisecond))
	if err := os.Chtimes(filePath, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		t.Fatal(err)
	}
	expectedFingerprint := FileFingerprint(FileFingerprintInput{
		AbsolutePath: absolutePath,
		Endpoint:     baseURL.String(),
		MtimeMs:      generatedTusFileFlowFingerprintFixtureMtimeMs,
		Size:         11,
	})

	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:        []string{createOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	createdUploadURL := srvMock.URL() + generatedTusFileFlowCreatedUploadPath
	encodedMetadata, err := EncodeMetadata(generatedTusFileFlowMetadata)
	if err != nil {
		t.Fatal(err)
	}

	createResponse := generatedResponseFor(createOperation, http.StatusCreated)
	createReply := generatedURLStorageFileResponseHeaders(
		reply.Status(createResponse.StatusCode),
		createResponse,
		map[string]string{
			"Location": createdUploadURL,
		},
	)
	srvMock.AddMocks(
		generatedURLStorageFileRequestHeaders(
			mocha.Request().
				URL(expect.URLPath("/uploads")).
				Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Upload-Metadata": encodedMetadata,
				"Upload-Length":   generatedTusFileFlowUploadLength,
			},
		).Reply(createReply),
	)

	patchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
	patchReply := generatedURLStorageFileResponseHeaders(
		reply.Status(patchResponse.StatusCode),
		patchResponse,
		map[string]string{
			"Upload-Offset": generatedTusFileFlowPatchAcceptedOffset,
		},
	)
	patchRequest := generatedURLStorageFileRequestHeaders(
		mocha.Request().
			URL(expect.URLPath(generatedTusFileFlowCreatedUploadPath)).
			Method(patchOperation.Method).
			Body(expect.ToEqual([]byte(generatedTusFileFlowPatchBody))),
		patchOperation,
		map[string]string{
			"Content-Type":  patchOperation.Request.ContentType,
			"Upload-Offset": generatedTusFileFlowPatchOffset,
		},
	)
	srvMock.AddMocks(patchRequest.Reply(patchReply))

	upload, err := client.UploadFileWithURLStorage(URLStorageFileUploadOptions{
		Storage:                    storage,
		Path:                       filePath,
		Metadata:                   generatedTusFileFlowMetadata,
		RemoveFingerprintOnSuccess: generatedTusFileFlowRemoveFingerprintOnSuccess,
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

	storedUploads, err := storage.FindUploadsByFingerprint(expectedFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if generatedTusFileFlowRemoveFingerprintOnSuccess {
		if len(storedUploads) != 0 {
			t.Fatalf("expected successful file flow to remove stored upload, got %#v", storedUploads)
		}
		return
	}
	if len(storedUploads) != 1 {
		t.Fatalf("expected successful file flow to store one upload, got %#v", storedUploads)
	}
	storedUploadURL, ok := stringFromURLStorageUpload(storedUploads[0], "uploadUrl")
	if !ok || storedUploadURL != createdUploadURL {
		t.Fatalf("expected stored upload URL %s, got %#v", createdUploadURL, storedUploads[0])
	}
}

func generatedURLStorageFileRequestHeaders(
	builder *mocha.MockBuilder,
	operation generatedTusProtocolOperation,
	values map[string]string,
) *mocha.MockBuilder {
	variant := operation.Request.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := values[field.DisplayName]
		if value == "" {
			value = DefaultProtocolVersion
		}
		builder = builder.Header(field.DisplayName, expect.ToEqual(value))
	}

	return builder
}

func generatedURLStorageFileResponseHeaders(
	response *reply.StdReply,
	contract generatedTusResponseContract,
	values map[string]string,
) *reply.StdReply {
	variant := contract.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := values[field.DisplayName]
		if value == "" {
			value = DefaultProtocolVersion
		}
		response = response.Header(field.DisplayName, value)
	}

	return response
}
