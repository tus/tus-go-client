package tusgo_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/bdragon300/tusgo"
)

// RedisClient is an abstract Redis client, the subset of Redis commands used to store the pending chunk.
type RedisClient interface {
	// Get returns the value of key, or found=false if key does not exist.
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	// Set sets key to value, which expires after ttl.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Del deletes key. Deleting a missing key is not an error.
	Del(ctx context.Context, key string) error
}

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

// UploadViaRedis transfers the data from the unseekable src to the upload at location in chunks of chunkSize bytes.
//
// Each chunk is saved to Redis before it is sent and deleted after the server accepts it, so the data consumed from
// src survives failed requests and process restarts. If a pending chunk is found in Redis, e.g. left by an interrupted
// previous run, it is sent first. Once src is exhausted, the upload size is declared on the server, if it was not
// known when the upload was created.
//
// The chunk is stored under one key per upload. The value is the chunk offset in the upload as 8-byte big-endian
// integer followed by the chunk data, so both are written atomically by a single SET.
func UploadViaRedis(ctx context.Context, s *tusgo.Stream, location string, rdb RedisClient, src io.Reader, chunkSize int) error {
	const ttl = 24 * time.Hour // Should not be shorter than the upload lifetime on the server
	key := "tus:pending-chunk:" + location

	buf := make([]byte, chunkSize)
	for eof := false; !eof; {
		var offset int64
		var data []byte

		v, ok, err := rdb.Get(ctx, key)
		switch {
		case err != nil:
			return fmt.Errorf("load pending chunk: %w", err)
		case ok && len(v) < 8:
			return fmt.Errorf("malformed pending chunk of %d bytes", len(v))
		case ok:
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
			if err = rdb.Set(ctx, key, append(v, data...), ttl); err != nil {
				return fmt.Errorf("save chunk: %w", err)
			}
		}

		if err = sendChunk(s, offset, data); err != nil {
			return fmt.Errorf("send chunk at offset %d: %w", offset, err)
		}
		// If the deletion fails, the chunk is loaded again on the next run and skipped, because the server has it
		if err = rdb.Del(ctx, key); err != nil {
			return fmt.Errorf("delete chunk: %w", err)
		}
	}

	return nil
}

// Example demonstrates the chunked transfer from an unseekable reader (data stream) of unknown size.
// Every chunk is saved to Redis before it is sent, so it can be sent again after a failure, even though the source
// cannot be rewound.
func Example_unseekableReaderWithRedisStorage() {
	endpoint, err := url.Parse("https://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}

	// Chunk size is a trade-off: smaller chunks lose less progress on a broken connection, but need more requests
	const chunkSize = 512 * 1024
	var rdb RedisClient // Abstract Redis client

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
	if err = UploadViaRedis(context.Background(), stream, u.Location, rdb, src, chunkSize); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	if err = stream.DeclareSize(stream.Tell()); err != nil {
		log.Fatalf("Failed to declare size: %s", err)
	}
	fmt.Printf("Uploading complete. Size: %d\n", stream.Size())
}
