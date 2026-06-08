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

func uploadWithOverridePatchMethod(
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
	uploadURLValue, err := api2devdock.TusConformanceInputStringOption(
		conformanceScenario,
		"uploadUrl",
	)
	if err != nil {
		return nil, err
	}
	overridePatchMethod, err := api2devdock.TusConformanceInputBoolOption(
		conformanceScenario,
		"overridePatchMethod",
		false,
	)
	if err != nil {
		return nil, err
	}
	content, err := api2devdock.TusConformanceInputSourceBytes(conformanceScenario)
	if err != nil {
		return nil, err
	}
	fingerprint, err := api2devdock.TusConformanceRuntimeFingerprint(conformanceScenario)
	if err != nil {
		return nil, err
	}
	if fingerprint == "" {
		fingerprint = "api2-go-override-patch-conformance-fingerprint"
	}
	capabilityPlan, err := api2devdock.TusConformanceServerCapabilities(conformanceScenario)
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
	localUploadURL, err := conformanceServer.LocalURL(uploadURLValue)
	if err != nil {
		return nil, err
	}

	storage := tusgo.NewMemoryURLStorage()
	if _, err := storage.AddUpload(
		fingerprint,
		tusgo.URLStorageUpload{"uploadUrl": localUploadURL},
	); err != nil {
		return nil, err
	}

	client := tusgo.NewClient(http.DefaultClient, localEndpointURL).WithContext(ctx)
	client.Capabilities = &tusgo.ServerCapabilities{
		Extensions:       capabilityPlan.ExtensionNames,
		ProtocolVersions: capabilityPlan.ProtocolVersions,
	}
	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:             ctx,
		Fingerprint:         fingerprint,
		OverridePatchMethod: overridePatchMethod,
		Size:                int64(len(content)),
		Source:              bytes.NewReader(content),
		Storage:             storage,
	})
	if err != nil {
		return nil, err
	}
	if upload == nil || upload.Location == "" {
		return nil, fmt.Errorf("override-PATCH TUS upload did not expose an upload URL")
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
	result["uploadUrl"] = canonicalUploadURL

	return result, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-override-patch-method/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadWithOverridePatchMethod(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("override PATCH method: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s overrode PATCH for %s\n", scenarioID, result["uploadUrl"])
}
