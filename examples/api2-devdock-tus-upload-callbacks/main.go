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

type eventRecordingReadSeeker struct {
	*bytes.Reader
	callbacks api2devdock.UploadCallbacksPlan
	events    *[]string
}

func (source *eventRecordingReadSeeker) Close() error {
	*source.events = append(
		*source.events,
		api2devdock.UploadCallbackEventKey(
			source.callbacks,
			source.callbacks.EventKinds.SourceClose,
		),
	)

	return nil
}

func uploadEventHooks(
	callbacks api2devdock.UploadCallbacksPlan,
	events *[]string,
) tusgo.UploadEventHooks {
	return tusgo.UploadEventHooks{
		OnUploadURLAvailable: func() error {
			*events = append(
				*events,
				api2devdock.UploadCallbackEventKey(
					callbacks,
					callbacks.EventKinds.UploadURLAvailable,
				),
			)
			return nil
		},
		OnProgress: func(bytesSent int64, bytesTotal *int64) error {
			*events = append(
				*events,
				api2devdock.UploadCallbackEventKey(
					callbacks,
					callbacks.EventKinds.Progress,
					api2devdock.UploadCallbackEventKeyNumber(bytesSent),
					api2devdock.UploadCallbackEventKeyTotal(bytesTotal),
				),
			)
			return nil
		},
		OnChunkComplete: func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error {
			*events = append(
				*events,
				api2devdock.UploadCallbackEventKey(
					callbacks,
					callbacks.EventKinds.ChunkComplete,
					api2devdock.UploadCallbackEventKeyNumber(chunkSize),
					api2devdock.UploadCallbackEventKeyNumber(bytesAccepted),
					api2devdock.UploadCallbackEventKeyTotal(bytesTotal),
				),
			)
			return nil
		},
		OnSuccess: func(payload tusgo.UploadSuccessPayload) error {
			if payload.Upload == nil || payload.Upload.Location == "" {
				return fmt.Errorf("upload callback success payload did not include an upload URL")
			}
			*events = append(
				*events,
				api2devdock.UploadCallbackEventKey(callbacks, callbacks.EventKinds.Success),
			)
			return nil
		},
	}
}

func uploadWithCallbacks(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]interface{}, error) {
	callbacks, err := api2devdock.UploadCallbacks(scenario)
	if err != nil {
		return nil, err
	}
	if err := api2devdock.RequireFullFileChunkSize(scenario); err != nil {
		return nil, err
	}
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return nil, err
	}
	content, err := api2devdock.ScenarioBytes(scenario)
	if err != nil {
		return nil, err
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return nil, err
	}
	retryDelays, err := api2devdock.RetryDelays(scenario)
	if err != nil {
		return nil, err
	}
	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		return nil, err
	}

	events := []string{}
	source := &eventRecordingReadSeeker{
		Reader:    bytes.NewReader(content),
		callbacks: callbacks,
		events:    &events,
	}
	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithContext(ctx)
	createdUpload, err := client.UploadWithURLStorage(tusgo.URLStorageUploadOptions{
		Context:     ctx,
		EventHooks:  uploadEventHooks(callbacks, &events),
		Fingerprint: scenarioID + "-fingerprint",
		Metadata:    metadata,
		RetryDelays: retryDelays,
		Size:        int64(len(content)),
		Source:      source,
		Storage:     tusgo.NewMemoryURLStorage(),
	})
	if err != nil {
		return nil, err
	}
	if createdUpload == nil || createdUpload.Location == "" {
		return nil, fmt.Errorf("upload callback TUS upload did not expose an upload URL")
	}
	if createdUpload.RemoteOffset != int64(len(content)) {
		return nil, fmt.Errorf(
			"upload callback scenario accepted %d bytes, expected %d",
			createdUpload.RemoteOffset,
			len(content),
		)
	}
	matchedEvents, err := api2devdock.MatchUploadCallbackEventKeys(callbacks, events)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"eventKeys":    matchedEvents,
		"rawEventKeys": events,
		"uploadUrl":    createdUpload.Location,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-upload-callbacks/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadWithCallbacks(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("upload callbacks: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf(
		"Go TUS SDK devdock scenario %s observed upload callbacks for %s\n",
		scenarioID,
		result["uploadUrl"],
	)
}
