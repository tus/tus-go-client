// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"strconv"
	"strings"
	"testing"
)

type generatedTusWireVersion struct {
	Default bool
	Value   string
}

type generatedTusHeaderField struct {
	DisplayName string
	Name        string
	Required    bool
}

type generatedTusHeaderVariant struct {
	Fields []generatedTusHeaderField
}

type generatedTusRequestContract struct {
	BodyKind       string
	ContentType    string
	HeaderVariants []generatedTusHeaderVariant
}

type generatedTusResponseContract struct {
	StatusCode     int
	BodyKind       string
	HeaderVariants []generatedTusHeaderVariant
}

type generatedTusProtocolOperation struct {
	OperationID string
	Role        string
	Method      string
	Path        string
	Request     generatedTusRequestContract
	Responses   []generatedTusResponseContract
}

type generatedTusClientFeature struct {
	Conformance  generatedTusClientFeatureConformance
	Description  string
	FeatureID    string
	Flow         []generatedTusClientFeatureFlowStep
	OperationIDs []string
	Primitives   []string
}

type generatedTusClientFeatureConformance struct {
	ScenarioIDs []string
	Status      string
}

type generatedTusClientFeatureFlowStep struct {
	Kind        string
	OperationID string
	Primitive   string
	Condition   string
	Summary     string
}

type generatedTusClientFlowContract struct {
	UrlStorage generatedTusClientUrlStoragePolicy
}

type generatedTusClientUrlStoragePolicy struct {
	ID        generatedTusClientUrlStorageIDPolicy
	Namespace string
	Separator string
}

type generatedTusClientUrlStorageIDPolicy struct {
	Multiplier float64
	Strategy   string
}

var generatedTusDefaultRequestHeaderValues = map[string]string{"Tus-Resumable": "1.0.0"}
var generatedTusDefaultResponseHeaderValues = map[string]string{"Tus-Resumable": "1.0.0"}

func generatedTusHeaderValue(defaultValues map[string]string, values map[string]string, name string) string {
	if value, ok := values[name]; ok {
		return value
	}

	return defaultValues[name]
}

func generatedTusRequestHeaderValue(values map[string]string, name string) string {
	return generatedTusHeaderValue(generatedTusDefaultRequestHeaderValues, values, name)
}

func generatedTusResponseHeaderValue(values map[string]string, name string) string {
	return generatedTusHeaderValue(generatedTusDefaultResponseHeaderValues, values, name)
}

type generatedTusClientUrlStorageConformanceScenario struct {
	Actions    []generatedTusClientUrlStorageConformanceAction
	Backend    string
	FeatureID  string
	Runtimes   []string
	ScenarioID string
}

type generatedTusClientUrlStorageConformanceAction struct {
	ExpectedKeyPrefix string
	ExpectedKeyRefs   []string
	Fingerprint       string
	KeyRef            string
	Kind              string
	Upload            map[string]any
}

type generatedTusManagedUploadProofCase struct {
	FeatureID          string
	Layer              string
	ScenarioID         string
	RequiredPrimitives []string
	ProtocolFeatureIDs []string
	RuntimeProfiles    []string
}

var generatedTusWireVersions = []generatedTusWireVersion{
	{
		Default: true,
		Value:   "1.0.0",
	},
}

var generatedTusProtocolOperations = []generatedTusProtocolOperation{
	{
		OperationID: "discoverTusCapabilities",
		Role:        "capability-discovery",
		Method:      "OPTIONS",
		Path:        "/resumable/files/",
		Request: generatedTusRequestContract{
			BodyKind:       "empty",
			ContentType:    "",
			HeaderVariants: nil,
		},
		Responses: []generatedTusResponseContract{
			{
				StatusCode: 200,
				BodyKind:   "empty",
				HeaderVariants: []generatedTusHeaderVariant{
					{
						Fields: []generatedTusHeaderField{
							{
								DisplayName: "Tus-Extension",
								Name:        "tus-extension",
								Required:    true,
							},
							{
								DisplayName: "Tus-Max-Size",
								Name:        "tus-max-size",
								Required:    true,
							},
							{
								DisplayName: "Tus-Resumable",
								Name:        "tus-resumable",
								Required:    true,
							},
							{
								DisplayName: "Tus-Version",
								Name:        "tus-version",
								Required:    true,
							},
						},
					},
				},
			},
		},
	},
	{
		OperationID: "createTusUpload",
		Role:        "creation",
		Method:      "POST",
		Path:        "/resumable/files/",
		Request: generatedTusRequestContract{
			BodyKind:    "empty",
			ContentType: "",
			HeaderVariants: []generatedTusHeaderVariant{
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
						{
							DisplayName: "Upload-Length",
							Name:        "upload-length",
							Required:    true,
						},
						{
							DisplayName: "Upload-Metadata",
							Name:        "upload-metadata",
							Required:    true,
						},
					},
				},
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
						{
							DisplayName: "Upload-Defer-Length",
							Name:        "upload-defer-length",
							Required:    true,
						},
						{
							DisplayName: "Upload-Metadata",
							Name:        "upload-metadata",
							Required:    true,
						},
					},
				},
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
						{
							DisplayName: "Upload-Concat",
							Name:        "upload-concat",
							Required:    true,
						},
						{
							DisplayName: "Upload-Length",
							Name:        "upload-length",
							Required:    true,
						},
						{
							DisplayName: "Upload-Metadata",
							Name:        "upload-metadata",
							Required:    false,
						},
					},
				},
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
						{
							DisplayName: "Upload-Concat",
							Name:        "upload-concat",
							Required:    true,
						},
						{
							DisplayName: "Upload-Metadata",
							Name:        "upload-metadata",
							Required:    false,
						},
					},
				},
			},
		},
		Responses: []generatedTusResponseContract{
			{
				StatusCode: 201,
				BodyKind:   "empty",
				HeaderVariants: []generatedTusHeaderVariant{
					{
						Fields: []generatedTusHeaderField{
							{
								DisplayName: "Location",
								Name:        "location",
								Required:    true,
							},
							{
								DisplayName: "Tus-Resumable",
								Name:        "tus-resumable",
								Required:    true,
							},
						},
					},
				},
			},
		},
	},
	{
		OperationID: "getTusUploadOffset",
		Role:        "offset-discovery",
		Method:      "HEAD",
		Path:        "/resumable/files/{upload_id}",
		Request: generatedTusRequestContract{
			BodyKind:    "empty",
			ContentType: "",
			HeaderVariants: []generatedTusHeaderVariant{
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
					},
				},
			},
		},
		Responses: []generatedTusResponseContract{
			{
				StatusCode: 200,
				BodyKind:   "empty",
				HeaderVariants: []generatedTusHeaderVariant{
					{
						Fields: []generatedTusHeaderField{
							{
								DisplayName: "Tus-Resumable",
								Name:        "tus-resumable",
								Required:    true,
							},
							{
								DisplayName: "Upload-Length",
								Name:        "upload-length",
								Required:    true,
							},
							{
								DisplayName: "Upload-Offset",
								Name:        "upload-offset",
								Required:    true,
							},
						},
					},
					{
						Fields: []generatedTusHeaderField{
							{
								DisplayName: "Tus-Resumable",
								Name:        "tus-resumable",
								Required:    true,
							},
							{
								DisplayName: "Upload-Defer-Length",
								Name:        "upload-defer-length",
								Required:    true,
							},
							{
								DisplayName: "Upload-Offset",
								Name:        "upload-offset",
								Required:    true,
							},
						},
					},
				},
			},
		},
	},
	{
		OperationID: "patchTusUpload",
		Role:        "upload-chunk",
		Method:      "PATCH",
		Path:        "/resumable/files/{upload_id}",
		Request: generatedTusRequestContract{
			BodyKind:    "binary",
			ContentType: "application/offset+octet-stream",
			HeaderVariants: []generatedTusHeaderVariant{
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Content-Type",
							Name:        "content-type",
							Required:    true,
						},
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
						{
							DisplayName: "Upload-Offset",
							Name:        "upload-offset",
							Required:    true,
						},
					},
				},
			},
		},
		Responses: []generatedTusResponseContract{
			{
				StatusCode: 204,
				BodyKind:   "empty",
				HeaderVariants: []generatedTusHeaderVariant{
					{
						Fields: []generatedTusHeaderField{
							{
								DisplayName: "Tus-Resumable",
								Name:        "tus-resumable",
								Required:    true,
							},
							{
								DisplayName: "Upload-Offset",
								Name:        "upload-offset",
								Required:    true,
							},
						},
					},
				},
			},
		},
	},
	{
		OperationID: "terminateTusUpload",
		Role:        "termination",
		Method:      "DELETE",
		Path:        "/resumable/files/{upload_id}",
		Request: generatedTusRequestContract{
			BodyKind:    "empty",
			ContentType: "",
			HeaderVariants: []generatedTusHeaderVariant{
				{
					Fields: []generatedTusHeaderField{
						{
							DisplayName: "Tus-Resumable",
							Name:        "tus-resumable",
							Required:    true,
						},
					},
				},
			},
		},
		Responses: []generatedTusResponseContract{
			{
				StatusCode: 204,
				BodyKind:   "empty",
				HeaderVariants: []generatedTusHeaderVariant{
					{
						Fields: []generatedTusHeaderField{
							{
								DisplayName: "Tus-Resumable",
								Name:        "tus-resumable",
								Required:    true,
							},
						},
					},
				},
			},
		},
	},
	{
		OperationID: "downloadTusUpload",
		Role:        "download",
		Method:      "GET",
		Path:        "/resumable/files/{upload_id}",
		Request: generatedTusRequestContract{
			BodyKind:       "empty",
			ContentType:    "",
			HeaderVariants: nil,
		},
		Responses: []generatedTusResponseContract{
			{
				StatusCode:     200,
				BodyKind:       "binary",
				HeaderVariants: nil,
			},
		},
	},
}

