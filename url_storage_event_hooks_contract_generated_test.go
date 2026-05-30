// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

const (
	generatedTusEventHooksContent             = "hello world"
	generatedTusEventHooksCreatedUploadPath   = "/uploads/generated-contract"
	generatedTusEventHooksFingerprint         = "contract-single-fingerprint"
	generatedTusEventHooksPatchAcceptedOffset = "11"
	generatedTusEventHooksPatchBody           = "hello world"
	generatedTusEventHooksPatchOffset         = "0"
	generatedTusEventHooksUploadLength        = "11"
)

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

	baseURL, err := url.Parse(srvMock.URL() + "/uploads")
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

	createResponse := generatedResponseFor(createOperation, http.StatusCreated)
	createReply := generatedURLStorageEventHooksResponseHeaders(
		reply.Status(createResponse.StatusCode),
		createResponse,
		map[string]string{
			"Location": createdUploadURL,
		},
	)
	srvMock.AddMocks(
		generatedURLStorageEventHooksRequestHeaders(
			mocha.Request().
				URL(expect.URLPath("/uploads")).
				Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Upload-Metadata": encodedMetadata,
				"Upload-Length":   generatedTusEventHooksUploadLength,
			},
		).Reply(createReply),
	)

	patchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
	patchReply := generatedURLStorageEventHooksResponseHeaders(
		reply.Status(patchResponse.StatusCode),
		patchResponse,
		map[string]string{
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
				events = append(events, "upload-url-available")
				return nil
			},
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				events = append(
					events,
					fmt.Sprintf("progress:%d:%s", bytesSent, generatedTusEventHooksTotal(bytesTotal)),
				)
				return nil
			},
			OnChunkComplete: func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error {
				events = append(
					events,
					fmt.Sprintf(
						"chunk-complete:%d:%d:%s",
						chunkSize,
						bytesAccepted,
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
				events = append(events, "success")
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
	if !reflect.DeepEqual(events, generatedTusEventHooksExpectedEvents) {
		t.Fatalf("expected event hooks %#v, got %#v", generatedTusEventHooksExpectedEvents, events)
	}
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
	*source.events = append(*source.events, "source-close")
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
		value := values[field.DisplayName]
		if value == "" {
			value = DefaultProtocolVersion
		}
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
		value := values[field.DisplayName]
		if value == "" {
			value = DefaultProtocolVersion
		}
		response = response.Header(field.DisplayName, value)
	}

	return response
}
