package tusgo

// ServerCapabilities contains the features and limits a TUS server announces on its OPTIONS endpoint.
// Use Client.UpdateCapabilities to query them from a server.
type ServerCapabilities struct {
	// Extensions are the TUS protocol extensions the server supports, such as creation, creation-defer-length,
	// creation-with-upload, termination, concatenation, concatenation-unfinished, checksum, checksum-trailer and
	// expiration. For the full list see the TUS protocol description.
	Extensions []string

	// MaxSize is the size of the largest upload the server accepts, in bytes. 0 means that the server sets no
	// such limit.
	MaxSize int64

	// ProtocolVersions are the TUS protocol versions the server supports. A client must pick one of them by
	// setting Client.ProtocolVersion.
	ProtocolVersions []string

	// ChecksumAlgorithms are the hash algorithms the server is able to verify the transferred data with. The server
	// must support at least the "checksum" extension for this feature. See also checksum.Algorithms for the hashes
	// tusgo is able to calculate.
	ChecksumAlgorithms []string
}
