// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/params"
	"github.com/vitorsalgado/mocha/v3/reply"
)

const (
	generatedTusTerminateFlowContent             = "hello world"
	generatedTusTerminateFlowEventPolicy         = "exact"
	generatedTusTerminateFlowPatchAcceptedOffset = "5"
	generatedTusTerminateFlowPatchBody           = "hello"
	generatedTusTerminateFlowPatchOffset         = "0"
	generatedTusTerminateFlowUploadLength        = "11"
	generatedTusTerminateFlowUploadPath          = "/uploads/terminate-contract"
)

type generatedTusTerminateRetryDecision struct {
	Decision     bool
	RetryAttempt int
}

var generatedTusTerminateFlowExpectedEvents = []string{"should-retry:0:true", "retry-schedule:0"}
var generatedTusTerminateFlowMetadata = map[string]string{"filename": "hello.txt"}
var generatedTusTerminateFlowRetryDelays = []time.Duration{0 * time.Millisecond, 0 * time.Millisecond}
var generatedTusTerminateFlowShouldRetryEvents = []generatedTusTerminateRetryDecision{
	{
		Decision:     true,
		RetryAttempt: 0,
	},
}

func TestGeneratedTerminationRetryFlow(t *testing.T) {
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
	terminateOperation := generatedProtocolOperation("terminateTusUpload")
	client := NewClient(http.DefaultClient, baseURL)
	client.Capabilities = &ServerCapabilities{
		Extensions:       []string{createOperation.Role, terminateOperation.Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	createdUploadURL := srvMock.URL() + generatedTusTerminateFlowUploadPath
	encodedMetadata, err := EncodeMetadata(generatedTusTerminateFlowMetadata)
	if err != nil {
		t.Fatal(err)
	}
	createResponse := generatedResponseFor(createOperation, http.StatusCreated)
	createReply := generatedTerminationRetryResponseHeaders(
		reply.Status(createResponse.StatusCode),
		createResponse,
		map[string]string{
			"Location": createdUploadURL,
		},
	)
	srvMock.AddMocks(
		generatedTerminationRetryRequestHeaders(
			mocha.Request().
				URL(expect.URLPath("/uploads")).
				Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Upload-Metadata": encodedMetadata,
				"Upload-Length":   generatedTusTerminateFlowUploadLength,
			},
		).Reply(createReply),
	)

	patchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
	patchReply := generatedTerminationRetryResponseHeaders(
		reply.Status(patchResponse.StatusCode),
		patchResponse,
		map[string]string{
			"Upload-Offset": generatedTusTerminateFlowPatchAcceptedOffset,
		},
	)
	srvMock.AddMocks(
		generatedTerminationRetryRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusTerminateFlowUploadPath)).
				Method(patchOperation.Method).
				Body(expect.ToEqual([]byte(generatedTusTerminateFlowPatchBody))),
			patchOperation,
			map[string]string{
				"Content-Type":  patchOperation.Request.ContentType,
				"Upload-Offset": generatedTusTerminateFlowPatchOffset,
			},
		).Reply(patchReply),
	)

	finalTerminateResponse := generatedResponseFor(terminateOperation, http.StatusNoContent)
	finalTerminateReply := generatedTerminationRetryResponseHeaders(
		reply.Status(204),
		finalTerminateResponse,
		map[string]string{},
	)
	terminateReplies := []*reply.StdReply{
		reply.Status(423),
		finalTerminateReply,
	}
	terminateReplyIndex := 0
	srvMock.AddMocks(
		generatedTerminationRetryRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusTerminateFlowUploadPath)).
				Method(terminateOperation.Method),
			terminateOperation,
			map[string]string{},
		).Repeat(len(terminateReplies)).ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
			if terminateReplyIndex >= len(terminateReplies) {
				t.Fatalf("unexpected termination request %d", terminateReplyIndex)
				return nil, nil
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			if len(body) != 0 {
				t.Fatalf("expected empty termination body, got %q", string(body))
			}
			expected := terminateReplies[terminateReplyIndex]
			terminateReplyIndex += 1
			return expected.Build(r, m, p)
		}),
	)

	upload := &Upload{}
	if _, err := client.CreateUpload(upload, 11, false, generatedTusTerminateFlowMetadata); err != nil {
		t.Fatal(err)
	}
	stream := NewUploadStream(client, upload)
	stream.ChunkSize = 5
	if _, err := stream.ReadFrom(strings.NewReader(generatedTusTerminateFlowPatchBody)); err != nil {
		t.Fatal(err)
	}
	if upload.RemoteOffset != 5 {
		t.Fatalf("expected uploaded offset 5, got %d", upload.RemoteOffset)
	}

	events := []string{}
	retryDecisionIndex := 0
	response, err := client.TerminateUploadWithRetry(*upload, TerminateUploadOptions{
		RetryDelays: generatedTusTerminateFlowRetryDelays,
		OnShouldRetry: func(err error, retryAttempt int) bool {
			if retryDecisionIndex >= len(generatedTusTerminateFlowShouldRetryEvents) {
				t.Fatalf("unexpected termination retry decision request %d for %v", retryDecisionIndex, err)
			}
			expected := generatedTusTerminateFlowShouldRetryEvents[retryDecisionIndex]
			if retryAttempt != expected.RetryAttempt {
				t.Fatalf("expected termination retry attempt %d, got %d", expected.RetryAttempt, retryAttempt)
			}
			events = append(events, fmt.Sprintf("should-retry:%d:%t", retryAttempt, expected.Decision))
			if expected.Decision {
				events = append(events, fmt.Sprintf("retry-schedule:%d", generatedTusTerminateFlowRetryDelays[retryAttempt].Milliseconds()))
			}
			retryDecisionIndex += 1
			return expected.Decision
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response == nil || response.StatusCode != 204 {
		t.Fatalf("expected termination status 204, got %#v", response)
	}
	if terminateReplyIndex != len(terminateReplies) {
		t.Fatalf("expected %d termination requests, got %d", len(terminateReplies), terminateReplyIndex)
	}
	if retryDecisionIndex != len(generatedTusTerminateFlowShouldRetryEvents) {
		t.Fatalf("expected %d termination retry decisions, got %d", len(generatedTusTerminateFlowShouldRetryEvents), retryDecisionIndex)
	}
	generatedTusAssertEvents(t, "terminateWithRetry", generatedTusTerminateFlowEventPolicy, generatedTusTerminateFlowExpectedEvents, events)
}

func generatedTerminationRetryRequestHeaders(
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

func generatedTerminationRetryResponseHeaders(
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
