// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	generatedTusAbortContent      = "hello world"
	generatedTusAbortEndpointPath = "/uploads"
	generatedTusAbortUploadLength = "11"
)

var generatedTusAbortExpectedEvents = []string{"request-abort:0"}
var generatedTusAbortMetadata = map[string]string{"filename": "hello.txt"}

func TestGeneratedAbortUploadContext(t *testing.T) {
	createOperation := generatedProtocolOperation("createTusUpload")
	encodedMetadata, err := EncodeMetadata(generatedTusAbortMetadata)
	if err != nil {
		t.Fatal(err)
	}

	requestStarted := make(chan struct{})
	requestDone := make(chan struct{})
	events := []string{}
	var requestErr error
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		defer close(requestDone)
		if request.URL.Path != generatedTusAbortEndpointPath {
			requestErr = fmt.Errorf("expected path %s, got %s", generatedTusAbortEndpointPath, request.URL.Path)
		}
		if request.Method != createOperation.Method {
			requestErr = fmt.Errorf("expected method %s, got %s", createOperation.Method, request.Method)
		}
		if err := generatedAssertTusAbortRequestHeaders(
			request,
			createOperation,
			map[string]string{
				"Upload-Metadata": encodedMetadata,
				"Upload-Length":   generatedTusAbortUploadLength,
			},
		); err != nil {
			requestErr = err
		}
		events = append(events, "request-abort:0")
		close(requestStarted)
		<-request.Context().Done()
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL + generatedTusAbortEndpointPath)
	if err != nil {
		t.Fatal(err)
	}
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	storage := NewMemoryURLStorage()
	go func() {
		_, err := client.UploadWithURLStorage(URLStorageUploadOptions{
			Context:     ctx,
			Storage:     storage,
			Source:      strings.NewReader(generatedTusAbortContent),
			Fingerprint: "contract-abort-fingerprint",
			Size:        11,
			Metadata:    generatedTusAbortMetadata,
		})
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for abort request")
	}
	if requestErr != nil {
		t.Fatal(requestErr)
	}
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for abort result")
	}
	select {
	case <-requestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server to observe abort")
	}
	if !reflect.DeepEqual(events, generatedTusAbortExpectedEvents) {
		t.Fatalf("expected abort events %#v, got %#v", generatedTusAbortExpectedEvents, events)
	}

	storedUploads, err := storage.FindAllUploads()
	if err != nil {
		t.Fatal(err)
	}
	if len(storedUploads) != 0 {
		t.Fatalf("expected aborted create not to store uploads, got %#v", storedUploads)
	}
}

func generatedAssertTusAbortRequestHeaders(
	request *http.Request,
	operation generatedTusProtocolOperation,
	values map[string]string,
) error {
	variant := operation.Request.HeaderVariants[0]
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
