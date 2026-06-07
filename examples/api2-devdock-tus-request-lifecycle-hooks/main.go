//go:build api2devdock

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func assertRequestMethods(label string, actual []string, expected []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf("%s expected request methods %v, got %v", label, expected, actual)
	}

	for index, method := range expected {
		if actual[index] != method {
			return fmt.Errorf(
				"%s expected request method %s at index %d, got %s",
				label,
				method,
				index,
				actual[index],
			)
		}
	}

	return nil
}

func assertStatusCodes(actual []int, expected []int) error {
	if len(actual) != len(expected) {
		return fmt.Errorf(
			"request lifecycle hooks expected status codes %v, got %v",
			expected,
			actual,
		)
	}

	for index, statusCode := range expected {
		if actual[index] != statusCode {
			return fmt.Errorf(
				"request lifecycle hooks expected status code %d at index %d, got %d",
				statusCode,
				index,
				actual[index],
			)
		}
	}

	return nil
}

func shouldIgnoreRequestMethod(method string, ignoredMethods []string) bool {
	for _, ignoredMethod := range ignoredMethods {
		if method == ignoredMethod {
			return true
		}
	}

	return false
}

func uploadWithLifecycleHooks(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]interface{}, error) {
	requestLifecycleHooks, err := api2devdock.RequestLifecycleHooks(scenario)
	if err != nil {
		return nil, err
	}
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return nil, err
	}
	content, err := api2devdock.ScenarioBytes(scenario)
	if err != nil {
		return nil, err
	}
	if err := api2devdock.RequireFullFileChunkSize(scenario); err != nil {
		return nil, err
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return nil, err
	}

	afterResponseMethods := []string{}
	afterResponseStatusCodes := []int{}
	beforeRequestMethods := []string{}
	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithRequestLifecycleHooks(
		tusgo.RequestLifecycleHooks{
			BeforeRequest: func(request *http.Request) error {
				if shouldIgnoreRequestMethod(request.Method, requestLifecycleHooks.IgnoredRequestMethods) {
					return nil
				}
				beforeRequestMethods = append(beforeRequestMethods, request.Method)

				return nil
			},
			AfterResponse: func(request *http.Request, response *http.Response) error {
				if shouldIgnoreRequestMethod(request.Method, requestLifecycleHooks.IgnoredRequestMethods) {
					return nil
				}
				afterResponseMethods = append(afterResponseMethods, request.Method)
				afterResponseStatusCodes = append(afterResponseStatusCodes, response.StatusCode)

				return nil
			},
		},
	).WithContext(ctx)

	upload := tusgo.Upload{}
	if _, err := client.CreateUpload(&upload, int64(len(content)), false, metadata); err != nil {
		return nil, err
	}
	if upload.Location == "" {
		return nil, fmt.Errorf("request lifecycle hooks TUS upload did not expose an upload URL")
	}

	stream := tusgo.NewUploadStream(client, &upload)
	stream.ChunkSize = tusgo.NoChunked
	written, err := stream.Write(content)
	if err != nil {
		return nil, err
	}
	if written != len(content) {
		return nil, fmt.Errorf("wrote %d bytes, expected %d", written, len(content))
	}
	if upload.RemoteOffset != int64(len(content)) {
		return nil, fmt.Errorf("remote offset %d, expected %d", upload.RemoteOffset, len(content))
	}
	if err := assertRequestMethods(
		"before request lifecycle hooks",
		beforeRequestMethods,
		requestLifecycleHooks.ExpectedBeforeRequestMethods,
	); err != nil {
		return nil, err
	}
	if err := assertRequestMethods(
		"after response lifecycle hooks",
		afterResponseMethods,
		requestLifecycleHooks.ExpectedAfterResponseMethods,
	); err != nil {
		return nil, err
	}
	if err := assertStatusCodes(
		afterResponseStatusCodes,
		requestLifecycleHooks.ExpectedAfterResponseStatusCodes,
	); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"afterResponseMethods":     afterResponseMethods,
		"afterResponseStatusCodes": afterResponseStatusCodes,
		"beforeRequestMethods":     beforeRequestMethods,
		"uploadUrl":                upload.Location,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-request-lifecycle-hooks/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadWithLifecycleHooks(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("request lifecycle hooks: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf(
		"Go TUS SDK devdock scenario %s observed lifecycle hooks for %s\n",
		scenarioID,
		result["uploadUrl"],
	)
}
