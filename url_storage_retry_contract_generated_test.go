// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
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
	generatedTusRetryFlowContent                  = "hello world"
	generatedTusRetryFlowEventPolicy              = "exact"
	generatedTusRetryFlowFinalPatchAcceptedOffset = "11"
	generatedTusRetryFlowFinalPatchBody           = " world"
	generatedTusRetryFlowFinalPatchOffset         = "5"
	generatedTusRetryFlowFingerprint              = "retryPatchAfterOffsetRecovery-fingerprint"
	generatedTusRetryFlowFirstPatchBody           = "hello world"
	generatedTusRetryFlowFirstPatchOffset         = "0"
	generatedTusRetryFlowFirstRecoveredLength     = "11"
	generatedTusRetryFlowFirstRecoveredOffset     = "5"
	generatedTusRetryFlowSecondPatchBody          = " world"
	generatedTusRetryFlowSecondPatchOffset        = "5"
	generatedTusRetryFlowSecondRecoveredLength    = "11"
	generatedTusRetryFlowSecondRecoveredOffset    = "5"
	generatedTusRetryFlowUploadLength             = "11"
	generatedTusRetryFlowUploadPath               = "/uploads/retry-contract"
)

type generatedTusRetryDecision struct {
	Decision     bool
	RetryAttempt int
}

var generatedTusRetryFlowExtraEventPrefixes = []string{}
var generatedTusRetryFlowExpectedEvents = []string{"should-retry:0:true", "retry-schedule:0", "should-retry:0:true", "retry-schedule:0"}
var generatedTusRetryFlowMetadata = map[string]string{"filename": "hello.txt"}
var generatedTusRetryFlowRetryDelays = []time.Duration{0 * time.Millisecond}
var generatedTusRetryFlowShouldRetryEvents = []generatedTusRetryDecision{
	{
		Decision:     true,
		RetryAttempt: 0,
	},
	{
		Decision:     true,
		RetryAttempt: 0,
	},
}

