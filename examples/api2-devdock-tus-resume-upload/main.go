//go:build api2devdock

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	"github.com/bdragon300/tusgo/examples/api2devdock"
)

func uploadOptions(
	scenario map[string]interface{},
	createResponse map[string]interface{},
	storage tusgo.URLStorage,
	content []byte,
) (tusgo.URLStorageUploadOptions, error) {
	chunkSize, err := api2devdock.FixedChunkSizeBytes(scenario)
	if err != nil {
		return tusgo.URLStorageUploadOptions{}, err
	}
	resume, err := api2devdock.Resume(scenario)
	if err != nil {
		return tusgo.URLStorageUploadOptions{}, err
	}
	metadata, err := api2devdock.UploadMetadata(scenario, createResponse)
	if err != nil {
		return tusgo.URLStorageUploadOptions{}, err
	}

	return tusgo.URLStorageUploadOptions{
		ChunkSize:                  chunkSize,
		Fingerprint:                resume.Fingerprint,
		Metadata:                   metadata,
		RemoveFingerprintOnSuccess: resume.RemoveFingerprintOnSuccess,
		Size:                       int64(len(content)),
		Source:                     bytes.NewReader(content),
		Storage:                    storage,
	}, nil
}

func uploadFirstChunkAndAbort(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
	storage tusgo.URLStorage,
	content []byte,
) (int, string, error) {
	resume, err := api2devdock.Resume(scenario)
	if err != nil {
		return 0, "", err
	}
	options, err := uploadOptions(scenario, createResponse, storage, content)
	if err != nil {
		return 0, "", err
	}
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return 0, "", err
	}

	abortCtx, cancelAbort := context.WithCancel(ctx)
	defer cancelAbort()

	var firstUploadURL string
	options.Context = abortCtx
	options.EventHooks = tusgo.UploadEventHooks{
		OnChunkComplete: func(_ int64, bytesAccepted int64, _ *int64) error {
			if int(bytesAccepted) < resume.StopAfterAcceptedBytes {
				return nil
			}

			cancelAbort()
			return nil
		},
		OnUploadURLAvailable: func() error {
			storedUploads, err := storage.FindUploadsByFingerprint(resume.Fingerprint)
			if err != nil {
				return err
			}
			if len(storedUploads) == 0 {
				return fmt.Errorf("resume scenario did not store the first upload URL")
			}
			uploadURL, ok := storedUploads[0]["uploadUrl"].(string)
			if !ok || uploadURL == "" {
				return fmt.Errorf("resume scenario stored upload is missing uploadUrl")
			}
			firstUploadURL = uploadURL
			return nil
		},
	}

	client := tusgo.NewClient(http.DefaultClient, endpointURL)
	upload, err := client.UploadWithURLStorage(options)
	if !errors.Is(err, context.Canceled) {
		return 0, "", fmt.Errorf("expected context cancellation, got upload=%#v err=%v", upload, err)
	}
	if firstUploadURL == "" {
		return 0, "", fmt.Errorf("resume scenario did not capture the first upload URL")
	}
	if upload == nil {
		return 0, "", fmt.Errorf("resume scenario did not return the aborted upload")
	}

	return int(upload.RemoteOffset), firstUploadURL, nil
}

func resumeStoredUpload(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
	storage tusgo.URLStorage,
	content []byte,
) (int, int, string, error) {
	resume, err := api2devdock.Resume(scenario)
	if err != nil {
		return 0, 0, "", err
	}
	previousUploads, err := storage.FindUploadsByFingerprint(resume.Fingerprint)
	if err != nil {
		return 0, 0, "", err
	}
	if len(previousUploads) != resume.ExpectedPreviousUploadCount {
		return 0, 0, "", fmt.Errorf(
			"expected %d stored upload(s), got %d",
			resume.ExpectedPreviousUploadCount,
			len(previousUploads),
		)
	}
	options, err := uploadOptions(scenario, createResponse, storage, content)
	if err != nil {
		return 0, 0, "", err
	}
	options.Context = ctx
	endpointURL, err := api2devdock.TusURL(scenario, createResponse)
	if err != nil {
		return 0, 0, "", err
	}

	client := tusgo.NewClient(http.DefaultClient, endpointURL)
	upload, err := client.UploadWithURLStorage(options)
	if err != nil {
		return 0, 0, "", err
	}
	if upload == nil || upload.Location == "" {
		return 0, 0, "", fmt.Errorf("resumed TUS upload did not expose an upload URL")
	}

	remainingUploads, err := storage.FindUploadsByFingerprint(resume.Fingerprint)
	if err != nil {
		return 0, 0, "", err
	}
	if len(remainingUploads) != resume.ExpectedRemainingPreviousUploadCount {
		return 0, 0, "", fmt.Errorf(
			"expected %d stored upload(s) after success, got %d",
			resume.ExpectedRemainingPreviousUploadCount,
			len(remainingUploads),
		)
	}

	return len(previousUploads), len(remainingUploads), upload.Location, nil
}

func uploadWithStoredResume(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]interface{}, error) {
	content, err := api2devdock.ScenarioBytes(scenario)
	if err != nil {
		return nil, err
	}
	tempDir, err := os.MkdirTemp("", "api2-tus-go-resume-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	storage := tusgo.NewFileURLStorage(filepath.Join(tempDir, "url-storage.json"))
	firstAcceptedBytes, firstUploadURL, err := uploadFirstChunkAndAbort(
		ctx,
		scenario,
		createResponse,
		storage,
		content,
	)
	if err != nil {
		return nil, err
	}
	previousUploadCount, remainingPreviousUploadCount, uploadURL, err := resumeStoredUpload(
		ctx,
		scenario,
		createResponse,
		storage,
		content,
	)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"firstAcceptedBytes":           firstAcceptedBytes,
		"firstUploadUrl":               firstUploadURL,
		"previousUploadCount":          previousUploadCount,
		"remainingPreviousUploadCount": remainingPreviousUploadCount,
		"uploadUrl":                    uploadURL,
	}, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := api2devdock.LoadScenario(
		"examples/api2-devdock-tus-resume-upload/api2-scenario.json",
	)
	if err != nil {
		api2devdock.Fail("load scenario: %v", err)
	}
	createResponse, err := api2devdock.CreateResponseFromScenario(scenario)
	if err != nil {
		api2devdock.Fail("read prepared create response: %v", err)
	}

	result, err := uploadWithStoredResume(ctx, scenario, createResponse)
	if err != nil {
		api2devdock.Fail("resume upload: %v", err)
	}
	if err := api2devdock.WriteResult(result); err != nil {
		api2devdock.Fail("write result: %v", err)
	}

	scenarioID, err := api2devdock.ScenarioID(scenario)
	if err != nil {
		api2devdock.Fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s resumed %s\n", scenarioID, result["uploadUrl"])
}
