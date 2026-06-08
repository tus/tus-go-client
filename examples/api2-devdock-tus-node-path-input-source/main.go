//go:build api2devdock

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func appendSourceOpenEvent(
	events []map[string]interface{},
	conformanceScenario map[string]interface{},
	inputKind string,
	size int,
) ([]map[string]interface{}, error) {
	wantsSourceOpen, err := api2devdock.TusConformanceScenarioWantsEvent(
		conformanceScenario,
		"source-open",
	)
	if err != nil {
		return nil, err
	}
	if !wantsSourceOpen {
		return events, nil
	}

	return append(events, map[string]interface{}{
		"inputKind": inputKind,
		"kind":      "source-open",
		"size":      size,
	}), nil
}

func appendSourceCloseEvent(
	events []map[string]interface{},
	conformanceScenario map[string]interface{},
) ([]map[string]interface{}, error) {
	wantsSourceClose, err := api2devdock.TusConformanceScenarioWantsEvent(
		conformanceScenario,
		"source-close",
	)
	if err != nil {
		return nil, err
	}
	if !wantsSourceClose {
		return events, nil
	}

	return append(events, map[string]interface{}{"kind": "source-close"}), nil
}

func appendSuccessEvent(
	events []map[string]interface{},
	conformanceScenario map[string]interface{},
) ([]map[string]interface{}, error) {
	wantsSuccess, err := api2devdock.TusConformanceScenarioWantsEvent(
		conformanceScenario,
		"success",
	)
	if err != nil {
		return nil, err
	}
	if !wantsSuccess {
		return events, nil
	}

	return append(events, map[string]interface{}{"kind": "success"}), nil
}

func uploadWithNodePathInputSource(
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
	content, err := api2devdock.TusConformanceInputSourceBytes(conformanceScenario)
	if err != nil {
		return nil, err
	}
	inputKind, err := api2devdock.TusConformanceInputSourceKind(conformanceScenario)
	if err != nil {
		return nil, err
	}
	capabilityPlan, err := api2devdock.TusConformanceServerCapabilities(conformanceScenario)
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "api2-go-tus-node-path-input-source-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(inputPath, content, 0o600); err != nil {
		return nil, err
	}
	source, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}

	events := []map[string]interface{}{}
	events, err = appendSourceOpenEvent(events, conformanceScenario, inputKind, len(content))
	if err != nil {
		source.Close()
		return nil, err
	}

	conformanceServer, err := api2devdock.NewTusConformancePlanServer(
		conformanceScenario,
		endpointOrigin,
	)
	if err != nil {
		source.Close()
		return nil, err
	}
	defer conformanceServer.Close()
	localEndpointURL, err := conformanceServer.EndpointURL()
	if err != nil {
		source.Close()
		return nil, err
	}

	client := tusgo.NewClient(http.DefaultClient, localEndpointURL).WithContext(ctx)
	client.Capabilities = &tusgo.ServerCapabilities{
		Extensions:       capabilityPlan.ExtensionNames,
		ProtocolVersions: capabilityPlan.ProtocolVersions,
	}
	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:     ctx,
		Fingerprint: "api2-go-node-path-input-source-conformance-fingerprint",
		Metadata:    metadata,
		Size:        int64(len(content)),
		Source:      source,
		Storage:     tusgo.NewMemoryURLStorage(),
		EventHooks: tusgo.UploadEventHooks{
			OnSuccess: func(tusgo.UploadSuccessPayload) error {
				var eventErr error
				events, eventErr = appendSuccessEvent(events, conformanceScenario)
				return eventErr
			},
		},
	})
	closeErr := source.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
		return nil, closeErr
	}
	events, err = appendSourceCloseEvent(events, conformanceScenario)
	if err != nil {
		return nil, err
	}
	if upload == nil || upload.Location == "" {
		return nil, fmt.Errorf("node-path TUS upload did not expose an upload URL")
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
	result["events"] = events
	result["inputKind"] = inputKind
	result["uploadUrl"] = canonicalUploadURL

	return result, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-node-path-input-source/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadWithNodePathInputSource(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("node path input source: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf(
		"Go TUS SDK devdock scenario %s read %s for %s\n",
		scenarioID,
		result["inputKind"],
		result["uploadUrl"],
	)
}
