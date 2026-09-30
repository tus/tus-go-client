package checksum

import (
	"encoding/base64"
	"hash"
	"io"
	"strings"
)

// HashBase64ReadWriter wraps a hash.Hash so that the data written to it can be read back as the prefix followed by
// the hash sum encoded in base64. The sum is calculated on the first read, therefore the whole data must be written
// before the reading begins.
type HashBase64ReadWriter struct {
	hash.Hash
	rd     io.Reader
	prefix string
}

// NewHashBase64ReadWriter returns a new HashBase64ReadWriter that wraps the hash h and prepends prefix to the
// encoded hash sum.
func NewHashBase64ReadWriter(h hash.Hash, prefix string) *HashBase64ReadWriter {
	return &HashBase64ReadWriter{Hash: h, prefix: prefix}
}

// Read reads up to len(p) bytes of the prefixed base64 hash sum into p, calculating the sum on the first call.
// It returns the number of bytes read (0 <= n <= len(p)) and any error encountered, which is io.EOF once the whole
// result has been read.
func (h *HashBase64ReadWriter) Read(p []byte) (n int, err error) {
	if h.rd == nil {
		sum := h.Hash.Sum(make([]byte, 0))
		s := h.prefix + base64.StdEncoding.EncodeToString(sum)
		h.rd = strings.NewReader(s)
	}
	return h.rd.Read(p)
}
