//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func observedCustomHeaders(
	request *http.Request,
	expectedHeaders map[string]string,
) map[string]string {
	observed := map[string]string{}
	for name := range expectedHeaders {
		observed[name] = request.Header.Get(name)
	}

	return observed
}

func assertObservedCustomHeaders(
	label string,
	actual map[string]string,
	expected map[string]string,
) error {
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("%s expected headers %v, got %v", label, expected, actual)
	}

	return nil
}

func uploadWithCustomHeaders(
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
					headersByMethod[request.Method] = observedCustomHeaders(request, headers)
				}

				return nil
			},
		},
	).WithContext(ctx)

	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:     ctx,
		Fingerprint: scenarioID + "-fingerprint",
		Headers:     headers,
		Metadata:    metadata,
		RetryDelays: retryDelays,
		Size:        int64(len(content)),
		Source:      bytes.NewReader(content),
		Storage:     tusgo.NewMemoryURLStorage(),
	})
	if err != nil {
		return nil, err
	}
	if upload == nil || upload.Location == "" {
		return nil, fmt.Errorf("custom request headers TUS upload did not expose an upload URL")
	}
	if upload.RemoteOffset != int64(len(content)) {
		return nil, fmt.Errorf(
			"custom request headers upload accepted %d bytes, expected %d",
			upload.RemoteOffset,
			len(content),
		)
	}
	if err := assertObservedCustomHeaders("POST", headersByMethod[http.MethodPost], headers); err != nil {
		return nil, err
	}
	if err := assertObservedCustomHeaders("PATCH", headersByMethod[http.MethodPatch], headers); err != nil {
		return nil, err
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
		"examples/api2-devdock-tus-custom-request-headers/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadWithCustomHeaders(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("custom request headers: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s sent custom headers to %s\n", scenarioID, result["uploadUrl"])
}
