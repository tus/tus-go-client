//go:build api2devdock

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func uploadWithTus(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (string, error) {
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return "", err
	}
	content, err := api2devdock.ScenarioBytes(scenario)
	if err != nil {
		return "", err
	}
	if err := api2devdock.RequireFullFileChunkSize(scenario); err != nil {
		return "", err
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return "", err
	}

	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithContext(ctx)
	upload := tusgo.Upload{}
	if _, err := client.CreateUpload(&upload, int64(len(content)), false, metadata); err != nil {
		return "", err
	}
	if upload.Location == "" {
		return "", fmt.Errorf("created upload did not include a Location")
	}

	stream := tusgo.NewUploadStream(client, &upload)
	stream.ChunkSize = tusgo.NoChunked
	written, err := stream.Write(content)
	if err != nil {
		return "", err
	}
	if written != len(content) {
		return "", fmt.Errorf("wrote %d bytes, expected %d", written, len(content))
	}
	if upload.RemoteOffset != int64(len(content)) {
		return "", fmt.Errorf("remote offset %d, expected %d", upload.RemoteOffset, len(content))
	}

	return upload.Location, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-transloadit-assembly-upload/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}

	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	uploadURL, err := uploadWithTus(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("upload: %v", err)
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
