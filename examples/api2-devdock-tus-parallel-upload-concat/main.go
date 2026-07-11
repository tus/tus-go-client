//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func uploadWithParallelConcat(
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
	endpointOrigin, err := url.Parse(endpointURLValue)
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
	metadataForPartialUploads, err := api2devdock.TusConformanceInputStringMapOption(
		conformanceScenario,
		"metadataForPartialUploads",
	)
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
	content, err := api2devdock.TusConformanceInputSourceBytes(conformanceScenario)
	if err != nil {
		return nil, err
	}
	capabilityPlan, err := api2devdock.TusConformanceServerCapabilities(conformanceScenario)
	if err != nil {
		return nil, err
	}
	completion, err := api2devdock.ObjectValue(
		conformanceScenario["completion"],
		"conformanceScenario.completion",
	)
	if err != nil {
		return nil, err
	}
	completionKind, err := api2devdock.StringValue(
		completion["kind"],
		"conformanceScenario.completion.kind",
	)
	if err != nil {
		return nil, err
	}

	conformanceServer, err := api2devdock.NewTusConformancePlanServer(
		conformanceScenario,
		endpointOrigin,
	)
	if err != nil {
		return nil, err
	}
	defer conformanceServer.Close()
	localEndpointURL, err := conformanceServer.EndpointURL()
	if err != nil {
		return nil, err
	}

	events := []map[string]interface{}{}
	successCalled := false
	client := tusgo.NewClient(http.DefaultClient, localEndpointURL).WithContext(ctx)
	client.Capabilities = &tusgo.ServerCapabilities{
		Extensions:       capabilityPlan.ExtensionNames,
		ProtocolVersions: capabilityPlan.ProtocolVersions,
	}
	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:                   ctx,
		Fingerprint:               "api2-go-parallel-concat-conformance-fingerprint",
		Metadata:                  metadata,
		MetadataForPartialUploads: metadataForPartialUploads,
		ParallelUploads:           parallelUploads,
		Size:                      int64(len(content)),
		Source:                    bytes.NewReader(content),
		Storage:                   tusgo.NewMemoryURLStorage(),
		EventHooks: tusgo.UploadEventHooks{
			OnProgress: func(bytesSent int64, bytesTotal *int64) error {
				event := map[string]interface{}{
					"bytesSent": bytesSent,
					"kind":      "progress",
				}
				if bytesTotal != nil {
					event["bytesTotal"] = *bytesTotal
				}
				events = append(events, event)

				return nil
			},
			OnChunkComplete: func(
				chunkSize int64,
				bytesAccepted int64,
				bytesTotal *int64,
			) error {
				event := map[string]interface{}{
					"bytesAccepted": bytesAccepted,
					"chunkSize":     chunkSize,
					"kind":          "chunk-complete",
				}
				if bytesTotal != nil {
					event["bytesTotal"] = *bytesTotal
				}
				events = append(events, event)

				return nil
			},
			OnSuccess: func(tusgo.UploadSuccessPayload) error {
				successCalled = true

				return nil
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if upload == nil || upload.Location == "" {
		return nil, fmt.Errorf("parallel TUS upload did not expose an upload URL")
	}
	if err := conformanceServer.AssertExhausted(); err != nil {
		return nil, err
	}

	result, err := conformanceServer.Result()
	if err != nil {
		return nil, err
	}
	canonicalUploadURL, err := conformanceServer.CanonicalURL(upload.Location)
	if err != nil {
		return nil, err
	}
	result["completionKind"] = completionKind
	result["errorCalled"] = false
	result["eventCount"] = len(events)
	result["events"] = events
	result["successCalled"] = successCalled
	result["uploadUrl"] = canonicalUploadURL

	return result, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-parallel-upload-concat/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadWithParallelConcat(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("parallel upload concat: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s concatenated %s\n", scenarioID, result["uploadUrl"])
}
