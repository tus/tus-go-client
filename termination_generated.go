// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"net/http"
	"time"
)

type TerminateUploadOptions struct {
	RetryDelays   []time.Duration
	OnShouldRetry func(error, int) bool
}

func (c *Client) TerminateUploadWithRetry(upload Upload, options TerminateUploadOptions) (*http.Response, error) {
	retryDelays := generatedTusRetryDelays(options.RetryDelays)
	retryAttempt := 0

	for {
		response, err := c.DeleteUpload(upload)
		if err == nil {
			return response, nil
		}

		statusCode := 0
		if response != nil {
			statusCode = response.StatusCode
		}
		if !generatedTusShouldScheduleRetry(
			options.OnShouldRetry,
			err,
			statusCode,
			retryAttempt,
			retryDelays,
		) {
			return response, err
		}

		delay := retryDelays[retryAttempt]
		if delay > 0 {
			time.Sleep(delay)
		}
		retryAttempt = generatedTusNextRetryAttempt(retryAttempt)
	}
}
