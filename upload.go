package tusgo

import "time"

const (
	// OffsetUnknown is the value Upload.RemoteOffset takes while the server is still concatenating an upload.
	// Client.GetUpload sets it for a final upload created by the Client.Concatenate* methods; once the server has
	// finished the concatenation, Client.GetUpload reports the actual offset instead.
	OffsetUnknown = -1
)

// Upload represents an upload on the server.
type Upload struct {
	// Location is the location of the upload on the server. Can be a relative or absolute URL.
	Location string

	// RemoteSize is the size of the upload on the server in bytes. SizeUnknown means that the upload has been
	// created with a deferred size, which has to be reported to the server before the first data transfer.
	RemoteSize int64

	// RemoteOffset is the current offset on server side, where the data upload has been stopped.
	// This offset is advanced as the data is transferred to the server, so the value equal to RemoteSize means that
	// the upload is full.
	// The stream uses this value to determine where to resume the upload.
	//
	// OffsetUnknown value means that the server is still concatenating this "final" upload from "partial" uploads,
	// and offset is unknown until the server has finished the concatenation.
	RemoteOffset int64

	// Metadata is the additional data assigned to the upload when it was created on the server.
	Metadata map[string]string

	// UploadExpired is the time the upload expires on the server, after which it is no longer available.
	// A nil value means that the upload does not expire.
	UploadExpired *time.Time

	// Partial reports whether the upload is "partial". Partial uploads may be transferred in parallel and,
	// once completed, are meant to be concatenated into a "final" upload.
	Partial bool
}