func TestGeneratedURLStorageRetryOffsetRecoveryFlow(t *testing.T) {
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
		Extensions:       []string{generatedProtocolOperation("createTusUpload").Role},
		ProtocolVersions: []string{DefaultProtocolVersion},
	}

	createOperation := generatedProtocolOperation("createTusUpload")
	patchOperation := generatedProtocolOperation("patchTusUpload")
	getOperation := generatedProtocolOperation("getTusUploadOffset")
	createdUploadURL := srvMock.URL() + generatedTusRetryFlowUploadPath
	encodedMetadata, err := EncodeMetadata(generatedTusRetryFlowMetadata)
	if err != nil {
		t.Fatal(err)
	}

	createResponse := generatedResponseFor(createOperation, http.StatusCreated)
	createReply := generatedURLStorageRetryResponseHeaders(
		reply.Status(createResponse.StatusCode),
		createResponse,
		map[string]string{
			"Location":      createdUploadURL,
			"Tus-Resumable": "1.0.0",
		},
	)
	srvMock.AddMocks(
		generatedURLStorageRetryRequestHeaders(
			mocha.Request().
				URL(expect.URLPath("/uploads")).
				Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Tus-Resumable":   "1.0.0",
				"Upload-Length":   generatedTusRetryFlowUploadLength,
				"Upload-Metadata": encodedMetadata,
			},
		).Repeat(1).Reply(createReply),
	)

	firstGetResponse := generatedResponseFor(getOperation, http.StatusOK)
	firstGetReply := generatedURLStorageRetryResponseHeaders(
		reply.Status(200),
		firstGetResponse,
		map[string]string{
			"Tus-Resumable": "1.0.0",
			"Upload-Length": generatedTusRetryFlowFirstRecoveredLength,
			"Upload-Offset": generatedTusRetryFlowFirstRecoveredOffset,
		},
	)
	secondGetResponse := generatedResponseFor(getOperation, http.StatusOK)
	secondGetReply := generatedURLStorageRetryResponseHeaders(
		reply.Status(200),
		secondGetResponse,
		map[string]string{
			"Tus-Resumable": "1.0.0",
			"Upload-Length": generatedTusRetryFlowSecondRecoveredLength,
			"Upload-Offset": generatedTusRetryFlowSecondRecoveredOffset,
		},
	)
	finalPatchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
	finalPatchReply := generatedURLStorageRetryResponseHeaders(
		reply.Status(204),
		finalPatchResponse,
		map[string]string{
			"Tus-Resumable": "1.0.0",
			"Upload-Offset": generatedTusRetryFlowFinalPatchAcceptedOffset,
		},
	)
	patchReplies := []struct {
		Body   string
		Offset string
		Reply  *reply.StdReply
	}{
		{
			Body:   generatedTusRetryFlowFirstPatchBody,
			Offset: generatedTusRetryFlowFirstPatchOffset,
			Reply:  reply.Status(500),
		},
		{
			Body:   generatedTusRetryFlowSecondPatchBody,
			Offset: generatedTusRetryFlowSecondPatchOffset,
			Reply:  reply.Status(500),
		},
		{
			Body:   generatedTusRetryFlowFinalPatchBody,
			Offset: generatedTusRetryFlowFinalPatchOffset,
			Reply:  finalPatchReply,
		},
	}
	patchReplyIndex := 0
	srvMock.AddMocks(
		generatedURLStorageRetryDynamicRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusRetryFlowUploadPath)).
				Method(patchOperation.Method),
			patchOperation,
			map[string]string{
				"Content-Type":  patchOperation.Request.ContentType,
				"Tus-Resumable": "1.0.0",
			},
			map[string]func() string{
				"Upload-Offset": func() string {
					if patchReplyIndex >= len(patchReplies) {
						return ""
					}
					return patchReplies[patchReplyIndex].Offset
				},
			},
		).Repeat(len(patchReplies)).ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
			if patchReplyIndex >= len(patchReplies) {
				t.Fatalf("unexpected retry PATCH request %d", patchReplyIndex)
				return nil, nil
			}
			expected := patchReplies[patchReplyIndex]
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			if string(body) != expected.Body {
				t.Fatalf("expected PATCH body %q, got %q", expected.Body, string(body))
			}
			patchReplyIndex += 1
			return expected.Reply.Build(r, m, p)
		}),
	)

	getReplies := []*reply.StdReply{firstGetReply, secondGetReply}
	getReplyIndex := 0
	srvMock.AddMocks(
		generatedURLStorageRetryRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusRetryFlowUploadPath)).
				Method(getOperation.Method),
			getOperation,
			map[string]string{},
		).Repeat(len(getReplies)).ReplyFunction(func(r *http.Request, m reply.M, p params.P) (*reply.Response, error) {
			if getReplyIndex >= len(getReplies) {
				t.Fatalf("unexpected retry HEAD request %d", getReplyIndex)
				return nil, nil
			}
			expected := getReplies[getReplyIndex]
			getReplyIndex += 1
			return expected.Build(r, m, p)
		}),
	)

	storage := NewMemoryURLStorage()
	retryDecisionIndex := 0
	events := []string{}
	upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
		Storage:     storage,
		Source:      strings.NewReader(generatedTusRetryFlowContent),
		Fingerprint: generatedTusRetryFlowFingerprint,
		Size:        11,
		Metadata:    generatedTusRetryFlowMetadata,
		RetryDelays: generatedTusRetryFlowRetryDelays,
		OnShouldRetry: func(err error, retryAttempt int) bool {
			if retryDecisionIndex >= len(generatedTusRetryFlowShouldRetryEvents) {
				t.Fatalf("unexpected retry decision request %d for %v", retryDecisionIndex, err)
			}
			expected := generatedTusRetryFlowShouldRetryEvents[retryDecisionIndex]
			if retryAttempt != expected.RetryAttempt {
				t.Fatalf("expected retry attempt %d, got %d", expected.RetryAttempt, retryAttempt)
			}
			events = append(events, generatedTusEventKeyShouldRetry(
				generatedTusEventKeyNumber(int64(retryAttempt)),
				generatedTusEventKeyBool(expected.Decision),
			))
			if expected.Decision {
				events = append(events, generatedTusEventKeyRetrySchedule(
					generatedTusEventKeyNumber(generatedTusRetryFlowRetryDelays[retryAttempt].Milliseconds()),
				))
			}
			retryDecisionIndex += 1
			return expected.Decision
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if retryDecisionIndex != len(generatedTusRetryFlowShouldRetryEvents) {
		t.Fatalf("expected %d retry decisions, got %d", len(generatedTusRetryFlowShouldRetryEvents), retryDecisionIndex)
	}
	if patchReplyIndex != len(patchReplies) {
		t.Fatalf("expected %d PATCH requests, got %d", len(patchReplies), patchReplyIndex)
	}
	if getReplyIndex != len(getReplies) {
		t.Fatalf("expected %d HEAD requests, got %d", len(getReplies), getReplyIndex)
	}
	if upload.Location != createdUploadURL {
		t.Fatalf("expected upload URL %s, got %s", createdUploadURL, upload.Location)
	}
	if upload.RemoteOffset != 11 {
		t.Fatalf("expected upload offset 11, got %d", upload.RemoteOffset)
	}
	generatedTusAssertEvents(t, "retryPatchAfterOffsetRecovery", generatedTusRetryFlowEventPolicy, generatedTusRetryFlowExtraEventPrefixes, generatedTusRetryFlowExpectedEvents, events)
}

func generatedURLStorageRetryRequestHeaders(
	builder *mocha.MockBuilder,
	operation generatedTusProtocolOperation,
	values map[string]string,
) *mocha.MockBuilder {
	return generatedURLStorageRetryDynamicRequestHeaders(builder, operation, values, nil)
}

func generatedURLStorageRetryDynamicRequestHeaders(
	builder *mocha.MockBuilder,
	operation generatedTusProtocolOperation,
	values map[string]string,
	dynamicValues map[string]func() string,
) *mocha.MockBuilder {
	variant := operation.Request.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		if dynamicValue, ok := dynamicValues[field.DisplayName]; ok {
			builder = builder.Header(field.DisplayName, expect.Func(func(value any, args expect.Args) (bool, error) {
				stringValue, ok := value.(string)
				if !ok {
					return false, nil
				}
				return stringValue == dynamicValue(), nil
			}))
			continue
		}
		value := generatedTusRequestHeaderValue(values, field.DisplayName)
		builder = builder.Header(field.DisplayName, expect.ToEqual(value))
	}

	return builder
}

func generatedURLStorageRetryResponseHeaders(
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
