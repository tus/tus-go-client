//go:build api2devdock

package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

type requestCountingTransport struct {
	base    http.RoundTripper
	methods []string
	mu      sync.Mutex
}

func (transport *requestCountingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	transport.methods = append(transport.methods, request.Method)
	transport.mu.Unlock()

	base := transport.base
	if base == nil {
		base = http.DefaultTransport
	}

	return base.RoundTrip(request)
}

func (transport *requestCountingTransport) Count(method string) int {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	count := 0
	for _, candidate := range transport.methods {
		if candidate == method {
			count += 1
		}
	}

	return count
}

func (transport *requestCountingTransport) Methods() []string {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	methods := make([]string, len(transport.methods))
	copy(methods, transport.methods)
	return methods
}

func verifyTerminatedUpload(
	ctx context.Context,
	httpClient *http.Client,
	termination api2devdock.TerminationPlan,
	uploadURL string,
) (int, error) {
	request, err := http.NewRequestWithContext(ctx, termination.VerificationMethod, uploadURL, nil)
	if err != nil {
		return 0, err
	}
	for name, value := range tusgo.DefaultProtocolRequestHeaders() {
		request.Header.Set(name, value)
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	return response.StatusCode, nil
}

func uploadAndTerminate(
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
	chunkSize, err := api2devdock.FixedChunkSizeBytes(scenario)
	if err != nil {
		return nil, err
	}
	termination, err := api2devdock.Termination(scenario)
	if err != nil {
		return nil, err
	}
	if termination.StopAfterAcceptedBytes > len(content) {
		return nil, fmt.Errorf(
			"stop-after bytes %d exceeds content length %d",
			termination.StopAfterAcceptedBytes,
			len(content),
		)
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return nil, err
	}

	transport := &requestCountingTransport{}
	httpClient := &http.Client{Transport: transport}
	client := tusgo.NewClient(httpClient, endpointURL).WithContext(ctx)
	upload := tusgo.Upload{}
	if _, err := client.CreateUpload(&upload, int64(len(content)), false, metadata); err != nil {
		return nil, err
	}
	if upload.Location == "" {
		return nil, fmt.Errorf("created upload did not include a Location")
	}

	stream := tusgo.NewUploadStream(client, &upload)
	stream.ChunkSize = chunkSize
	stopAfterAcceptedBytes := termination.StopAfterAcceptedBytes
	written, err := stream.Write(content[:stopAfterAcceptedBytes])
	if err != nil {
		return nil, err
	}
	if written != stopAfterAcceptedBytes {
		return nil, fmt.Errorf("wrote %d bytes, expected %d", written, stopAfterAcceptedBytes)
	}
	acceptedBytes := int(upload.RemoteOffset)
	if acceptedBytes != stopAfterAcceptedBytes {
		return nil, fmt.Errorf("accepted %d bytes, expected %d", acceptedBytes, stopAfterAcceptedBytes)
	}

	if _, err := client.TerminateUploadWithRetry(upload, tusgo.TerminateUploadOptions{}); err != nil {
		return nil, err
	}
	verificationStatus, err := verifyTerminatedUpload(ctx, httpClient, termination, upload.Location)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"acceptedBytes":      acceptedBytes,
		"deleteRequestCount": transport.Count(http.MethodDelete),
		"requestMethods":     transport.Methods(),
		"terminated":         true,
		"uploadUrl":          upload.Location,
		"verificationStatus": verificationStatus,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-terminate-upload/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadAndTerminate(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("terminate upload: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s terminated %s\n", scenarioID, result["uploadUrl"])
}
