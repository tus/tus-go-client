package tusgo_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/bdragon300/tusgo/v2"
)

// withRetry calls op until it succeeds, returns a permanent error or runs out of attempts, waiting with
// an exponential backoff between attempts. If the offsets are out of sync, the stream adopts the server offset before
// the next attempt.
func withRetry(s *tusgo.Stream, op func() error) error {
	const maxAttempts = 10
	delay := time.Second
	for attempt := 1; ; attempt++ {
		err := op()
		if err == nil || !tusgo.IsErrorRetryable(err) || attempt == maxAttempts {
			return err
		}
		log.Printf("Attempt %d failed, retrying in %s: %s", attempt, delay, err)
		time.Sleep(delay)
		delay = min(delay*2, time.Minute)

		if errors.Is(err, tusgo.ErrOffsetsNotSynced) {
			// If Sync fails, the next op fails with the same error and is retried
			if _, err = s.Sync(); err != nil {
				log.Printf("Failed to sync: %s", err)
			}
		}
	}
}

// sendChunk sends the part of data, which starts at chunkOffset in the upload, that the server does not have yet.
func sendChunk(s *tusgo.Stream, chunkOffset int64, data []byte) error {
	for {
		// The server may already have some or all of the chunk, e.g. after a lost response or a partial write
		skip := s.Tell() - chunkOffset
		if skip < 0 || skip > int64(len(data)) {
			return fmt.Errorf("server offset %d is outside the stored chunk [%d, %d)", s.Tell(), chunkOffset, chunkOffset+int64(len(data)))
		}
		if skip == int64(len(data)) {
			return nil
		}
		// On ErrIncompleteWrite the stream offset is advanced by the accepted bytes, so the loop sends the rest
		if err := withRetry(s, func() error {
			_, err := s.Write(data[skip:])
			return err
		}); err != nil && !errors.Is(err, tusgo.ErrIncompleteWrite) {
			return err
		}
	}
}

// writeFileAtomic writes data to a temporary file in the same directory as name and renames it to name, so name
// contains either the previous or the new data in full, even if the process crashes in the middle of writing.
func writeFileAtomic(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // No-op after a successful rename

	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

// UploadViaTempFile transfers the data from the unseekable src to the upload at location in chunks of chunkSize bytes.
//
// Each chunk is saved to a file in dir before it is sent and deleted after the server accepts it, so the data consumed
// from src survives failed requests and process restarts. If a pending chunk file is found, e.g. left by an
// interrupted previous run, it is sent first.
//
// The chunk is stored in one file per upload, named after the hash of location. The file contains the chunk offset in
// the upload as 8-byte big-endian integer followed by the chunk data, and is replaced atomically, so both are always
// consistent.
func UploadViaTempFile(s *tusgo.Stream, location, dir string, src io.Reader, chunkSize int) error {
	sum := sha256.Sum256([]byte(location))
	name := filepath.Join(dir, "tus-pending-chunk-"+hex.EncodeToString(sum[:]))

	buf := make([]byte, chunkSize)
	for eof := false; !eof; {
		var offset int64
		var data []byte

		v, err := os.ReadFile(name)
		ok := err == nil
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return fmt.Errorf("load pending chunk: %w", err)
		case len(v) < 8:
			return fmt.Errorf("malformed pending chunk of %d bytes", len(v))
		default:
			offset, data = int64(binary.BigEndian.Uint64(v[:8])), v[8:]
		}

		if !ok {
			// No pending chunk, so read the next chunk from src. It follows the data the server already has.
			n, err := io.ReadFull(src, buf)
			switch {
			case errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF):
				eof = true
			case err != nil:
				return fmt.Errorf("read source: %w", err)
			}
			if n == 0 {
				break
			}
			offset, data = s.Tell(), buf[:n]
			v = binary.BigEndian.AppendUint64(make([]byte, 0, 8+n), uint64(offset))
			if err = writeFileAtomic(name, append(v, data...)); err != nil {
				return fmt.Errorf("save chunk: %w", err)
			}
		}

		if err = sendChunk(s, offset, data); err != nil {
			return fmt.Errorf("send chunk at offset %d: %w", offset, err)
		}
		// If the deletion fails, the chunk is loaded again on the next run and skipped, because the server has it
		if err = os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("delete chunk: %w", err)
		}
	}

	return nil
}

// Example demonstrates the chunked transfer from an unseekable reader (data stream) of unknown size.
// Every chunk is saved to a temporary file before it is sent, so it can be sent again after a failure, even though
// the source cannot be rewound.
func Example_unseekableReaderWithTempFileStorage() {
	endpoint, err := url.Parse("https://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}

	// Chunk size is a trade-off: smaller chunks lose less progress on a broken connection, but need more requests
	const chunkSize = 512 * 1024
	// To resume after a reboot, use a directory that is not cleared on boot, unlike the system temp directory may be
	dir := filepath.Join(os.TempDir(), "tusgo")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		log.Fatalf("Failed to create directory: %s", err)
	}

	cl := tusgo.NewClient(endpoint)
	var src io.Reader = os.Stdin

	// The size is unknown until src is exhausted, so it is declared later ("creation-defer-length" extension)
	u, err := cl.CreateUpload(tusgo.SizeUnknown, false, nil)
	if err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	stream, err := tusgo.NewStream(cl, u)
	if err != nil {
		log.Fatalf("Failed to create upload stream: %s", err)
	}

	if _, err = stream.Sync(); err != nil {
		log.Fatalf("Failed to sync offsets: %s", err)
	}
	if err = UploadViaTempFile(stream, u.Location, dir, src, chunkSize); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	if err = stream.DeclareSize(stream.Tell()); err != nil {
		log.Fatalf("Failed to declare size: %s", err)
	}
	fmt.Printf("Uploading complete. Size: %d\n", stream.Size())
}