var generatedTusClientFeatures = []generatedTusClientFeature{
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"singleUploadLifecycle"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Create an upload, store its URL, upload bytes, and finish successfully.",
		FeatureID:   "singleUploadLifecycle",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "open-input-source",
				Condition:   "",
				Summary:     "Open the caller input as a sliceable source.",
			},
			{
				Kind:        "operation",
				OperationID: "createTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Create the remote upload resource.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Upload bytes until the accepted offset reaches the known length.",
			},
		},
		OperationIDs: []string{"createTusUpload", "getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"open-input-source", "fingerprint-input", "store-resume-url", "retry-with-backoff", "emit-progress", "abort-current-request"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"resumeFromPreviousUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Resume a stored upload URL by discovering the remote offset before patching.",
		FeatureID:   "resumeUpload",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "resume-from-previous-upload",
				Condition:   "",
				Summary:     "Load a stored upload URL selected by fingerprint.",
			},
			{
				Kind:        "operation",
				OperationID: "getTusUploadOffset",
				Primitive:   "",
				Condition:   "",
				Summary:     "Read the server offset for the stored upload URL.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Continue uploading from the discovered offset.",
			},
		},
		OperationIDs: []string{"getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"fingerprint-input", "resume-from-previous-upload", "store-resume-url"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"deferredLengthUpload", "deferredLengthChunkedUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Create an upload without a known length and declare the length on the final upload request.",
		FeatureID:   "deferredLengthUpload",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "operation",
				OperationID: "createTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Create the upload with deferred length.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "defer-upload-length",
				Condition:   "",
				Summary:     "Track the source until the final upload request reveals the total size.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Declare Upload-Length on the final upload request.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"defer-upload-length", "emit-chunk-complete", "emit-progress"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"creationWithUpload", "creationWithUploadPartialChunk"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Send the first bytes on the creation request when the server/client support it.",
		FeatureID:   "creationWithUpload",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "operation",
				OperationID: "createTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Create the upload while streaming the initial body.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "upload-during-creation",
				Condition:   "",
				Summary:     "Interpret the creation response as an accepted offset.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"upload-during-creation", "emit-progress"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"uploadBodyHeaders"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Send protocol-specific upload body headers whenever the client transmits file bytes.",
		FeatureID:   "uploadBodyHeaders",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "send-upload-body-headers",
				Condition:   "",
				Summary:     "Attach the protocol-specific upload body content type when a request has bytes.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Upload bytes with the protocol-specific body headers.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"send-upload-body-headers"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"customRequestHeaders"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Apply user-provided request headers to every upload request.",
		FeatureID:   "customRequestHeaders",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "apply-custom-request-headers",
				Condition:   "",
				Summary:     "Merge user-provided headers after protocol headers are prepared.",
			},
			{
				Kind:        "operation",
				OperationID: "createTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Create uploads with the configured custom headers.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Upload bytes with the configured custom headers.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"apply-custom-request-headers"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"requestIdHeaders"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Add generated request IDs after protocol and custom request headers.",
		FeatureID:   "requestIdHeaders",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "add-request-id-header",
				Condition:   "",
				Summary:     "Generate a request ID and apply it after custom request headers so it is authoritative.",
			},
			{
				Kind:        "operation",
				OperationID: "createTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Create uploads with a generated request ID.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Upload bytes with a generated request ID.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"add-request-id-header", "apply-custom-request-headers"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"overridePatchMethod"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Tunnel PATCH through POST with the method-override header.",
		FeatureID:   "overridePatchMethod",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "operation",
				OperationID: "getTusUploadOffset",
				Primitive:   "",
				Condition:   "",
				Summary:     "Resume from the upload URL before sending bytes.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "override-patch-method",
				Condition:   "",
				Summary:     "Replace PATCH with POST while preserving the protocol operation intent.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Upload bytes through the overridden request.",
			},
		},
		OperationIDs: []string{"getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"override-patch-method"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"parallelUploadConcat", "parallelUploadAbortCleanup"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Split one input into partial uploads, run the parts concurrently, clean up aborted parts, and concatenate their upload URLs.",
		FeatureID:   "parallelUploadConcat",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "split-parallel-upload-boundaries",
				Condition:   "",
				Summary:     "Split the input into stable byte ranges.",
			},
			{
				Kind:        "operation",
				OperationID: "createTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Create partial uploads for each range.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "concatenate-partial-uploads",
				Condition:   "",
				Summary:     "Create the final upload from completed partial upload URLs.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"abort-current-request", "concatenate-partial-uploads", "emit-progress", "split-parallel-upload-boundaries", "terminate-upload"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"retryPatchAfterOffsetRecovery"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Recover from a failed chunk by reading the server offset before retrying.",
		FeatureID:   "retryOffsetRecovery",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Attempt the chunk upload.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "recover-offset-after-error",
				Condition:   "",
				Summary:     "Discover the accepted offset after a retryable failure.",
			},
			{
				Kind:        "operation",
				OperationID: "getTusUploadOffset",
				Primitive:   "",
				Condition:   "",
				Summary:     "Use HEAD to recover the offset before retrying PATCH.",
			},
		},
		OperationIDs: []string{"createTusUpload", "getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"retry-with-backoff", "recover-offset-after-error"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"retryPatchAfterOffsetRecovery"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Schedule retry timers and reset retry attempts after accepted progress.",
		FeatureID:   "retryStateTransitions",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "schedule-retry-timer",
				Condition:   "",
				Summary:     "Consume the current retry delay and restart the upload after that timer fires.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "reset-retry-attempt-after-progress",
				Condition:   "",
				Summary:     "Reset retry attempts once a later retry observes server-side offset progress.",
			},
		},
		OperationIDs: []string{"getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"retry-with-backoff", "schedule-retry-timer", "reset-retry-attempt-after-progress"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"terminateWithRetry"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Terminate an upload resource and retry retryable termination failures.",
		FeatureID:   "terminateUpload",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "terminate-upload",
				Condition:   "",
				Summary:     "Choose server-side termination for an upload URL.",
			},
			{
				Kind:        "operation",
				OperationID: "terminateTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Delete the upload resource.",
			},
		},
		OperationIDs: []string{"terminateTusUpload"},
		Primitives:   []string{"terminate-upload", "retry-with-backoff"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"abortUpload", "abortUploadAfterStoredUrl"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Abort the active request, pending retry timer, and any partial uploads.",
		FeatureID:   "abortUpload",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "abort-current-request",
				Condition:   "",
				Summary:     "Cancel in-flight transport work without emitting user callbacks after abort.",
			},
		},
		OperationIDs: []string{"terminateTusUpload"},
		Primitives:   []string{"abort-current-request", "terminate-upload"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"singleUploadLifecycle", "creationWithUpload", "resumeFromPreviousUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Expose progress and accepted-chunk callbacks from runtime upload activity.",
		FeatureID:   "uploadCallbacks",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "emit-progress",
				Condition:   "",
				Summary:     "Report bytes sent against known or deferred length.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "emit-chunk-complete",
				Condition:   "",
				Summary:     "Report chunk size, accepted offset, and total size after server acceptance.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "emit-upload-url",
				Condition:   "",
				Summary:     "Notify once a usable upload URL is known.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"emit-progress", "emit-chunk-complete", "emit-upload-url"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"requestLifecycleHooks", "retryPatchAfterOffsetRecovery"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Run before-request, after-response, and custom retry hooks around transport.",
		FeatureID:   "requestLifecycleHooks",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "run-request-hooks",
				Condition:   "",
				Summary:     "Call user hooks around each HTTP request/response pair.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "customize-retry",
				Condition:   "",
				Summary:     "Let user retry policy override default retry decisions.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"customize-retry", "run-request-hooks"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"singleUploadLifecycle", "resumeFromPreviousUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Persist, find, resume, and optionally remove upload URLs by fingerprint.",
		FeatureID:   "resumeUrlStorage",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "fingerprint-input",
				Condition:   "",
				Summary:     "Derive a stable key for the input when possible.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "store-resume-url",
				Condition:   "",
				Summary:     "Persist upload URLs and partial-upload URLs for future resumption.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "remove-stored-url-on-success",
				Condition:   "",
				Summary:     "Remove stored upload URLs when configured after success or invalidation.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"fingerprint-input", "store-resume-url", "remove-stored-url-on-success"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"arrayBufferInput", "arrayBufferViewInput", "webReadableStreamInput", "nodeReadableStreamInput", "nodePathInput"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Support the reference client input/source families across runtimes.",
		FeatureID:   "inputSources",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "read-browser-file",
				Condition:   "",
				Summary:     "Read browser Blob/File and ArrayBuffer-family inputs.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "read-node-stream",
				Condition:   "",
				Summary:     "Read Node streams when size and chunk constraints are satisfied.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "read-web-stream",
				Condition:   "",
				Summary:     "Read Web Streams with deferred or configured size.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "read-node-file",
				Condition:   "",
				Summary:     "Read filesystem paths and fs streams, including parallel ranges.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"read-browser-file", "read-node-file", "read-node-stream", "read-web-stream"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"webStorageUrlStorageBackend", "fileUrlStorageBackend"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Support browser and file-backed URL storage implementations.",
		FeatureID:   "urlStorageBackends",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "store-browser-url",
				Condition:   "",
				Summary:     "Persist upload records in browser localStorage.",
			},
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "store-file-url",
				Condition:   "",
				Summary:     "Persist upload records in the Node file store.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"store-browser-url", "store-file-url"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"ietfDraft05CreationWithUpload", "ietfDraft05ChunkedUploadComplete", "ietfDraft03ResumeWithoutKnownLength"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Select between tus v1 and supported IETF draft client protocol modes.",
		FeatureID:   "protocolVersionSelection",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "select-client-protocol",
				Condition:   "",
				Summary:     "Choose request headers and response expectations for the selected protocol.",
			},
		},
		OperationIDs: []string{"createTusUpload", "getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"select-client-protocol"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"relativeLocationResolution"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Normalize relative Location headers against the request endpoint.",
		FeatureID:   "relativeLocationResolution",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "resolve-relative-location",
				Condition:   "",
				Summary:     "Resolve server Location headers with the creation endpoint as origin.",
			},
		},
		OperationIDs: []string{"createTusUpload"},
		Primitives:   []string{"resolve-relative-location"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"startValidationMissingInput", "startValidationMissingEndpointOrUploadUrl", "startValidationUnsupportedProtocol", "startValidationRetryDelaysNotArray", "startValidationParallelUploadsWithUploadUrl", "startValidationParallelUploadsWithUploadSize", "startValidationParallelUploadsWithDeferredLength", "startValidationParallelUploadsWithUploadDataDuringCreation", "startValidationParallelBoundariesWithoutParallelUploads", "startValidationParallelBoundariesLengthMismatch"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Validate option combinations before starting runtime work.",
		FeatureID:   "startOptionValidation",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "validate-start-options",
				Condition:   "",
				Summary:     "Reject missing inputs and incompatible parallel/deferred/resume options.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"validate-start-options"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"detailedCreateResponseError", "detailedCreateRequestError"},
			Status:      "covered-by-generated-scenario",
		},
		Description: "Attach request, response, status, body, and request ID context to errors.",
		FeatureID:   "detailedErrors",
		Flow: []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "report-detailed-errors",
				Condition:   "",
				Summary:     "Return user-facing errors with enough transport context for debugging.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"report-detailed-errors"},
	},
}

