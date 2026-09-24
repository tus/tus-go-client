//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func assertRequestMethods(actual []string, expected []string) error {
	if len(actual) != len(expected) {
		return fmt.Errorf(
			"retry offset recovery expected request methods %v, got %v",
			expected,
			actual,
		)
	}

	for index, method := range expected {
		if actual[index] != method {
			return fmt.Errorf(
				"retry offset recovery expected request method %s at index %d, got %s",
				method,
				index,
				actual[index],
			)
		}
	}

	return nil
}

func readOffsetHeader(response *http.Response, headerName string) (int, error) {
	value := response.Header.Get(headerName)
	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf(
			"retry offset recovery expected numeric %s response header, got %q",
			headerName,
			value,
		)
	}

	return offset, nil
}

func uploadWithRetryOffsetRecovery(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]interface{}, error) {
	retryOffsetRecovery, err := api2devdock.RetryOffsetRecovery(scenario)
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
	chunkSize, err := api2devdock.FixedChunkSizeBytes(scenario)
	if err != nil {
		return nil, err
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return nil, err
	}
	retryDelays, err := api2devdock.RetryDelays(scenario)
	if err != nil {
		return nil, err
	}
	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		return nil, err
	}

	recoveredOffsets := []int{}
	requestMethods := []string{}
	failureCandidateCount := 0
	simulatedFailureCount := 0
	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithRequestLifecycleHooks(
		tusgo.RequestLifecycleHooks{
			BeforeRequest: func(request *http.Request) error {
				if request.Method != http.MethodOptions {
					requestMethods = append(requestMethods, request.Method)
				}

				return nil
			},
			AfterResponse: func(request *http.Request, response *http.Response) error {
				if response == nil {
					return nil
				}
				if request.Method == retryOffsetRecovery.RecoveryResponse.Method {
					offset, err := readOffsetHeader(
						response,
						retryOffsetRecovery.RecoveryResponse.OffsetHeader,
					)
					if err != nil {
						return err
					}
					recoveredOffsets = append(recoveredOffsets, offset)
				}
				if request.Method != retryOffsetRecovery.FailAfterResponse.Method {
					return nil
				}

				failureCandidateCount += 1
				if failureCandidateCount != retryOffsetRecovery.FailAfterResponse.Occurrence {
					return nil
				}

				simulatedFailureCount += 1
				response.StatusCode = http.StatusInternalServerError
				response.Status = "500 " + retryOffsetRecovery.FailAfterResponse.Message
				return nil
			},
		},
	).WithContext(ctx)

	createdUpload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		ChunkSize:     chunkSize,
		Context:       ctx,
		Fingerprint:   scenarioID + "-fingerprint",
		Metadata:      metadata,
		RetryDelays:   retryDelays,
		Size:          int64(len(content)),
		Source:        bytes.NewReader(content),
		Storage:       tusgo.NewMemoryURLStorage(),
		OnShouldRetry: func(_ error, _ int) bool { return true },
	})
	if err != nil {
		return nil, err
	}
	if createdUpload == nil || createdUpload.Location == "" {
		return nil, fmt.Errorf("retry offset recovery TUS upload did not expose an upload URL")
	}
	if simulatedFailureCount != retryOffsetRecovery.ExpectedFailureCount {
		return nil, fmt.Errorf(
			"retry offset recovery expected %d simulated failure(s), got %d",
			retryOffsetRecovery.ExpectedFailureCount,
			simulatedFailureCount,
		)
	}
	if len(recoveredOffsets) != retryOffsetRecovery.ExpectedRecoveryRequestCount {
		return nil, fmt.Errorf(
			"retry offset recovery expected %d recovery request(s), got %d",
			retryOffsetRecovery.ExpectedRecoveryRequestCount,
			len(recoveredOffsets),
		)
	}
	if recoveredOffsets[0] != retryOffsetRecovery.ExpectedRecoveredOffset {
		return nil, fmt.Errorf(
			"retry offset recovery expected recovered offset %d, got %d",
			retryOffsetRecovery.ExpectedRecoveredOffset,
			recoveredOffsets[0],
		)
	}
	if err := assertRequestMethods(
		requestMethods,
		retryOffsetRecovery.ExpectedRequestMethods,
	); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"recoveredOffsets":      recoveredOffsets,
		"recoveryRequestCount":  len(recoveredOffsets),
		"requestMethods":        requestMethods,
		"simulatedFailureCount": simulatedFailureCount,
		"uploadUrl":             createdUpload.Location,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-retry-offset-recovery/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadWithRetryOffsetRecovery(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("retry offset recovery: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf(
		"Go TUS SDK devdock scenario %s recovered offset for %s\n",
		scenarioID,
		result["uploadUrl"],
	)
}
