// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

const (
	// DefaultProtocolVersion is the wire protocol version used by default.
	DefaultProtocolVersion = "1.0.0"

	// DefaultClientProtocol is the generated client protocol mode used by default.
	DefaultClientProtocol = "tus-v1"

	ProtocolTusV1       = "tus-v1"
	ProtocolIetfDraft03 = "ietf-draft-03"
	ProtocolIetfDraft05 = "ietf-draft-05"
)

var defaultProtocolRequestHeaders = map[string]string{"Tus-Resumable": "1.0.0"}
var defaultProtocolResponseHeaders = map[string]string{"Tus-Resumable": "1.0.0"}

type clientProtocolCompatibilityVersion struct {
	RequestHeaders                map[string]string
	ResponseHeaders               map[string]string
	UploadBodyContentType         string
	UploadCompleteHeaderName      string
	UploadCompleteCompleteValue   string
	UploadCompleteIncompleteValue string
}

var clientProtocolCompatibilityVersions = map[string]clientProtocolCompatibilityVersion{
	"tus-v1": {
		RequestHeaders:        map[string]string{"Tus-Resumable": "1.0.0"},
		ResponseHeaders:       map[string]string{"Tus-Resumable": "1.0.0"},
		UploadBodyContentType: "application/offset+octet-stream",
	},
	"ietf-draft-03": {
		RequestHeaders:                map[string]string{"Upload-Draft-Interop-Version": "5"},
		ResponseHeaders:               map[string]string{},
		UploadBodyContentType:         "",
		UploadCompleteHeaderName:      "Upload-Complete",
		UploadCompleteCompleteValue:   "?1",
		UploadCompleteIncompleteValue: "?0",
	},
	"ietf-draft-05": {
		RequestHeaders:                map[string]string{"Upload-Draft-Interop-Version": "6"},
		ResponseHeaders:               map[string]string{},
		UploadBodyContentType:         "application/partial-upload",
		UploadCompleteHeaderName:      "Upload-Complete",
		UploadCompleteCompleteValue:   "?1",
		UploadCompleteIncompleteValue: "?0",
	},
}

func copyDefaultProtocolHeaders(headers map[string]string) map[string]string {
	copied := make(map[string]string, len(headers))
	for name, value := range headers {
		copied[name] = value
	}
	return copied
}

func clientProtocolCompatibilityVersionFor(protocolVersion string) (clientProtocolCompatibilityVersion, bool) {
	if protocolVersion == "" || protocolVersion == DefaultProtocolVersion {
		protocolVersion = DefaultClientProtocol
	}

	compatibilityVersion, ok := clientProtocolCompatibilityVersions[protocolVersion]
	return compatibilityVersion, ok
}

func protocolRequestHeaders(protocolVersion string) (map[string]string, bool) {
	compatibilityVersion, ok := clientProtocolCompatibilityVersionFor(protocolVersion)
	if !ok {
		return nil, false
	}

	return copyDefaultProtocolHeaders(compatibilityVersion.RequestHeaders), true
}

func protocolUploadBodyContentType(protocolVersion string) (string, bool) {
	compatibilityVersion, ok := clientProtocolCompatibilityVersionFor(protocolVersion)
	if !ok || compatibilityVersion.UploadBodyContentType == "" {
		return "", false
	}

	return compatibilityVersion.UploadBodyContentType, true
}

func protocolUploadCompleteHeader(protocolVersion string, done bool) (string, string, bool) {
	compatibilityVersion, ok := clientProtocolCompatibilityVersionFor(protocolVersion)
	if !ok || compatibilityVersion.UploadCompleteHeaderName == "" {
		return "", "", false
	}
	if done {
		return compatibilityVersion.UploadCompleteHeaderName, compatibilityVersion.UploadCompleteCompleteValue, true
	}

	return compatibilityVersion.UploadCompleteHeaderName, compatibilityVersion.UploadCompleteIncompleteValue, true
}

// DefaultProtocolRequestHeaders returns the protocol request headers used by default.
func DefaultProtocolRequestHeaders() map[string]string {
	return copyDefaultProtocolHeaders(defaultProtocolRequestHeaders)
}

// DefaultProtocolResponseHeaders returns the protocol response headers used by default.
func DefaultProtocolResponseHeaders() map[string]string {
	return copyDefaultProtocolHeaders(defaultProtocolResponseHeaders)
}