const generatedTusManagedUploadJSON = `{
  "capabilities": {
    "cleanup": {
      "policies": [
        "absent-after-source-unavailable",
        "remove-owned-source-after-success",
        "remove-owned-source-after-cancel",
        "retain-owned-source-while-deferred",
        "retain-owned-source-after-permanent-failure",
        "retain-source-after-retryable-failure",
        "remove-managed-state-after-terminal-retention"
      ]
    },
    "failureClassification": {
      "permanentFailures": [
        "source-unavailable",
        "unretryable-protocol-error",
        "retry-policy-exhausted"
      ],
      "retryableFailures": [
        "retryable-protocol-error",
        "io-error",
        "network-unavailable"
      ]
    },
    "networkConstraints": {
      "options": [
        "any-network",
        "unmetered-network"
      ]
    },
    "retryPolicy": {
      "controls": [
        "max-attempts",
        "deadline",
        "progress-sensitive-budget",
        "unbounded-until-permanent-failure"
      ],
      "permanentFailure": "stop-without-retry",
      "progressReset": "reset-budget-after-accepted-offset-advances"
    },
    "scheduling": {
      "strategies": [
        "foreground-task",
        "process-lifetime-worker-pool",
        "durable-os-scheduler"
      ]
    },
    "sourceDurability": {
      "ownedCopyCleanup": "after-success-or-cancel",
      "strategies": [
        "copy-to-owned-storage",
        "reference-original-source",
        "memory-only"
      ]
    },
    "stateReporting": {
      "states": [
        "pending",
        "running",
        "succeeded",
        "failed"
      ],
      "terminalRetention": "session-and-next-launch",
      "transientRetention": "until-terminal"
    }
  },
  "conformance": {
    "scenarioIds": [
      "managedUploadDurableRetry",
      "managedUploadPermanentFailure",
      "managedUploadRetryPolicyExhausted",
      "managedUploadSourceUnavailable",
      "managedUploadNetworkConstraint"
    ],
    "status": "covered-by-generated-scenario"
  },
  "description": "Submit upload work that can make sources durable, schedule/resume execution, retry, report state, and clean up while reusing the raw TUS protocol features underneath.",
  "featureId": "managedUpload",
  "flow": [
    {
      "kind": "managed-primitive",
      "primitive": "accept-upload-submission",
      "summary": "Accept source, metadata, headers, endpoint, and retry/scheduling policy."
    },
    {
      "kind": "managed-primitive",
      "primitive": "make-source-durable",
      "summary": "Keep the source readable according to the selected runtime durability strategy."
    },
    {
      "kind": "managed-primitive",
      "primitive": "schedule-upload-work",
      "summary": "Run upload work according to the runtime scheduler capability."
    },
    {
      "featureId": "singleUploadLifecycle",
      "kind": "protocol-feature",
      "summary": "Use the raw protocol upload lifecycle for each execution attempt."
    },
    {
      "featureId": "retryOffsetRecovery",
      "kind": "protocol-feature",
      "summary": "Use protocol retry and offset recovery before classifying terminal failure."
    },
    {
      "kind": "managed-primitive",
      "primitive": "publish-upload-state",
      "summary": "Expose pending, running, succeeded, and failed state snapshots."
    },
    {
      "kind": "managed-primitive",
      "primitive": "cleanup-managed-upload",
      "summary": "Remove owned sources and terminal state according to cleanup policy."
    }
  ],
  "layer": "feature-over-protocol",
  "primitives": [
    "accept-upload-submission",
    "make-source-durable",
    "schedule-upload-work",
    "run-protocol-upload",
    "apply-managed-retry-policy",
    "classify-failure",
    "publish-upload-state",
    "cleanup-managed-upload"
  ],
  "protocolPrimitives": [
    "store-resume-url",
    "resume-from-previous-upload",
    "recover-offset-after-error",
    "retry-with-backoff",
    "emit-progress",
    "emit-chunk-complete",
    "terminate-upload"
  ],
  "runtimeProfiles": [
    {
      "networkConstraints": [
        "any-network",
        "unmetered-network"
      ],
      "runtime": "android",
      "scheduler": "durable-os-scheduler",
      "sourceDurability": [
        "copy-to-owned-storage",
        "reference-original-source"
      ],
      "stateBackend": "platform-key-value-store"
    },
    {
      "networkConstraints": [
        "any-network",
        "unmetered-network"
      ],
      "runtime": "ios",
      "scheduler": "durable-os-scheduler",
      "sourceDurability": [
        "copy-to-owned-storage",
        "reference-original-source"
      ],
      "stateBackend": "platform-key-value-store"
    },
    {
      "networkConstraints": [
        "any-network"
      ],
      "runtime": "browser",
      "scheduler": "foreground-task",
      "sourceDurability": [
        "reference-original-source",
        "memory-only"
      ],
      "stateBackend": "web-storage"
    },
    {
      "networkConstraints": [
        "any-network"
      ],
      "runtime": "java",
      "scheduler": "process-lifetime-worker-pool",
      "sourceDurability": [
        "copy-to-owned-storage",
        "reference-original-source"
      ],
      "stateBackend": "filesystem"
    },
    {
      "networkConstraints": [
        "any-network"
      ],
      "runtime": "node",
      "scheduler": "process-lifetime-worker-pool",
      "sourceDurability": [
        "copy-to-owned-storage",
        "reference-original-source",
        "memory-only"
      ],
      "stateBackend": "filesystem"
    },
    {
      "networkConstraints": [
        "any-network"
      ],
      "runtime": "react-native",
      "scheduler": "foreground-task",
      "sourceDurability": [
        "reference-original-source",
        "memory-only"
      ],
      "stateBackend": "platform-key-value-store"
    }
  ],
  "scenarios": [
    {
      "proofs": [
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "afterAcceptedOffset": 7,
                "kind": "io-error",
                "phase": "after-accepted-offset"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {
                      "Location": "https://tus.io/uploads/managed-durable-retry"
                    },
                    "statusCode": 201
                  },
                  "url": "endpoint"
                },
                {
                  "bodySize": 7,
                  "headers": {
                    "Upload-Offset": "0"
                  },
                  "operationId": "patchTusUpload",
                  "response": {
                    "headers": {
                      "Upload-Offset": "7"
                    },
                    "statusCode": 204
                  },
                  "url": "upload"
                }
              ],
              "stateAfterAttempt": "failed"
            },
            {
              "attemptIndex": 1,
              "requests": [
                {
                  "headers": {},
                  "operationId": "getTusUploadOffset",
                  "response": {
                    "headers": {
                      "Upload-Length": "14",
                      "Upload-Offset": "7"
                    },
                    "statusCode": 200
                  },
                  "url": "upload"
                },
                {
                  "bodySize": 7,
                  "headers": {
                    "Upload-Offset": "7"
                  },
                  "operationId": "patchTusUpload",
                  "response": {
                    "headers": {
                      "Upload-Offset": "14"
                    },
                    "statusCode": 204
                  },
                  "url": "upload"
                }
              ],
              "stateAfterAttempt": "succeeded"
            }
          ],
          "cleanup": {
            "ownedSource": "remove-owned-source-after-success",
            "resumeUrl": "remove-after-success"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello managed!",
            "fingerprint": "managed-durable-retry-fingerprint",
            "metadata": {
              "filename": "managed.txt"
            },
            "uploadPath": "managed-durable-retry"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "kind": "terminal",
            "state": "succeeded"
          },
          "retryDelays": [
            0
          ],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed",
            "running",
            "succeeded"
          ],
          "runtime": "java",
          "scheduler": "process-lifetime-worker-pool",
          "stateBackend": "filesystem"
        },
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "afterAcceptedOffset": 7,
                "kind": "io-error",
                "phase": "after-accepted-offset"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {
                      "Location": "https://tus.io/uploads/managed-durable-retry"
                    },
                    "statusCode": 201
                  },
                  "url": "endpoint"
                },
                {
                  "bodySize": 7,
                  "headers": {
                    "Upload-Offset": "0"
                  },
                  "operationId": "patchTusUpload",
                  "response": {
                    "headers": {
                      "Upload-Offset": "7"
                    },
                    "statusCode": 204
                  },
                  "url": "upload"
                }
              ],
              "stateAfterAttempt": "failed"
            },
            {
              "attemptIndex": 1,
              "requests": [
                {
                  "headers": {},
                  "operationId": "getTusUploadOffset",
                  "response": {
                    "headers": {
                      "Upload-Length": "14",
                      "Upload-Offset": "7"
                    },
                    "statusCode": 200
                  },
                  "url": "upload"
                },
                {
                  "bodySize": 7,
                  "headers": {
                    "Upload-Offset": "7"
                  },
                  "operationId": "patchTusUpload",
                  "response": {
                    "headers": {
                      "Upload-Offset": "14"
                    },
                    "statusCode": 204
                  },
                  "url": "upload"
                }
              ],
              "stateAfterAttempt": "succeeded"
            }
          ],
          "cleanup": {
            "ownedSource": "remove-owned-source-after-success",
            "resumeUrl": "remove-after-success"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello managed!",
            "fingerprint": "managed-durable-retry-fingerprint",
            "metadata": {
              "filename": "managed.txt"
            },
            "uploadPath": "managed-durable-retry"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "kind": "terminal",
            "state": "succeeded"
          },
          "retryDelays": [
            0
          ],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed",
            "running",
            "succeeded"
          ],
          "runtime": "android",
          "scheduler": "durable-os-scheduler",
          "stateBackend": "platform-key-value-store"
        }
      ],
      "requiredPrimitives": [
        "accept-upload-submission",
        "make-source-durable",
        "schedule-upload-work",
        "run-protocol-upload",
        "apply-managed-retry-policy",
        "publish-upload-state",
        "cleanup-managed-upload"
      ],
      "scenarioId": "managedUploadDurableRetry",
      "summary": "Submit a durable source, survive scheduler/process interruption, resume by stored upload URL, and finish with cleanup."
    },
    {
      "proofs": [
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "kind": "unretryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 400
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            }
          ],
          "cleanup": {
            "ownedSource": "retain-owned-source-after-permanent-failure",
            "resumeUrl": "absent-after-permanent-failure"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello failure!",
            "fingerprint": "managed-permanent-failure-fingerprint",
            "metadata": {
              "filename": "managed-permanent-failure.txt"
            },
            "uploadPath": "managed-permanent-failure"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "failure": "unretryable-protocol-error",
            "kind": "terminal",
            "state": "failed"
          },
          "retryDelays": [],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed"
          ],
          "runtime": "java",
          "scheduler": "process-lifetime-worker-pool",
          "stateBackend": "filesystem"
        },
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "kind": "unretryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 400
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            }
          ],
          "cleanup": {
            "ownedSource": "retain-owned-source-after-permanent-failure",
            "resumeUrl": "absent-after-permanent-failure"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello failure!",
            "fingerprint": "managed-permanent-failure-fingerprint",
            "metadata": {
              "filename": "managed-permanent-failure.txt"
            },
            "uploadPath": "managed-permanent-failure"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "failure": "unretryable-protocol-error",
            "kind": "terminal",
            "state": "failed"
          },
          "retryDelays": [],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed"
          ],
          "runtime": "android",
          "scheduler": "durable-os-scheduler",
          "stateBackend": "platform-key-value-store"
        }
      ],
      "requiredPrimitives": [
        "accept-upload-submission",
        "make-source-durable",
        "schedule-upload-work",
        "run-protocol-upload",
        "classify-failure",
        "publish-upload-state",
        "cleanup-managed-upload"
      ],
      "scenarioId": "managedUploadPermanentFailure",
      "summary": "Classify unretryable protocol failures as terminal without further retry."
    },
    {
      "proofs": [
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "kind": "retryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 500
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            },
            {
              "attemptIndex": 1,
              "failure": {
                "kind": "retryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 500
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            },
            {
              "attemptIndex": 2,
              "failure": {
                "kind": "retryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 500
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            }
          ],
          "cleanup": {
            "ownedSource": "retain-owned-source-after-permanent-failure",
            "resumeUrl": "absent-after-permanent-failure"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello retries!",
            "fingerprint": "managed-retry-exhausted-fingerprint",
            "metadata": {
              "filename": "managed-retry-exhausted.txt"
            },
            "uploadPath": "managed-retry-exhausted"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "failure": "retry-policy-exhausted",
            "kind": "terminal",
            "state": "failed"
          },
          "retryDelays": [
            0,
            0
          ],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed",
            "running",
            "failed",
            "running",
            "failed"
          ],
          "runtime": "java",
          "scheduler": "process-lifetime-worker-pool",
          "stateBackend": "filesystem"
        },
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "kind": "retryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 500
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            },
            {
              "attemptIndex": 1,
              "failure": {
                "kind": "retryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 500
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            },
            {
              "attemptIndex": 2,
              "failure": {
                "kind": "retryable-protocol-error",
                "phase": "during-protocol-request"
              },
              "requests": [
                {
                  "bodySize": 0,
                  "headers": {
                    "Upload-Length": "14"
                  },
                  "operationId": "createTusUpload",
                  "response": {
                    "headers": {},
                    "statusCode": 500
                  },
                  "url": "endpoint"
                }
              ],
              "stateAfterAttempt": "failed"
            }
          ],
          "cleanup": {
            "ownedSource": "retain-owned-source-after-permanent-failure",
            "resumeUrl": "absent-after-permanent-failure"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello retries!",
            "fingerprint": "managed-retry-exhausted-fingerprint",
            "metadata": {
              "filename": "managed-retry-exhausted.txt"
            },
            "uploadPath": "managed-retry-exhausted"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "failure": "retry-policy-exhausted",
            "kind": "terminal",
            "state": "failed"
          },
          "retryDelays": [
            0,
            0
          ],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed",
            "running",
            "failed",
            "running",
            "failed"
          ],
          "runtime": "android",
          "scheduler": "durable-os-scheduler",
          "stateBackend": "platform-key-value-store"
        }
      ],
      "requiredPrimitives": [
        "accept-upload-submission",
        "make-source-durable",
        "schedule-upload-work",
        "run-protocol-upload",
        "apply-managed-retry-policy",
        "classify-failure",
        "publish-upload-state",
        "cleanup-managed-upload"
      ],
      "scenarioId": "managedUploadRetryPolicyExhausted",
      "summary": "Retry transient protocol failures up to the managed retry budget and then classify the upload as terminally failed."
    },
    {
      "proofs": [
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "kind": "source-unavailable",
                "phase": "before-protocol-request"
              },
              "requests": [],
              "stateAfterAttempt": "failed"
            }
          ],
          "cleanup": {
            "ownedSource": "absent-after-source-unavailable",
            "resumeUrl": "absent-after-permanent-failure"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello missing!",
            "fingerprint": "managed-source-unavailable-fingerprint",
            "metadata": {
              "filename": "managed-source-unavailable.txt"
            },
            "uploadPath": "managed-source-unavailable"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "failure": "source-unavailable",
            "kind": "terminal",
            "state": "failed"
          },
          "retryDelays": [],
          "sourceAvailability": "missing-before-durable-copy",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed"
          ],
          "runtime": "java",
          "scheduler": "process-lifetime-worker-pool",
          "stateBackend": "filesystem"
        },
        {
          "attempts": [
            {
              "attemptIndex": 0,
              "failure": {
                "kind": "source-unavailable",
                "phase": "before-protocol-request"
              },
              "requests": [],
              "stateAfterAttempt": "failed"
            }
          ],
          "cleanup": {
            "ownedSource": "absent-after-source-unavailable",
            "resumeUrl": "absent-after-permanent-failure"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello missing!",
            "fingerprint": "managed-source-unavailable-fingerprint",
            "metadata": {
              "filename": "managed-source-unavailable.txt"
            },
            "uploadPath": "managed-source-unavailable"
          },
          "network": {
            "current": "unmetered-network",
            "decision": "start-upload-work",
            "required": "any-network"
          },
          "outcome": {
            "failure": "source-unavailable",
            "kind": "terminal",
            "state": "failed"
          },
          "retryDelays": [],
          "sourceAvailability": "missing-before-durable-copy",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending",
            "running",
            "failed"
          ],
          "runtime": "android",
          "scheduler": "durable-os-scheduler",
          "stateBackend": "platform-key-value-store"
        }
      ],
      "requiredPrimitives": [
        "accept-upload-submission",
        "make-source-durable",
        "schedule-upload-work",
        "classify-failure",
        "publish-upload-state",
        "cleanup-managed-upload"
      ],
      "scenarioId": "managedUploadSourceUnavailable",
      "summary": "Classify source disappearance before protocol requests as terminal without issuing a TUS request."
    },
    {
      "proofs": [
        {
          "attempts": [],
          "cleanup": {
            "ownedSource": "retain-owned-source-while-deferred",
            "resumeUrl": "absent-while-deferred"
          },
          "input": {
            "chunkSize": 7,
            "content": "hello later!",
            "fingerprint": "managed-network-constraint-fingerprint",
            "metadata": {
              "filename": "managed-network-constraint.txt"
            },
            "uploadPath": "managed-network-constraint"
          },
          "network": {
            "current": "metered-network",
            "decision": "defer-until-network-constraint-satisfied",
            "required": "unmetered-network"
          },
          "outcome": {
            "kind": "deferred",
            "reason": "network-constraint-unsatisfied",
            "state": "pending"
          },
          "retryDelays": [],
          "sourceAvailability": "available",
          "sourceDurability": "copy-to-owned-storage",
          "states": [
            "pending"
          ],
          "runtime": "android",
          "scheduler": "durable-os-scheduler",
          "stateBackend": "platform-key-value-store"
        }
      ],
      "requiredPrimitives": [
        "accept-upload-submission",
        "make-source-durable",
        "schedule-upload-work",
        "publish-upload-state"
      ],
      "scenarioId": "managedUploadNetworkConstraint",
      "summary": "Honor network constraints before starting or resuming upload work."
    }
  ]
}
`

