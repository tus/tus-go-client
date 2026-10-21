package tusgo

// ServerCapabilities contains the features and limits a TUS server announces on its OPTIONS endpoint.
// Use Client.FetchCapabilities to query them from a server.
type ServerCapabilities struct {
	// Extensions are the TUS protocol extensions the server advertises. Possible values are:
	//
	//   - creation -- create new uploads, see [Client.CreateUpload]
	//   - creation-defer-length -- create an upload without its size, which is declared on the first data transfer
	//   - creation-with-upload -- create an upload and transfer its data in a single request, see
	//     [Client.CreateUploadWithData]
	//   - expiration -- uploads expire after some time, see [Upload.Expires]
	//   - checksum -- verify the integrity of chunked uploads
	//   - checksum-trailer -- verify the integrity of streamed uploads, the checksum is sent in an HTTP trailer
	//   - termination -- delete uploads, see [Client.DeleteUpload]
	//   - concatenation -- upload the data in several parts in parallel and concatenate them into a single upload,
	//     see [Client.ConcatenateUploads]
	//   - concatenation-unfinished -- same as concatenation, but the parts may be still unfinished, so the server
	//     concatenates them once they are finished, see [Client.ConcatenateStreams]
	Extensions []string

	// MaxSize is the size of the largest upload the server accepts, in bytes. 0 means that the server sets no
	// such limit.
	MaxSize int64

	// ProtocolVersions are the TUS protocol versions the server supports.
	ProtocolVersions []string

	// ChecksumAlgorithms are the hash algorithms the server is able to verify the transferred data with. The server
	// must support at least the "checksum" extension for this feature. See also checksum.Algorithms for hashes
	// supported by this library.
	ChecksumAlgorithms []string
}
