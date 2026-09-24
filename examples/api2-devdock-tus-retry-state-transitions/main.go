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

type retryStateObserver struct {
	decisions   []api2devdock.TusConformanceRetryDecision
	err         error
	events      []map[string]interface{}
	index       int
	retryDelays []time.Duration
}

func newRetryStateObserver(
	decisions []api2devdock.TusConformanceRetryDecision,
	retryDelays []time.Duration,
) *retryStateObserver {
	return &retryStateObserver{
		decisions:   decisions,
		events:      []map[string]interface{}{},
		retryDelays: retryDelays,
	}
}

func (observer *retryStateObserver) onShouldRetry(_ error, retryAttempt int) bool {
	if observer.index >= len(observer.decisions) {
		observer.err = fmt.Errorf(
			"retry state transition received unexpected retry decision request %d",
			observer.index,
		)
		return false
	}

	decision := observer.decisions[observer.index]
	if retryAttempt != decision.RetryAttempt {
		observer.err = fmt.Errorf(
			"retry state transition expected retry attempt %d, got %d",
			decision.RetryAttempt,
			retryAttempt,
		)
		return false
	}

	observer.events = append(observer.events, map[string]interface{}{
		"decision":     decision.Decision,
		"kind":         "should-retry",
		"retryAttempt": retryAttempt,
	})
	if decision.Decision {
		if retryAttempt < 0 || retryAttempt >= len(observer.retryDelays) {
			observer.err = fmt.Errorf(
				"retry state transition retry attempt %d has no retry delay",
				retryAttempt,
			)
			return false
		}
		observer.events = append(observer.events, map[string]interface{}{
			"delay": observer.retryDelays[retryAttempt].Milliseconds(),
			"kind":  "retry-schedule",
		})
	}
	observer.index += 1

	return decision.Decision
}

func (observer *retryStateObserver) assertComplete() error {
	if observer.err != nil {
		return observer.err
	}
	if observer.index != len(observer.decisions) {
		return fmt.Errorf(
			"retry state transition expected %d retry decision(s), got %d",
			len(observer.decisions),
			observer.index,
		)
	}

	return nil
}

func tusConformanceRetryDelays(
	conformanceScenario map[string]interface{},
) ([]time.Duration, error) {
	options, err := api2devdock.TusConformanceInputOptions(conformanceScenario)
	if err != nil {
		return nil, err
	}
	rawRetryDelays, ok := options["retryDelays"]
	if !ok {
		return nil, fmt.Errorf("conformanceScenario.inputOptionEntries.retryDelays is required")
	}
	delayValues, err := api2devdock.IntArrayValue(
		rawRetryDelays,
		"conformanceScenario.inputOptionEntries.retryDelays",
	)
	if err != nil {
		return nil, err
	}

	retryDelays := make([]time.Duration, 0, len(delayValues))
	for _, delayValue := range delayValues {
		retryDelays = append(retryDelays, time.Duration(delayValue)*time.Millisecond)
	}

	return retryDelays, nil
}

func uploadWithRetryStateTransitions(
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
	retryDecisions, err := api2devdock.TusConformanceRetryDecisions(conformanceScenario)
	if err != nil {
		return nil, err
	}
	retryDelays, err := tusConformanceRetryDelays(conformanceScenario)
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

	observer := newRetryStateObserver(retryDecisions, retryDelays)
	successCalled := false
	client := tusgo.NewClient(http.DefaultClient, localEndpointURL).WithContext(ctx)
	client.Capabilities = &tusgo.ServerCapabilities{
		Extensions:       capabilityPlan.ExtensionNames,
		ProtocolVersions: capabilityPlan.ProtocolVersions,
	}
	upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context: ctx,
		EventHooks: tusgo.UploadEventHooks{OnSuccess: func(tusgo.UploadSuccessPayload) error {
			successCalled = true
			return nil
		}},
		Fingerprint:   "api2-go-retry-state-conformance-fingerprint",
		Metadata:      metadata,
		OnShouldRetry: observer.onShouldRetry,
		RetryDelays:   retryDelays,
		Size:          int64(len(content)),
		Source:        bytes.NewReader(content),
		Storage:       tusgo.NewMemoryURLStorage(),
	})
	if err != nil {
		return nil, err
	}
	if err := observer.assertComplete(); err != nil {
		return nil, err
	}
	if upload == nil || upload.Location == "" {
		return nil, fmt.Errorf("retry state transition TUS upload did not expose an upload URL")
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
	result["eventCount"] = len(observer.events)
	result["events"] = observer.events
	result["successCalled"] = successCalled
	result["uploadUrl"] = canonicalUploadURL

	return result, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-retry-state-transitions/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadWithRetryStateTransitions(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("retry state transitions: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf(
		"Go TUS SDK devdock scenario %s observed %d retry event(s) for %s\n",
		scenarioID,
		result["eventCount"],
		result["uploadUrl"],
	)
}
