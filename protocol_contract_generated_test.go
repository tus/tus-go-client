// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

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
	Conformance generatedTusClientFeatureConformance
	Description string
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
		Description:  "Create an upload, store its URL, upload bytes, and finish successfully.",
		FeatureID:    "singleUploadLifecycle",
		Flow:         []generatedTusClientFeatureFlowStep{
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
		Description:  "Resume a stored upload URL by discovering the remote offset before patching.",
		FeatureID:    "resumeUpload",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: []string{"deferredLengthUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Create an upload without a known length and declare the length on final PATCH.",
		FeatureID:    "deferredLengthUpload",
		Flow:         []generatedTusClientFeatureFlowStep{
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
				Summary:     "Track the source until the final chunk reveals the total size.",
			},
			{
				Kind:        "operation",
				OperationID: "patchTusUpload",
				Primitive:   "",
				Condition:   "",
				Summary:     "Declare Upload-Length on the final chunk request.",
			},
		},
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"defer-upload-length", "emit-progress"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"creationWithUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Send the first bytes on the creation request when the server/client support it.",
		FeatureID:    "creationWithUpload",
		Flow:         []generatedTusClientFeatureFlowStep{
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
		OperationIDs: []string{"createTusUpload"},
		Primitives:   []string{"upload-during-creation", "emit-progress"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"uploadBodyHeaders"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Send protocol-specific upload body headers whenever the client transmits file bytes.",
		FeatureID:    "uploadBodyHeaders",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: []string{"overridePatchMethod"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Tunnel PATCH through POST with the method-override header.",
		FeatureID:    "overridePatchMethod",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: []string{"parallelUploadConcat"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Split one input into partial uploads and concatenate their upload URLs.",
		FeatureID:    "parallelUploadConcat",
		Flow:         []generatedTusClientFeatureFlowStep{
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
		Primitives:   []string{"concatenate-partial-uploads", "emit-progress", "split-parallel-upload-boundaries"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"retryPatchAfterOffsetRecovery"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Recover from a failed chunk by reading the server offset before retrying.",
		FeatureID:    "retryOffsetRecovery",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: []string{"terminateWithRetry"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Terminate an upload resource and retry retryable termination failures.",
		FeatureID:    "terminateUpload",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Abort the active request, pending retry timer, and any partial uploads.",
		FeatureID:    "abortUpload",
		Flow:         []generatedTusClientFeatureFlowStep{
			{
				Kind:        "primitive",
				OperationID: "",
				Primitive:   "abort-current-request",
				Condition:   "",
				Summary:     "Cancel in-flight transport work without emitting user callbacks after abort.",
			},
		},
		OperationIDs: nil,
		Primitives:   []string{"abort-current-request"},
	},
	{
		Conformance: generatedTusClientFeatureConformance{
			ScenarioIDs: []string{"singleUploadLifecycle", "creationWithUpload", "resumeFromPreviousUpload"},
			Status:      "covered-by-generated-scenario",
		},
		Description:  "Expose progress and accepted-chunk callbacks from runtime upload activity.",
		FeatureID:    "uploadCallbacks",
		Flow:         []generatedTusClientFeatureFlowStep{
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
		Description:  "Run before-request, after-response, and custom retry hooks around transport.",
		FeatureID:    "requestLifecycleHooks",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Persist, find, resume, and optionally remove upload URLs by fingerprint.",
		FeatureID:    "resumeUrlStorage",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Support the reference client input/source families across runtimes.",
		FeatureID:    "inputSources",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Support browser and file-backed URL storage implementations.",
		FeatureID:    "urlStorageBackends",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Select between tus v1 and supported IETF draft client protocol modes.",
		FeatureID:    "protocolVersionSelection",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Normalize relative Location headers against the request endpoint.",
		FeatureID:    "relativeLocationResolution",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Validate option combinations before starting runtime work.",
		FeatureID:    "startOptionValidation",
		Flow:         []generatedTusClientFeatureFlowStep{
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
			ScenarioIDs: nil,
			Status:      "needs-generated-scenario",
		},
		Description:  "Attach request, response, status, body, and request ID context to errors.",
		FeatureID:    "detailedErrors",
		Flow:         []generatedTusClientFeatureFlowStep{
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
