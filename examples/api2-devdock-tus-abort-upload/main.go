//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

type observedAbortRequest struct {
	Method string
	URL    string
}

type abortConformanceServer struct {
	cancelUpload   context.CancelFunc
	endpointOrigin *url.URL
	errs           []error
	mu             sync.Mutex
	observed       []observedAbortRequest
	requests       []interface{}
	server         *httptest.Server
	events         []map[string]interface{}
}

func newAbortConformanceServer(
	conformanceScenario map[string]interface{},
	endpointOrigin *url.URL,
	cancelUpload context.CancelFunc,
) (*abortConformanceServer, error) {
	requests, err := api2devdock.ArrayValue(
		conformanceScenario["requests"],
		"conformanceScenario.requests",
	)
	if err != nil {
		return nil, err
	}

	conformanceServer := &abortConformanceServer{
		cancelUpload:   cancelUpload,
		endpointOrigin: endpointOrigin,
		requests:       requests,
	}
	conformanceServer.server = httptest.NewServer(conformanceServer)

	return conformanceServer, nil
}

func (conformanceServer *abortConformanceServer) Close() {
	conformanceServer.server.Close()
}

func (conformanceServer *abortConformanceServer) EndpointURL() (*url.URL, error) {
	endpointURL := *conformanceServer.endpointOrigin
	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return nil, err
	}
	endpointURL.Scheme = serverURL.Scheme
	endpointURL.Host = serverURL.Host

	return &endpointURL, nil
}

func (conformanceServer *abortConformanceServer) CanonicalURL(actualURL string) (string, error) {
	parsedActual, err := url.Parse(actualURL)
	if err != nil {
		return "", err
	}
	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return "", err
	}
	if parsedActual.Scheme != serverURL.Scheme || parsedActual.Host != serverURL.Host {
		return actualURL, nil
	}

	canonical := *parsedActual
	canonical.Scheme = conformanceServer.endpointOrigin.Scheme
	canonical.Host = conformanceServer.endpointOrigin.Host

	return canonical.String(), nil
}

func (conformanceServer *abortConformanceServer) Result() (map[string]interface{}, error) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	if len(conformanceServer.errs) > 0 {
		return nil, conformanceServer.errs[0]
	}

	requestMethods := make([]string, 0, len(conformanceServer.observed))
	requestURLs := make([]string, 0, len(conformanceServer.observed))
	for _, request := range conformanceServer.observed {
		requestMethods = append(requestMethods, request.Method)
		requestURLs = append(requestURLs, request.URL)
	}

	return map[string]interface{}{
		"events":         conformanceServer.events,
		"requestCount":   len(conformanceServer.observed),
		"requestMethods": requestMethods,
		"requestUrls":    requestURLs,
	}, nil
}

func (conformanceServer *abortConformanceServer) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	requestIndex, requestPlan, err := conformanceServer.nextRequestPlan()
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	expectedURL, err := api2devdock.StringValue(
		requestPlan["expectedUrl"],
		fmt.Sprintf("conformanceScenario.requests[%d].expectedUrl", requestIndex),
	)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	expectedMethod, err := api2devdock.StringValue(
		requestPlan["effectiveMethod"],
		fmt.Sprintf("conformanceScenario.requests[%d].effectiveMethod", requestIndex),
	)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	requestURL := conformanceServer.endpointOrigin.ResolveReference(request.URL).String()
	conformanceServer.observeRequest(observedAbortRequest{
		Method: request.Method,
		URL:    requestURL,
	})
	if requestURL != expectedURL {
		conformanceServer.recordErr(
			fmt.Errorf("request %d expected URL %s, got %s", requestIndex, expectedURL, requestURL),
		)
	}
	if request.Method != expectedMethod {
		conformanceServer.recordErr(
			fmt.Errorf("request %d expected method %s, got %s", requestIndex, expectedMethod, request.Method),
		)
	}

	shouldAbort, err := api2devdock.BoolValue(
		requestPlan["abort"],
		fmt.Sprintf("conformanceScenario.requests[%d].abort", requestIndex),
	)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if shouldAbort {
		conformanceServer.recordAbortEvent(requestIndex, request.Method, requestURL)
		conformanceServer.cancelUpload()
		select {
		case <-request.Context().Done():
		case <-time.After(2 * time.Second):
			conformanceServer.recordErr(
				fmt.Errorf("request %d did not observe cancellation", requestIndex),
			)
		}
		return
	}

	if err := conformanceServer.writeResponse(responseWriter, requestIndex, requestPlan); err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}
}

func (conformanceServer *abortConformanceServer) nextRequestPlan() (
	int,
	map[string]interface{},
	error,
) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	requestIndex := len(conformanceServer.observed)
	if requestIndex >= len(conformanceServer.requests) {
		return 0, nil, fmt.Errorf("unexpected request %d", requestIndex)
	}
	requestPlan, err := api2devdock.ObjectValue(
		conformanceServer.requests[requestIndex],
		fmt.Sprintf("conformanceScenario.requests[%d]", requestIndex),
	)
	if err != nil {
		return 0, nil, err
	}

	return requestIndex, requestPlan, nil
}

func (conformanceServer *abortConformanceServer) observeRequest(request observedAbortRequest) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	conformanceServer.observed = append(conformanceServer.observed, request)
}

func (conformanceServer *abortConformanceServer) recordAbortEvent(
	requestIndex int,
	method string,
	requestURL string,
) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	conformanceServer.events = append(conformanceServer.events, map[string]interface{}{
		"kind":         "request-abort",
		"method":       method,
		"requestIndex": requestIndex,
		"url":          requestURL,
	})
}

