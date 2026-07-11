package tusgo

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGeneratedURLStorageOptionValidation(t *testing.T) {
	testCases := []struct {
		name                     string
		content                  string
		parallelUploads          int
		uploadDataDuringCreation bool
		uploadLengthDeferred     bool
		expectedError            string
	}{
		{
			name:                     "startValidationParallelUploadsWithDeferredLength",
			content:                  "hello world",
			parallelUploads:          2,
			uploadDataDuringCreation: false,
			uploadLengthDeferred:     true,
			expectedError:            "tus: cannot use the `uploadLengthDeferred` option when parallelUploads is enabled",
		},
		{
			name:                     "startValidationParallelUploadsWithUploadDataDuringCreation",
			content:                  "hello world",
			parallelUploads:          2,
			uploadDataDuringCreation: true,
			uploadLengthDeferred:     false,
			expectedError:            "tus: cannot use the `uploadDataDuringCreation` option when parallelUploads is enabled",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				requestCount += 1
				responseWriter.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			baseURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			client := NewClient(http.DefaultClient, baseURL)
			upload, err := client.UploadWithURLStorage(URLStorageUploadOptions{
				Storage:                  NewMemoryURLStorage(),
				Source:                   strings.NewReader(testCase.content),
				Fingerprint:              "contract-" + testCase.name,
				Size:                     int64(len(testCase.content)),
				ParallelUploads:          testCase.parallelUploads,
				UploadDataDuringCreation: testCase.uploadDataDuringCreation,
				UploadLengthDeferred:     testCase.uploadLengthDeferred,
			})
			if err == nil {
				t.Fatalf("expected validation error %q", testCase.expectedError)
			}
			if err.Error() != testCase.expectedError {
				t.Fatalf("expected validation error %q, got %q", testCase.expectedError, err.Error())
			}
			if upload != nil {
				t.Fatalf("expected validation to fail before creating an upload, got %#v", upload)
			}
			if requestCount != 0 {
				t.Fatalf("expected validation to fail before any request, got %d request(s)", requestCount)
			}
		})
	}
}