var generatedTusManagedUploadProofCases = []generatedTusManagedUploadProofCase{
	{
		FeatureID:          "managedUpload",
		Layer:              "feature-over-protocol",
		ScenarioID:         "managedUploadDurableRetry",
		RequiredPrimitives: []string{"accept-upload-submission", "make-source-durable", "schedule-upload-work", "run-protocol-upload", "apply-managed-retry-policy", "publish-upload-state", "cleanup-managed-upload"},
		ProtocolFeatureIDs: []string{"singleUploadLifecycle", "retryOffsetRecovery"},
		RuntimeProfiles:    []string{"android", "ios", "browser", "java", "node", "react-native"},
	},
	{
		FeatureID:          "managedUpload",
		Layer:              "feature-over-protocol",
		ScenarioID:         "managedUploadPermanentFailure",
		RequiredPrimitives: []string{"accept-upload-submission", "make-source-durable", "schedule-upload-work", "run-protocol-upload", "classify-failure", "publish-upload-state", "cleanup-managed-upload"},
		ProtocolFeatureIDs: []string{"singleUploadLifecycle", "retryOffsetRecovery"},
		RuntimeProfiles:    []string{"android", "ios", "browser", "java", "node", "react-native"},
	},
	{
		FeatureID:          "managedUpload",
		Layer:              "feature-over-protocol",
		ScenarioID:         "managedUploadRetryPolicyExhausted",
		RequiredPrimitives: []string{"accept-upload-submission", "make-source-durable", "schedule-upload-work", "run-protocol-upload", "apply-managed-retry-policy", "classify-failure", "publish-upload-state", "cleanup-managed-upload"},
		ProtocolFeatureIDs: []string{"singleUploadLifecycle", "retryOffsetRecovery"},
		RuntimeProfiles:    []string{"android", "ios", "browser", "java", "node", "react-native"},
	},
	{
		FeatureID:          "managedUpload",
		Layer:              "feature-over-protocol",
		ScenarioID:         "managedUploadSourceUnavailable",
		RequiredPrimitives: []string{"accept-upload-submission", "make-source-durable", "schedule-upload-work", "classify-failure", "publish-upload-state", "cleanup-managed-upload"},
		ProtocolFeatureIDs: []string{"singleUploadLifecycle", "retryOffsetRecovery"},
		RuntimeProfiles:    []string{"android", "ios", "browser", "java", "node", "react-native"},
	},
	{
		FeatureID:          "managedUpload",
		Layer:              "feature-over-protocol",
		ScenarioID:         "managedUploadNetworkConstraint",
		RequiredPrimitives: []string{"accept-upload-submission", "make-source-durable", "schedule-upload-work", "publish-upload-state"},
		ProtocolFeatureIDs: []string{"singleUploadLifecycle", "retryOffsetRecovery"},
		RuntimeProfiles:    []string{"android", "ios", "browser", "java", "node", "react-native"},
	},
}

