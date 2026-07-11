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

func uploadWithProtocolVersionSelection(
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
	protocol, err := api2devdock.TusConformanceInputStringOption(conformanceScenario, "protocol")
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

	successCalled := false
	client := tusgo.NewClient(http.DefaultClient, localEndpointURL).WithContext(ctx)
	client.ProtocolVersion = protocol
	client.Capabilities = &tusgo.ServerCapabilities{
		Extensions:       capabilityPlan.ExtensionNames,
		ProtocolVersions: capabilityPlan.ProtocolVersions,
	}
	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:                  ctx,
		Fingerprint:              "api2-go-protocol-version-conformance-fingerprint",
		Metadata:                 metadata,
		Size:                     int64(len(content)),
		Source:                   bytes.NewReader(content),
		Storage:                  tusgo.NewMemoryURLStorage(),
		UploadDataDuringCreation: uploadDataDuringCreation,
		EventHooks: tusgo.UploadEventHooks{
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
		return nil, fmt.Errorf("protocol-version TUS upload did not expose an upload URL")
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
	result["successCalled"] = successCalled
	result["uploadUrl"] = canonicalUploadURL

	return result, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-protocol-version-selection/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadWithProtocolVersionSelection(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("protocol version selection: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s selected protocol for %s\n", scenarioID, result["uploadUrl"])
}
