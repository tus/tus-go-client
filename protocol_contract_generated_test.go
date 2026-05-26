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
	FeatureID    string
	OperationIDs []string
	Primitives   []string
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
		FeatureID:    "singleUploadLifecycle",
		OperationIDs: []string{"createTusUpload", "getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"open-input-source", "fingerprint-input", "store-resume-url", "retry-with-backoff", "emit-progress", "abort-current-request"},
	},
	{
		FeatureID:    "resumeUpload",
		OperationIDs: []string{"getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"fingerprint-input", "resume-from-previous-upload", "store-resume-url"},
	},
	{
		FeatureID:    "deferredLengthUpload",
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"defer-upload-length", "emit-progress"},
	},
	{
		FeatureID:    "creationWithUpload",
		OperationIDs: []string{"createTusUpload"},
		Primitives:   []string{"upload-during-creation", "emit-progress"},
	},
	{
		FeatureID:    "overridePatchMethod",
		OperationIDs: []string{"getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"override-patch-method"},
	},
	{
		FeatureID:    "parallelUploadConcat",
		OperationIDs: []string{"createTusUpload", "patchTusUpload"},
		Primitives:   []string{"concatenate-partial-uploads", "emit-progress"},
	},
	{
		FeatureID:    "retryOffsetRecovery",
		OperationIDs: []string{"createTusUpload", "getTusUploadOffset", "patchTusUpload"},
		Primitives:   []string{"retry-with-backoff", "recover-offset-after-error"},
	},
	{
		FeatureID:    "terminateUpload",
		OperationIDs: []string{"terminateTusUpload"},
		Primitives:   []string{"terminate-upload", "retry-with-backoff"},
	},
}