var generatedTusClientFlow = generatedTusClientFlowContract{
	UrlStorage: generatedTusClientUrlStoragePolicy{
		ID: generatedTusClientUrlStorageIDPolicy{
			Multiplier: 1000000000000,
			Strategy:   "rounded-random-number",
		},
		Namespace: "tus",
		Separator: "::",
	},
}

var generatedTusClientUrlStorageConformanceScenarios = []generatedTusClientUrlStorageConformanceScenario{
	{
		Actions: []generatedTusClientUrlStorageConformanceAction{
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "",
				KeyRef:            "",
				Kind:              "assert-empty",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "tus::contract-storage-a::",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-a",
				KeyRef:            "a1",
				Kind:              "add-upload",
				Upload: map[string]any{
					"id": 1.0,
					"metadata": map[string]any{
						"filename": "a1.txt",
					},
					"size":      11.0,
					"uploadUrl": "https://tus.io/uploads/storage-a1",
				},
			},
			{
				ExpectedKeyPrefix: "tus::contract-storage-a::",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-a",
				KeyRef:            "a2",
				Kind:              "add-upload",
				Upload: map[string]any{
					"id": 2.0,
					"metadata": map[string]any{
						"filename": "a2.txt",
					},
					"size":      12.0,
					"uploadUrl": "https://tus.io/uploads/storage-a2",
				},
			},
			{
				ExpectedKeyPrefix: "tus::contract-storage-b::",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-b",
				KeyRef:            "b1",
				Kind:              "add-upload",
				Upload: map[string]any{
					"id": 3.0,
					"metadata": map[string]any{
						"filename": "b1.txt",
					},
					"size":      13.0,
					"uploadUrl": "https://tus.io/uploads/storage-b1",
				},
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"a1", "a2"},
				Fingerprint:       "contract-storage-a",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"b1"},
				Fingerprint:       "contract-storage-b",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"a1", "a2", "b1"},
				Fingerprint:       "",
				KeyRef:            "",
				Kind:              "find-all",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "",
				KeyRef:            "a2",
				Kind:              "remove-upload",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "",
				KeyRef:            "b1",
				Kind:              "remove-upload",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"a1"},
				Fingerprint:       "contract-storage-a",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-b",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
		},
		Backend:    "web-storage",
		FeatureID:  "urlStorageBackends",
		Runtimes:   []string{"browser"},
		ScenarioID: "webStorageUrlStorageBackend",
	},
	{
		Actions: []generatedTusClientUrlStorageConformanceAction{
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "",
				KeyRef:            "",
				Kind:              "assert-empty",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "tus::contract-storage-a::",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-a",
				KeyRef:            "a1",
				Kind:              "add-upload",
				Upload: map[string]any{
					"id": 1.0,
					"metadata": map[string]any{
						"filename": "a1.txt",
					},
					"size":      11.0,
					"uploadUrl": "https://tus.io/uploads/storage-a1",
				},
			},
			{
				ExpectedKeyPrefix: "tus::contract-storage-a::",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-a",
				KeyRef:            "a2",
				Kind:              "add-upload",
				Upload: map[string]any{
					"id": 2.0,
					"metadata": map[string]any{
						"filename": "a2.txt",
					},
					"size":      12.0,
					"uploadUrl": "https://tus.io/uploads/storage-a2",
				},
			},
			{
				ExpectedKeyPrefix: "tus::contract-storage-b::",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-b",
				KeyRef:            "b1",
				Kind:              "add-upload",
				Upload: map[string]any{
					"id": 3.0,
					"metadata": map[string]any{
						"filename": "b1.txt",
					},
					"size":      13.0,
					"uploadUrl": "https://tus.io/uploads/storage-b1",
				},
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"a1", "a2"},
				Fingerprint:       "contract-storage-a",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"b1"},
				Fingerprint:       "contract-storage-b",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"a1", "a2", "b1"},
				Fingerprint:       "",
				KeyRef:            "",
				Kind:              "find-all",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "",
				KeyRef:            "a2",
				Kind:              "remove-upload",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "",
				KeyRef:            "b1",
				Kind:              "remove-upload",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   []string{"a1"},
				Fingerprint:       "contract-storage-a",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
			{
				ExpectedKeyPrefix: "",
				ExpectedKeyRefs:   nil,
				Fingerprint:       "contract-storage-b",
				KeyRef:            "",
				Kind:              "find-by-fingerprint",
				Upload:            nil,
			},
		},
		Backend:    "file-storage",
		FeatureID:  "urlStorageBackends",
		Runtimes:   []string{"deno", "node"},
		ScenarioID: "fileUrlStorageBackend",
	},
}

