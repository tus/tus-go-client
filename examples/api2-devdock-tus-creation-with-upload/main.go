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

func uploadWithCreationData(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (string, error) {
	upload, err := api2devdock.ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return "", err
	}
	uploadDataDuringCreation, err := api2devdock.BoolValue(
		upload["uploadDataDuringCreation"],
		"upload.uploadDataDuringCreation",
	)
	if err != nil {
		return "", err
	}
	if !uploadDataDuringCreation {
		return "", fmt.Errorf("creation-with-upload scenario must set uploadDataDuringCreation")
	}
	if err := api2devdock.RequireFullFileChunkSize(scenario); err != nil {
		return "", err
	}
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return "", err
	}
	content, err := api2devdock.ScenarioBytes(scenario)
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
		Context:                  ctx,
		Fingerprint:              scenarioID + "-fingerprint",
		Metadata:                 metadata,
		RetryDelays:              retryDelays,
		Size:                     int64(len(content)),
		Source:                   bytes.NewReader(content),
		Storage:                  tusgo.NewMemoryURLStorage(),
		UploadDataDuringCreation: true,
	})
	if err != nil {
		return "", err
	}
	if createdUpload == nil || createdUpload.Location == "" {
		return "", fmt.Errorf("creation-with-upload TUS upload did not expose an upload URL")
	}
	if createdUpload.RemoteOffset != int64(len(content)) {
		return "", fmt.Errorf(
			"creation-with-upload accepted %d bytes, expected %d",
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
		"examples/api2-devdock-tus-creation-with-upload/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	uploadURL, err := uploadWithCreationData(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("creation-with-upload: %v", err)
	}
	if err := api2devdock.WriteResult(map[string]interface{}{"uploadUrl": uploadURL}); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s uploaded to %s\n", scenarioID, uploadURL)
}
