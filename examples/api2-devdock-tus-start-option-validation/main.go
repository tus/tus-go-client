//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func startOptionValidationCompletion(
	conformanceScenario map[string]interface{},
) (string, string, error) {
	completion, err := api2devdock.ObjectValue(
		conformanceScenario["completion"],
		"conformanceScenario.completion",
	)
	if err != nil {
		return "", "", err
	}
	reason, err := api2devdock.StringValue(
		completion["reason"],
		"conformanceScenario.completion.reason",
	)
	if err != nil {
		return "", "", err
	}
	message, err := api2devdock.StringValue(
		completion["message"],
		"conformanceScenario.completion.message",
	)
	if err != nil {
		return "", "", err
	}

	return reason, message, nil
}

func localEndpointURLForStartValidation(
	conformanceScenario map[string]interface{},
	serverURL string,
) (*url.URL, error) {
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
	localURL, err := url.Parse(serverURL)
	if err != nil {
		return nil, err
	}
	localURL.Path = endpointURL.Path
	localURL.RawQuery = endpointURL.RawQuery

	return localURL, nil
}

func validateStartOptions(
	ctx context.Context,
	conformanceScenario map[string]interface{},
) (map[string]interface{}, error) {
	reason, expectedMessage, err := startOptionValidationCompletion(conformanceScenario)
	if err != nil {
		return nil, err
	}
	content, err := api2devdock.TusConformanceInputSourceBytes(conformanceScenario)
	if err != nil {
		return nil, err
	}
	parallelUploads, err := api2devdock.TusConformanceInputIntOption(
		conformanceScenario,
		"parallelUploads",
	)
	if err != nil {
		return nil, err
	}
	uploadDataDuringCreation, err := api2devdock.TusConformanceInputBoolOption(
		conformanceScenario,
		"uploadDataDuringCreation",
		false,
	)
	if err != nil {
		return nil, err
	}
	uploadLengthDeferred, err := api2devdock.TusConformanceInputBoolOption(
		conformanceScenario,
		"uploadLengthDeferred",
		false,
	)
	if err != nil {
		return nil, err
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		requestCount += 1
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	localEndpointURL, err := localEndpointURLForStartValidation(conformanceScenario, server.URL)
	if err != nil {
		return nil, err
	}
	client := tusgo.NewClient(http.DefaultClient, localEndpointURL).WithContext(ctx)
	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:                  ctx,
		Fingerprint:              "api2-go-start-option-validation-" + reason,
		ParallelUploads:          parallelUploads,
		Size:                     int64(len(content)),
		Source:                   bytes.NewReader(content),
		Storage:                  tusgo.NewMemoryURLStorage(),
		UploadDataDuringCreation: uploadDataDuringCreation,
		UploadLengthDeferred:     uploadLengthDeferred,
	})
	if err == nil {
		return nil, fmt.Errorf("start option validation unexpectedly created upload %#v", upload)
	}
	if err.Error() != expectedMessage {
		return nil, fmt.Errorf("expected start validation error %q, got %q", expectedMessage, err.Error())
	}
	if upload != nil {
		return nil, fmt.Errorf("start option validation returned upload %#v", upload)
	}

	return map[string]interface{}{
		"errorCaught":  true,
		"errorMessage": err.Error(),
		"requestCount": requestCount,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-start-option-validation/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := validateStartOptions(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("start option validation: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s rejected conflicting start options\n", scenarioID)
}
