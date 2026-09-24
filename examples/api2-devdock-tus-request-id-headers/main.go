//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func observedRequestIDHeader(request *http.Request, headerName string) (string, error) {
	value := request.Header.Get(headerName)
	if value == "" {
		return "", fmt.Errorf(
			"request ID headers scenario did not observe %s on %s",
			headerName,
			request.Method,
		)
	}

	return value, nil
}

func uploadWithRequestIDHeaders(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]interface{}, error) {
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
	addRequestID, err := api2devdock.UploadAddRequestID(scenario)
	if err != nil {
		return nil, err
	}
	requestIDHeaderName, err := api2devdock.UploadRequestIDHeaderName(scenario)
	if err != nil {
		return nil, err
	}
	headers, err := api2devdock.UploadHeaders(scenario)
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

	headersByMethod := map[string]map[string]string{}
	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithRequestLifecycleHooks(
		tusgo.RequestLifecycleHooks{
			BeforeRequest: func(request *http.Request) error {
				switch request.Method {
				case http.MethodPost, http.MethodPatch:
					value, err := observedRequestIDHeader(request, requestIDHeaderName)
					if err != nil {
						return err
					}
					headersByMethod[request.Method] = map[string]string{
						requestIDHeaderName: value,
					}
				}

				return nil
			},
		},
	).WithContext(ctx)

	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		AddRequestID: addRequestID,
		Context:      ctx,
		Fingerprint:  scenarioID + "-fingerprint",
		Headers:      headers,
		Metadata:     metadata,
		RetryDelays:  retryDelays,
		Size:         int64(len(content)),
		Source:       bytes.NewReader(content),
		Storage:      tusgo.NewMemoryURLStorage(),
	})
	if err != nil {
		return nil, err
	}
	if upload == nil || upload.Location == "" {
		return nil, fmt.Errorf("request ID headers TUS upload did not expose an upload URL")
	}
	if upload.RemoteOffset != int64(len(content)) {
		return nil, fmt.Errorf(
			"request ID headers upload accepted %d bytes, expected %d",
			upload.RemoteOffset,
			len(content),
		)
	}

	return map[string]interface{}{
		"headersByMethod": headersByMethod,
		"uploadUrl":       upload.Location,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-request-id-headers/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadWithRequestIDHeaders(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("request ID headers: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s observed request ID headers to %s\n", scenarioID, result["uploadUrl"])
}
