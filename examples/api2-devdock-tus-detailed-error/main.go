//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

type detailedErrorTransport struct {
	requestCount   int
	requestMethods []string
	requestPlan    map[string]interface{}
	requestURLs    []string
}

func (transport *detailedErrorTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	transport.requestCount += 1
	transport.requestMethods = append(transport.requestMethods, request.Method)
	transport.requestURLs = append(transport.requestURLs, request.URL.String())

	errorPlan, ok, err := optionalObject(
		transport.requestPlan,
		"error",
		"conformanceScenario.requests[0].error",
	)
	if err != nil {
		return nil, err
	}
	if ok {
		message, err := api2devdock.StringValue(
			errorPlan["message"],
			"conformanceScenario.requests[0].error.message",
		)
		if err != nil {
			return nil, err
		}

		return nil, errors.New(message)
	}
	if rawErrorMessage, ok := transport.requestPlan["errorMessage"]; ok && rawErrorMessage != nil {
		message, err := api2devdock.StringValue(
			rawErrorMessage,
			"conformanceScenario.requests[0].errorMessage",
		)
		if err != nil {
			return nil, err
		}

		return nil, errors.New(message)
	}

	responsePlan, ok, err := optionalObject(
		transport.requestPlan,
		"response",
		"conformanceScenario.requests[0].response",
	)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("detailed error scenario did not provide a response or error plan")
	}

	headers, err := api2devdock.StringMapValue(
		responsePlan["effectiveHeaders"],
		"conformanceScenario.requests[0].response.effectiveHeaders",
	)
	if err != nil {
		return nil, err
	}
	statusCode, err := api2devdock.IntValue(
		responsePlan["statusCode"],
		"conformanceScenario.requests[0].response.statusCode",
	)
	if err != nil {
		return nil, err
	}
	body := ""
	if rawBody, ok := responsePlan["body"]; ok && rawBody != nil {
		body, err = api2devdock.StringValue(
			rawBody,
			"conformanceScenario.requests[0].response.body",
		)
		if err != nil {
			return nil, err
		}
	}

	responseHeaders := http.Header{}
	for name, value := range headers {
		responseHeaders.Set(name, value)
	}

	return &http.Response{
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     responseHeaders,
		Request:    request,
		StatusCode: statusCode,
	}, nil
}

func optionalObject(
	object map[string]interface{},
	key string,
	label string,
) (map[string]interface{}, bool, error) {
	rawValue, ok := object[key]
	if !ok || rawValue == nil {
		return nil, false, nil
	}
	value, err := api2devdock.ObjectValue(rawValue, label)
	if err != nil {
		return nil, false, err
	}

	return value, true, nil
}

func firstDetailedErrorRequestPlan(
	conformanceScenario map[string]interface{},
) (map[string]interface{}, error) {
	requests, err := api2devdock.ArrayValue(
		conformanceScenario["requests"],
		"conformanceScenario.requests",
	)
	if err != nil {
		return nil, err
	}
	if len(requests) != 1 {
		return nil, fmt.Errorf("detailed error scenario expected one request, got %d", len(requests))
	}

	return api2devdock.ObjectValue(requests[0], "conformanceScenario.requests[0]")
}

func detailedErrorRequestIDHeaderName(
	conformanceScenario map[string]interface{},
	requestPlan map[string]interface{},
) (string, error) {
	inputHeaders, err := api2devdock.TusConformanceInputStringMapOption(
		conformanceScenario,
		"headers",
	)
	if err != nil {
		return "", err
	}
	expectedHeaders, err := api2devdock.StringMapValue(
		requestPlan["effectiveHeaders"],
		"conformanceScenario.requests[0].effectiveHeaders",
	)
	if err != nil {
		return "", err
	}

	matchingHeaderNames := []string{}
	for name, value := range inputHeaders {
		if expectedHeaders[name] == value {
			matchingHeaderNames = append(matchingHeaderNames, name)
		}
	}
	if len(matchingHeaderNames) != 1 {
		return "", fmt.Errorf(
			"detailed error scenario expected one request ID header candidate, got %d",
			len(matchingHeaderNames),
		)
	}

	return matchingHeaderNames[0], nil
}

func uploadExpectingDetailedError(
	ctx context.Context,
	conformanceScenario map[string]interface{},
) (map[string]interface{}, error) {
	endpointURLValue, err := api2devdock.TusConformanceInputStringOption(
		conformanceScenario,
		"endpointUrl",
	)
	if err != nil {
		return nil, err
	}
	endpointURL, err := url.Parse(endpointURLValue)
	if err != nil {
		return nil, err
	}
	content, err := api2devdock.TusConformanceInputSourceBytes(conformanceScenario)
	if err != nil {
		return nil, err
	}
	metadata, err := api2devdock.TusConformanceInputStringMapOption(
		conformanceScenario,
		"metadata",
	)
	if err != nil {
		return nil, err
	}
	headers, err := api2devdock.TusConformanceInputStringMapOption(conformanceScenario, "headers")
	if err != nil {
		return nil, err
	}
	requestPlan, err := firstDetailedErrorRequestPlan(conformanceScenario)
	if err != nil {
		return nil, err
	}
	requestIDHeaderName, err := detailedErrorRequestIDHeaderName(conformanceScenario, requestPlan)
	if err != nil {
		return nil, err
	}

	transport := &detailedErrorTransport{
		requestPlan: requestPlan,
	}
	client := tusgo.NewClient(&http.Client{Transport: transport}, endpointURL).WithContext(ctx)
	client.Capabilities = &tusgo.ServerCapabilities{
		Extensions:       []string{"creation"},
		ProtocolVersions: []string{tusgo.DefaultProtocolVersion},
	}

	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:     ctx,
		Fingerprint: "api2-go-detailed-error-conformance-fingerprint",
		Headers:     headers,
		Metadata:    metadata,
		RetryDelays: []time.Duration{},
		Size:        int64(len(content)),
		Source:      bytes.NewReader(content),
		Storage:     tusgo.NewMemoryURLStorage(),
	})
	if err == nil {
		return nil, fmt.Errorf("detailed error scenario unexpectedly created upload %#v", upload)
	}
	var detailedError *tusgo.DetailedError
	errorIsDetailed := errors.As(err, &detailedError)
	result := map[string]interface{}{
		"errorCaught":     true,
		"errorMessage":    err.Error(),
		"errorIsDetailed": errorIsDetailed,
		"requestCount":    transport.requestCount,
		"requestMethods":  transport.requestMethods,
		"requestUrls":     transport.requestURLs,
	}
	if !errorIsDetailed {
		return result, nil
	}

	result["causingErrorPresent"] = detailedError.CausingError != nil
	if detailedError.CausingError != nil {
		result["causingErrorMessage"] = detailedError.CausingError.Error()
	}
	if detailedError.OriginalRequest != nil {
		result["originalRequestMethod"] = detailedError.OriginalRequest.Method
		result["originalRequestRequestId"] = detailedError.OriginalRequest.Header.Get(
			requestIDHeaderName,
		)
		result["originalRequestUrl"] = detailedError.OriginalRequest.URL.String()
	}
	result["originalResponsePresent"] = detailedError.OriginalResponse != nil
	if detailedError.OriginalResponse != nil {
		result["originalResponseBody"] = detailedError.OriginalResponseBody
		result["originalResponseStatus"] = detailedError.OriginalResponse.StatusCode
	}

	return result, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-detailed-error/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadExpectingDetailedError(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("detailed error: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s reported a detailed error\n", scenarioID)
}