func (conformanceServer *abortConformanceServer) recordErr(err error) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	conformanceServer.errs = append(conformanceServer.errs, err)
}

func (conformanceServer *abortConformanceServer) writeResponse(
	responseWriter http.ResponseWriter,
	requestIndex int,
	requestPlan map[string]interface{},
) error {
	responsePlan, err := api2devdock.ObjectValue(
		requestPlan["response"],
		fmt.Sprintf("conformanceScenario.requests[%d].response", requestIndex),
	)
	if err != nil {
		return err
	}
	headers, err := api2devdock.StringMapValue(
		responsePlan["effectiveHeaders"],
		fmt.Sprintf("conformanceScenario.requests[%d].response.effectiveHeaders", requestIndex),
	)
	if err != nil {
		return err
	}
	for name, value := range headers {
		if name == "Location" {
			value, err = conformanceServer.localResponseURL(value)
			if err != nil {
				return err
			}
		}
		responseWriter.Header().Set(name, value)
	}
	statusCode, err := api2devdock.IntValue(
		responsePlan["statusCode"],
		fmt.Sprintf("conformanceScenario.requests[%d].response.statusCode", requestIndex),
	)
	if err != nil {
		return err
	}
	responseWriter.WriteHeader(statusCode)

	return nil
}

func (conformanceServer *abortConformanceServer) localResponseURL(value string) (string, error) {
	canonicalURL, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return "", err
	}
	if canonicalURL.Scheme != conformanceServer.endpointOrigin.Scheme ||
		canonicalURL.Host != conformanceServer.endpointOrigin.Host {
		return value, nil
	}

	localURL := *canonicalURL
	localURL.Scheme = serverURL.Scheme
	localURL.Host = serverURL.Host

	return localURL.String(), nil
}

func uploadAndAbort(
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
	content, err := api2devdock.TusConformanceInputSourceBytes(conformanceScenario)
	if err != nil {
		return nil, err
	}
	metadata, err := api2devdock.TusConformanceInputStringMapOption(conformanceScenario, "metadata")
	if err != nil {
		return nil, err
	}
	headers, err := api2devdock.TusConformanceInputStringMapOption(conformanceScenario, "headers")
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
	terminateUploadOnAbort, err := api2devdock.TusConformanceRuntimeAbortTerminateUpload(
		conformanceScenario,
	)
	if err != nil {
		return nil, err
	}
	fingerprint, err := api2devdock.TusConformanceRuntimeFingerprint(conformanceScenario)
	if err != nil {
		return nil, err
	}
	if fingerprint == "" {
		fingerprint = "api2-go-abort-conformance-fingerprint"
	}
	capabilityPlan, err := api2devdock.TusConformanceServerCapabilities(conformanceScenario)
	if err != nil {
		return nil, err
	}
	capabilities := &tusgo.ServerCapabilities{
		Extensions:       capabilityPlan.ExtensionNames,
		ProtocolVersions: capabilityPlan.ProtocolVersions,
	}

	uploadCtx, cancelUpload := context.WithCancel(ctx)
	defer cancelUpload()
	conformanceServer, err := newAbortConformanceServer(
		conformanceScenario,
		endpointOrigin,
		cancelUpload,
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
	client.Capabilities = capabilities

	type uploadResult struct {
		err    error
		upload *tusgo.Upload
	}
	result := make(chan uploadResult, 1)
	go func() {
		upload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
			Context:                uploadCtx,
			Fingerprint:            fingerprint,
			Headers:                headers,
			Metadata:               metadata,
			OverridePatchMethod:    overridePatchMethod,
			Size:                   int64(len(content)),
			Source:                 bytes.NewReader(content),
			Storage:                tusgo.NewMemoryURLStorage(),
			TerminateUploadOnAbort: terminateUploadOnAbort,
			EventHooks: tusgo.UploadEventHooks{
				OnSuccess: func(tusgo.UploadSuccessPayload) error {
					successCalled = true
					return nil
				},
			},
		})
		result <- uploadResult{err: err, upload: upload}
	}()

	var upload *tusgo.Upload
	select {
	case uploadResult := <-result:
		if !errors.Is(uploadResult.err, context.Canceled) {
			return nil, fmt.Errorf("expected upload abort, got upload=%#v err=%v", uploadResult.upload, uploadResult.err)
		}
		upload = uploadResult.upload
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timed out waiting for upload abort")
	}

	serverResult, err := conformanceServer.Result()
	if err != nil {
		return nil, err
	}
	uploadURL := interface{}(nil)
	if upload != nil && upload.Location != "" {
		canonicalUploadURL, err := conformanceServer.CanonicalURL(upload.Location)
		if err != nil {
			return nil, err
		}
		uploadURL = canonicalUploadURL
	}
	serverResult["completionKind"] = "aborted"
	serverResult["errorCalled"] = false
	serverResult["successCalled"] = successCalled
	serverResult["uploadUrl"] = uploadURL

	return serverResult, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-abort-upload/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	conformanceScenario, err := api2devdock.TusConformanceScenario(scenario)
	if err != nil {
		api2devdock.Fail("read conformance scenario: %v", err)
	}

	result, err := uploadAndAbort(ctx, conformanceScenario)
	if err != nil {
		api2devdock.Fail("abort upload: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s aborted the upload\n", scenarioID)
}
