// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

const (
	// DefaultProtocolVersion is the wire protocol version used by default.
	DefaultProtocolVersion = "1.0.0"
)

var defaultProtocolRequestHeaders = map[string]string{"Tus-Resumable": "1.0.0"}
var defaultProtocolResponseHeaders = map[string]string{"Tus-Resumable": "1.0.0"}

func copyDefaultProtocolHeaders(headers map[string]string) map[string]string {
	copied := make(map[string]string, len(headers))
	for name, value := range headers {
		copied[name] = value
	}
	return copied
}

// DefaultProtocolRequestHeaders returns the protocol request headers used by default.
func DefaultProtocolRequestHeaders() map[string]string {
	return copyDefaultProtocolHeaders(defaultProtocolRequestHeaders)
}

// DefaultProtocolResponseHeaders returns the protocol response headers used by default.
func DefaultProtocolResponseHeaders() map[string]string {
	return copyDefaultProtocolHeaders(defaultProtocolResponseHeaders)
}
