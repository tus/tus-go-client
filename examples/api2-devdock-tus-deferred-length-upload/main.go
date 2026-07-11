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

func uploadWithDeferredLength(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (string, error) {
	uploadLengthDeferred, err := api2devdock.UploadLengthDeferred(scenario)
	if err != nil {
		return "", err
	}
	if !uploadLengthDeferred {
		return "", fmt.Errorf("deferred-length scenario must set uploadLengthDeferred")
	}
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return "", err
	}
	content, err := api2devdock.ScenarioBytes(scenario)
	if err != nil {
		return "", err
	}
	chunkSize, err := api2devdock.FixedChunkSizeBytes(scenario)
	if err != nil {
		return "", err
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return "", err
	}
	retryDelays, err := api2devdock.RetryDelays(scenario)
	if err != nil {
		return "", err
	}
	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		return "", err
	}

	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithContext(ctx)
	createdUpload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		ChunkSize:            chunkSize,
		Context:              ctx,
		Fingerprint:          scenarioID + "-fingerprint",
		Metadata:             metadata,
		RetryDelays:          retryDelays,
		Size:                 int64(len(content)),
		Source:               bytes.NewReader(content),
		Storage:              tusgo.NewMemoryURLStorage(),
		UploadLengthDeferred: uploadLengthDeferred,
	})
	if err != nil {
		return "", err
	}
	if createdUpload == nil || createdUpload.Location == "" {
		return "", fmt.Errorf("deferred-length TUS upload did not expose an upload URL")
	}
	if createdUpload.RemoteSize != int64(len(content)) {
		return "", fmt.Errorf(
			"deferred-length upload size is %d, expected %d",
			createdUpload.RemoteSize,
			len(content),
		)
	}
	if createdUpload.RemoteOffset != int64(len(content)) {
		return "", fmt.Errorf(
			"deferred-length upload accepted %d bytes, expected %d",
			createdUpload.RemoteOffset,
			len(content),
		)
	}

	return createdUpload.Location, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-deferred-length-upload/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	uploadURL, err := uploadWithDeferredLength(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("deferred-length upload: %v", err)
	}
	if err := api2devdock.WriteResult(map[string]interface{}{"uploadUrl": uploadURL}); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s deferred length to %s\n", scenarioID, uploadURL)
}