type generatedTusTestingT interface {
	Fatalf(format string, args ...any)
	Helper()
}

func generatedTusAssertEvents(
	t generatedTusTestingT,
	scenarioID string,
	matching string,
	allowedExtraPrefixes []string,
	expected []string,
	actual []string,
) {
	t.Helper()

	if matching == "exact" {
		if generatedTusStringSlicesEqual(expected, actual) {
			return
		}
		t.Fatalf("expected %s events %#v, got %#v", scenarioID, expected, actual)
	}

	if matching != "exact-except-allowed-extra-events" {
		t.Fatalf("unsupported generated event policy %s for %s", matching, scenarioID)
	}

	expectedIndex := 0
	for _, event := range actual {
		if expectedIndex < len(expected) && event == expected[expectedIndex] {
			expectedIndex += 1
			continue
		}
		if generatedTusHasAllowedExtraEventPrefix(event, allowedExtraPrefixes) {
			continue
		}
		t.Fatalf(
			"%s emitted unexpected extra event %s; allowed prefixes %#v; expected %#v, got %#v",
			scenarioID,
			event,
			allowedExtraPrefixes,
			expected,
			actual,
		)
	}
	if expectedIndex == len(expected) {
		return
	}
	t.Fatalf(
		"%s did not emit every expected non-extra event; expected %#v, got %#v",
		scenarioID,
		expected,
		actual,
	)
}

