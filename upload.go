package tusgo

import "time"

const (
	// SizeUnknown passed as the remoteSize parameter to Client.CreateUpload, creates an upload whose size is
	// reported later, when the data transfer begins. UploadStream.SetUploadSize must be set to true before the
	// transfer starts, and the server must support the "creation-defer-length" extension.
	SizeUnknown = -1

	// OffsetUnknown is the value Upload.RemoteOffset takes while the server is still concatenating an upload.
	// Client.GetUpload sets it for a final upload created by the Client.Concatenate* methods; once the server has
	// finished the concatenation, Client.GetUpload reports the actual offset instead.
	OffsetUnknown = -1
)

// Upload represents an upload on the server.
type Upload struct {
	// Location is the location of the upload on the server, either a path or a full URL. A path is resolved
	// relative to Client.BaseURL.
	Location string

	// RemoteSize is the size of the upload on the server in bytes. SizeUnknown means that the upload has been
	// created with a deferred size, which has to be reported to the server before the first data transfer.
	RemoteSize int64

	// RemoteOffset is the offset of the upload on the server in bytes, i.e. the number of bytes the server has
	// received and committed so far.
	//
	// The TUS server keeps track of this offset, which moves forward as the data is uploaded, so that the client can
	// resume the upload later. Per the TUS protocol, every data transfer request must carry the offset it starts at,
	// and it must be equal to the server offset, otherwise the server rejects the request with 409 Conflict.
	//
	// An UploadStream uses RemoteOffset for requests and advances it as the server accepts the data. If the value
	// gets out of sync with the server, the stream returns ErrOffsetsNotSynced; call UploadStream.Sync to adopt the
	// server offset.
	//
	// OffsetUnknown value means that the server is still concatenating the upload if concatenation was requested.
	RemoteOffset int64

	// Metadata is the additional data assigned to the upload when it was created on the server.
	Metadata map[string]string

	// UploadExpired is the time the upload expires on the server, after which it is no longer available.
	// A nil value means that the upload does not expire.
	UploadExpired *time.Time

	// Partial reports whether the upload is partial, i.e. holds a portion of the data of a future final upload.
	// Partial uploads may be transferred in parallel and, once completed, are meant to be concatenated into
	// a final upload.
	Partial bool
}
