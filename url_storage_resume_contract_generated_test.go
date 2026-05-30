// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

const (
	generatedTusResumeFlowContent             = "hello world"
	generatedTusResumeFlowFingerprint         = "contract-resume-fingerprint"
	generatedTusResumeFlowPatchAcceptedOffset = "11"
	generatedTusResumeFlowPatchBody           = " world"
	generatedTusResumeFlowPatchOffset         = "5"
	generatedTusResumeFlowStoredUploadPath    = "/uploads/resume-contract"
	generatedTusResumeFlowUploadLength        = "11"
)

var generatedTusResumeFlowMetadata = map[string]string{}

func TestGeneratedURLStorageResumeFlow(t *testing.T) {
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
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	storage := NewMemoryURLStorage()
	storedUploadURL := srvMock.URL() + generatedTusResumeFlowStoredUploadPath
	if _, err := storage.AddUpload(
		generatedTusResumeFlowFingerprint,
		URLStorageUpload{
			"metadata":  stringMapToAnyMap(generatedTusResumeFlowMetadata),
			"size":      11,
			"uploadUrl": storedUploadURL,
		},
	); err != nil {
		t.Fatal(err)
	}

	getOperation := generatedProtocolOperation("getTusUploadOffset")
	getResponse := generatedResponseFor(getOperation, http.StatusOK)
	getReply := generatedURLStorageResumeResponseHeaders(
		reply.Status(getResponse.StatusCode),
		getResponse,
		map[string]string{
			"Upload-Length": generatedTusResumeFlowUploadLength,
			"Upload-Offset": generatedTusResumeFlowPatchOffset,
		},
	)
	srvMock.AddMocks(
		generatedURLStorageResumeRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusResumeFlowStoredUploadPath)).
				Method(getOperation.Method),
			getOperation,
			map[string]string{},
		).Reply(getReply),
	)

	patchOperation := generatedProtocolOperation("patchTusUpload")
	patchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
	patchReply := generatedURLStorageResumeResponseHeaders(
		reply.Status(patchResponse.StatusCode),
		patchResponse,
		map[string]string{
			"Upload-Offset": generatedTusResumeFlowPatchAcceptedOffset,
		},
	)
	patchRequest := generatedURLStorageResumeRequestHeaders(
		mocha.Request().
			URL(expect.URLPath(generatedTusResumeFlowStoredUploadPath)).
			Method(patchOperation.Method).
			Body(expect.ToEqual([]byte(generatedTusResumeFlowPatchBody))),
		patchOperation,
		map[string]string{
			"Content-Type":  patchOperation.Request.ContentType,
			"Upload-Offset": generatedTusResumeFlowPatchOffset,
		},
	)
	srvMock.AddMocks(patchRequest.Reply(patchReply))

	successCalled := false
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:                    storage,
		Source:                     strings.NewReader(generatedTusResumeFlowContent),
		Fingerprint:                generatedTusResumeFlowFingerprint,
		Size:                       11,
		Metadata:                   generatedTusResumeFlowMetadata,
		RemoveFingerprintOnSuccess: true,
		EventHooks: UploadEventHooks{
			OnSuccess: func(UploadSuccessPayload) error {
				remainingUploads, err := storage.FindUploadsByFingerprint(generatedTusResumeFlowFingerprint)
				if err != nil {
					return err
				}
				if len(remainingUploads) != 0 {
					return fmt.Errorf(
						"expected success hook to run after storage removal, got %#v",
						remainingUploads,
					)
				}
				successCalled = true
				return nil
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !successCalled {
		t.Fatal("expected success hook to be called")
	}
	if upload.Location != storedUploadURL {
		t.Fatalf("expected resumed upload URL %s, got %s", storedUploadURL, upload.Location)
	}
	if upload.RemoteOffset != 11 {
		t.Fatalf("expected upload offset 11, got %d", upload.RemoteOffset)
	}

	remainingUploads, err := storage.FindUploadsByFingerprint(generatedTusResumeFlowFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingUploads) != 0 {
		t.Fatalf("expected successful resume to remove stored upload, got %#v", remainingUploads)
	}
}

func generatedURLStorageResumeRequestHeaders(
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

func generatedURLStorageResumeResponseHeaders(
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
