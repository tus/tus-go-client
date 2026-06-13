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
	generatedTusEventHooksContent             = "hello world"
	generatedTusEventHooksCreatedUploadPath   = "/uploads/generated-contract"
	generatedTusEventHooksEndpointPath        = "/uploads"
	generatedTusEventHooksEventPolicy         = "exact-except-allowed-extra-events"
	generatedTusEventHooksFingerprint         = "contract-single-fingerprint"
	generatedTusEventHooksPatchAcceptedOffset = "11"
	generatedTusEventHooksPatchBody           = "hello world"
	generatedTusEventHooksPatchOffset         = "0"
	generatedTusEventHooksUploadLength        = "11"
)

var generatedTusEventHooksExtraEventPrefixes = []string{"progress:"}
var generatedTusEventHooksExpectedEvents = []string{"upload-url-available", "progress:0:11", "progress:11:11", "chunk-complete:11:11:11", "success", "source-close"}
var generatedTusEventHooksMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedURLStorageEventHooks(t *testing.T) {
	srvMock := mocha.New(t)
	srvMock.Start()
	defer func() {
		if err := srvMock.Close(); err != nil {
			t.Fatal(err)
		}
		srvMock.AssertCalled(t)
	}()

	baseURL, err := url.Parse(srvMock.URL() + generatedTusEventHooksEndpointPath)
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
	createdUploadURL := srvMock.URL() + generatedTusEventHooksCreatedUploadPath
	encodedMetadata, err := EncodeMetadata(generatedTusEventHooksMetadata)
	if err != nil {
		t.Fatal(err)
	}

	createResponse := generatedResponseFor(createOperation, 201)
	createReply := generatedURLStorageEventHooksResponseHeaders(
		reply.Status(createResponse.StatusCode),
		createResponse,
		map[string]string{
			"Location":      createdUploadURL,
			"Tus-Resumable": "1.0.0",
		},
	)
	srvMock.AddMocks(
		generatedURLStorageEventHooksRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusEventHooksEndpointPath)).
				Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Tus-Resumable":   "1.0.0",
				"Upload-Length":   generatedTusEventHooksUploadLength,
				"Upload-Metadata": encodedMetadata,
			},
		).Reply(createReply),
	)

	patchResponse := generatedResponseFor(patchOperation, 204)
	patchReply := generatedURLStorageEventHooksResponseHeaders(
		reply.Status(patchResponse.StatusCode),
		patchResponse,
		map[string]string{
			"Tus-Resumable": "1.0.0",
			"Upload-Offset": generatedTusEventHooksPatchAcceptedOffset,
		},
	)
	patchRequest := generatedURLStorageEventHooksRequestHeaders(
		mocha.Request().
			URL(expect.URLPath(generatedTusEventHooksCreatedUploadPath)).
			Method(patchOperation.Method).
			Body(expect.ToEqual([]byte(generatedTusEventHooksPatchBody))),
		patchOperation,
		map[string]string{
			"Content-Type":  patchOperation.Request.ContentType,
			"Tus-Resumable": "1.0.0",
			"Upload-Offset": generatedTusEventHooksPatchOffset,
		},
	)
	srvMock.AddMocks(patchRequest.Reply(patchReply))

	events := []string{}
	source := &generatedTusEventHooksSource{
		Reader: strings.NewReader(generatedTusEventHooksContent),
		events: &events,
	}
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:     storage,
		Source:      source,
		Fingerprint: generatedTusEventHooksFingerprint,
		Size:        11,
		Metadata:    generatedTusEventHooksMetadata,
		EventHooks: UploadEventHooks{
			OnUploadURLAvailable: func() error {
				events = append(events, generatedTusEventKeyUploadUrlAvailable())
				return nil
			},
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				events = append(
					events,
					generatedTusEventKeyProgress(
						generatedTusEventKeyNumber(bytesSent),
						generatedTusEventHooksTotal(bytesTotal),
					),
				)
				return nil
			},
			OnChunkComplete: func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error {
				events = append(
					events,
					generatedTusEventKeyChunkComplete(
						generatedTusEventKeyNumber(chunkSize),
						generatedTusEventKeyNumber(bytesAccepted),
						generatedTusEventHooksTotal(bytesTotal),
					),
				)
				return nil
			},
			OnSuccess: func(payload UploadSuccessPayload) error {
				if payload.Upload == nil || payload.Upload.Location != createdUploadURL {
					return fmt.Errorf("expected success upload URL %s, got %#v", createdUploadURL, payload.Upload)
				}
				if payload.LastResponse == nil || payload.LastResponse.StatusCode != patchResponse.StatusCode {
					return fmt.Errorf(
						"expected success response status %d, got %#v",
						patchResponse.StatusCode,
						payload.LastResponse,
					)
				}
				events = append(events, generatedTusEventKeySuccess())
				return nil
			},
		},
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
	generatedTusAssertEvents(t, "singleUploadLifecycle", generatedTusEventHooksEventPolicy, generatedTusEventHooksExtraEventPrefixes, generatedTusEventHooksExpectedEvents, events)
}

func generatedTusEventHooksTotal(bytesTotal *int64) string {
	if bytesTotal == nil {
		return "null"
	}

	return fmt.Sprintf("%d", *bytesTotal)
}

type generatedTusEventHooksSource struct {
	*strings.Reader
	events *[]string
}

func (source *generatedTusEventHooksSource) Close() error {
	*source.events = append(*source.events, generatedTusEventKeySourceClose())
	return nil
}

func generatedURLStorageEventHooksRequestHeaders(
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

func generatedURLStorageEventHooksResponseHeaders(
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