func TestGeneratedTusManagedUploadProofCases(t *testing.T) {
	if len(generatedTusManagedUploadProofCases) == 0 {
		t.Fatal("expected generated managed upload proof cases")
	}

	for _, testCase := range generatedTusManagedUploadProofCases {
		if testCase.FeatureID != "managedUpload" {
			t.Fatalf("expected managed upload feature ID, got %s", testCase.FeatureID)
		}
		if testCase.Layer != "feature-over-protocol" {
			t.Fatalf("expected managed upload feature-over-protocol layer, got %s", testCase.Layer)
		}
		if len(testCase.RequiredPrimitives) == 0 {
			t.Fatalf("expected %s required primitives", testCase.ScenarioID)
		}
		if len(testCase.RuntimeProfiles) == 0 {
			t.Fatalf("expected %s runtime profiles", testCase.ScenarioID)
		}
		for _, featureID := range testCase.ProtocolFeatureIDs {
			if generatedTusFindClientFeature(featureID) == nil {
				t.Fatalf(
					"managed upload proof case %s references missing feature %s",
					testCase.ScenarioID,
					featureID,
				)
			}
		}
	}
}

func generatedTusFindClientFeature(featureID string) *generatedTusClientFeature {
	for index := range generatedTusClientFeatures {
		if generatedTusClientFeatures[index].FeatureID == featureID {
			return &generatedTusClientFeatures[index]
		}
	}

	return nil
}

