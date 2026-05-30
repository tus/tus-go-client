// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

const (
	generatedTusRequestLifecycleUploadLength = "11"
	generatedTusRequestLifecycleUploadOffset = "11"
	generatedTusRequestLifecycleUploadPath   = "/uploads/request-hooks-contract"
)

var generatedTusRequestLifecycleExpectedHookEvents = []string{"before-request:0", "after-response:0"}

func TestGeneratedRequestLifecycleHooks(t *testing.T) {
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

	getOperation := generatedProtocolOperation("getTusUploadOffset")
	client := NewClient(http.DefaultClient, baseURL)
	createdUploadURL := srvMock.URL() + generatedTusRequestLifecycleUploadPath

	getResponse := generatedResponseFor(getOperation, 200)
	getReply := generatedRequestLifecycleResponseHeaders(
		reply.Status(getResponse.StatusCode),
		getResponse,
		map[string]string{
			"Upload-Length": generatedTusRequestLifecycleUploadLength,
			"Upload-Offset": generatedTusRequestLifecycleUploadOffset,
		},
	)
	srvMock.AddMocks(
		generatedRequestLifecycleRequestHeaders(
			mocha.Request().
				URL(expect.URLPath(generatedTusRequestLifecycleUploadPath)).
				Method(getOperation.Method),
			getOperation,
			map[string]string{},
		).Reply(getReply),
	)

	events := []string{}
	beforeRequestIndex := 0
	afterResponseIndex := 0
	client = client.WithRequestLifecycleHooks(RequestLifecycleHooks{
		BeforeRequest: func(request *http.Request) error {
			if request.Method != getOperation.Method {
				return fmt.Errorf("expected %s request, got %s", getOperation.Method, request.Method)
			}
			if request.URL.Path != generatedTusRequestLifecycleUploadPath {
				return fmt.Errorf(
					"expected request path %s, got %s",
					generatedTusRequestLifecycleUploadPath,
					request.URL.Path,
				)
			}
			if err := generatedAssertRequestLifecycleRequestHeaders(
				request,
				getOperation,
				map[string]string{},
			); err != nil {
				return err
			}
			events = append(events, fmt.Sprintf("before-request:%d", beforeRequestIndex))
			beforeRequestIndex += 1
			return nil
		},
		AfterResponse: func(request *http.Request, response *http.Response) error {
			if request.Method != getOperation.Method {
				return fmt.Errorf("expected %s request, got %s", getOperation.Method, request.Method)
			}
			if response.StatusCode != getResponse.StatusCode {
				return fmt.Errorf("expected response status %d, got %d", getResponse.StatusCode, response.StatusCode)
			}
			if err := generatedAssertRequestLifecycleResponseHeaders(
				response,
				getResponse,
				map[string]string{
					"Upload-Length": generatedTusRequestLifecycleUploadLength,
					"Upload-Offset": generatedTusRequestLifecycleUploadOffset,
				},
			); err != nil {
				return err
			}
			events = append(events, fmt.Sprintf("after-response:%d", afterResponseIndex))
			afterResponseIndex += 1
			return nil
		},
	})

	upload := Upload{}
	response, err := client.GetUpload(&upload, createdUploadURL)
	if err != nil {
		t.Fatal(err)
	}
	if response == nil || response.StatusCode != getResponse.StatusCode {
		t.Fatalf("expected response status %d, got %#v", getResponse.StatusCode, response)
	}
	if upload.Location != createdUploadURL {
		t.Fatalf("expected upload URL %s, got %s", createdUploadURL, upload.Location)
	}
	if upload.RemoteOffset != 11 {
		t.Fatalf("expected upload offset %s, got %d", generatedTusRequestLifecycleUploadOffset, upload.RemoteOffset)
	}
	if upload.RemoteSize != 11 {
		t.Fatalf("expected upload length %s, got %d", generatedTusRequestLifecycleUploadLength, upload.RemoteSize)
	}
	if !reflect.DeepEqual(events, generatedTusRequestLifecycleExpectedHookEvents) {
		t.Fatalf(
			"expected request lifecycle events %#v, got %#v",
			generatedTusRequestLifecycleExpectedHookEvents,
			events,
		)
	}
}

func generatedRequestLifecycleRequestHeaders(
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

func generatedRequestLifecycleResponseHeaders(
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

func generatedAssertRequestLifecycleRequestHeaders(
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

func generatedAssertRequestLifecycleResponseHeaders(
	response *http.Response,
	contract generatedTusResponseContract,
	values map[string]string,
) error {
	variant := contract.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		expected := values[field.DisplayName]
		if expected == "" {
			expected = DefaultProtocolVersion
		}
		if actual := response.Header.Get(field.DisplayName); actual != expected {
			return fmt.Errorf(
				"expected response header %s=%s, got %s",
				field.DisplayName,
				expected,
				actual,
			)
		}
	}

	return nil
}