func generatedTusHasAllowedExtraEventPrefix(event string, allowedExtraPrefixes []string) bool {
	for _, prefix := range allowedExtraPrefixes {
		if len(event) >= len(prefix) && event[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

const generatedTusEventKeyPartSeparator = ":"

func generatedTusEventKey(parts ...string) string {
	return strings.Join(parts, generatedTusEventKeyPartSeparator)
}

func generatedTusEventKeyBool(value bool) string {
	if value {
		return "true"
	}

	return "false"
}

func generatedTusEventKeyNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}

func generatedTusEventKeyAfterResponse(requestIndex string) string {
	return generatedTusEventKey("after-response", requestIndex)
}

func generatedTusEventKeyBeforeRequest(requestIndex string) string {
	return generatedTusEventKey("before-request", requestIndex)
}

func generatedTusEventKeyChunkComplete(chunkSize string, bytesAccepted string, bytesTotal string) string {
	return generatedTusEventKey("chunk-complete", chunkSize, bytesAccepted, bytesTotal)
}

func generatedTusEventKeyFingerprint(fingerprint string) string {
	return generatedTusEventKey("fingerprint", fingerprint)
}

func generatedTusEventKeyProgress(bytesSent string, bytesTotal string) string {
	return generatedTusEventKey("progress", bytesSent, bytesTotal)
}

func generatedTusEventKeyRequestAbort(requestIndex string) string {
	return generatedTusEventKey("request-abort", requestIndex)
}

func generatedTusEventKeyRetrySchedule(delay string) string {
	return generatedTusEventKey("retry-schedule", delay)
}

func generatedTusEventKeyShouldRetry(retryAttempt string, decision string) string {
	return generatedTusEventKey("should-retry", retryAttempt, decision)
}

func generatedTusEventKeySourceClose() string {
	return generatedTusEventKey("source-close")
}

func generatedTusEventKeySourceOpen(inputKind string, size string) string {
	return generatedTusEventKey("source-open", inputKind, size)
}

func generatedTusEventKeySuccess() string {
	return generatedTusEventKey("success")
}

func generatedTusEventKeyUploadUrlAvailable() string {
	return generatedTusEventKey("upload-url-available")
}

func generatedTusEventKeyUrlStorageAdd(fingerprint string, uploadUrl string) string {
	return generatedTusEventKey("url-storage-add", fingerprint, uploadUrl)
}

func generatedTusEventKeyUrlStorageFind(fingerprint string, count string) string {
	return generatedTusEventKey("url-storage-find", fingerprint, count)
}

func generatedTusEventKeyUrlStorageRemove(urlStorageKey string) string {
	return generatedTusEventKey("url-storage-remove", urlStorageKey)
}

func generatedTusStringSlicesEqual(expected []string, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	for index, expectedValue := range expected {
		if actual[index] != expectedValue {
			return false
		}
	}
	return true
}
