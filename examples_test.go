package tusgo_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/bdragon300/tusgo/v2"
)

// chunkSize is the size of a chunk sent in a single request.
const chunkSize = 2 * 1024 * 1024

// UploadWithRetry transfers the data from src to the upload in chunks, resuming the transfer after temporary errors.
func UploadWithRetry(s *tusgo.Stream, src io.ReadSeeker) error {
	// Set stream and file pointer to be equal to the remote pointer
	// (if we resume the upload that was interrupted earlier)
	if _, err := s.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if _, err := src.Seek(s.Tell(), io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	dst := tusgo.NewChunkedWriter(s, chunkSize)
	attempts := 10
	for {
		_, err := io.Copy(dst, src)
		switch {
		case tusgo.IsErrorRetryable(err):
			if errors.Is(err, tusgo.ErrOffsetsNotSynced) {
				if d, err := s.Sync(); err != nil {
					return fmt.Errorf("sync: %w", err)
				} else if !dst.Advance(d) {
					if _, err := src.Seek(s.Tell(), io.SeekStart); err != nil {
						return fmt.Errorf("seek: %w", err)
					}
				}
			} else {
				time.Sleep(time.Second)
			}
			attempts--
			continue
		case err != nil:
			return fmt.Errorf("read from permanent error: %w", err)
		}
		break
	}
	if attempts == 0 {
		return errors.New("too many attempts to upload the data")
	}
	return nil
}

// OpenFile opens the file at the given path and returns the file handle and its info.
func OpenFile(path string) (*os.File, os.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	finfo, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}
	return f, finfo, nil
}

// SplitFile splits the file into the given number of parts of roughly equal size. The parts can be read concurrently.
func SplitFile(file *os.File, size int64, parts int64) []*io.SectionReader {
	partSize := (size + parts - 1) / parts
	res := make([]*io.SectionReader, 0, parts)
	for off := int64(0); off < size; off += partSize {
		res = append(res, io.NewSectionReader(file, off, min(partSize, size-off)))
	}
	return res
}

func Example_minimal() {
	endpoint, _ := url.Parse("http://example.com/files")
	cl := tusgo.NewClient(endpoint)

	// Assume that the upload has already been created on the server with a size of 1 MiB
	u := tusgo.Upload{
		Location:   "foo/bar",
		RemoteSize: 1024 * 1024,
	}

	// Open the file to upload
	f, err := os.Open("/tmp/file.txt")
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer f.Close()

	s, err := tusgo.NewStream(cl, u)
	if err != nil {
		log.Fatalf("Failed to create upload stream: %s", err)
	}

	// Align the stream and file offsets with the offset reported by the server
	if _, err = s.Sync(); err != nil {
		log.Fatalf("Failed to sync upload stream: %s", err)
	}
	if _, err = f.Seek(s.Tell(), io.SeekStart); err != nil {
		log.Fatalf("Failed to seek file: %s", err)
	}

	// Resumable transfer the file in chunks of 2 MiB, one request per chunk
	w := tusgo.NewChunkedWriter(s, 2*1024*1024)
	var r int64
	for {
		_, err := io.Copy(w, f)
		switch {
		case tusgo.IsErrorRetryable(err):
			if errors.Is(err, tusgo.ErrOffsetsNotSynced) {
				if d, err := s.Sync(); err != nil {
					log.Fatalf("Failed to sync upload stream: %s", err)
				} else if !w.Advance(d) {
					if _, err := f.Seek(s.Tell(), io.SeekStart); err != nil {
						log.Fatalf("Failed to seek file: %s", err)
					}
				}
			} else {
				time.Sleep(time.Second)
			}
			continue
		case err != nil:
			log.Fatalf("Written %d bytes, permanent error: %s", r, err)
		}
		break
	}
	log.Printf("Written %d bytes\n", r)
}

// Example demonstrates the "creation" extension.
func ExampleClient_CreateUpload() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u, err := cl.CreateUpload(finfo.Size(), false, nil)
	if err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	stream, err := tusgo.NewStream(cl, u)
	if err != nil {
		log.Fatalf("Failed to create upload stream: %s", err)
	}
	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Printf("Uploading complete. Offset: %d, Size: %d\n", stream.Tell(), stream.Size())
}

// Example demonstrates the "creation-with-upload" extension. The whole data is sent within the creation request,
// which is suitable for small files.
func ExampleClient_CreateUploadWithData() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	data, err := os.ReadFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	uploaded, u, err := cl.CreateUploadWithData(data, int64(len(data)), false, map[string]string{"filename": "file.txt"})
	if err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s, uploaded %d of %d bytes\n", u.Location, uploaded, u.RemoteSize)

	// The server may accept only a part of the data
	if u.RemoteOffset < u.RemoteSize {
		log.Fatalf("Failed to upload the whole data: %d of %d bytes uploaded", u.RemoteOffset, u.RemoteSize)
	}
	fmt.Println("Uploading complete")
}

// Example demonstrates the "termination" extension. The upload is deleted from the server if the user
// interrupts the transfer by pressing Ctrl-C.
func ExampleClient_DeleteUpload() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u, err := cl.CreateUpload(finfo.Size(), false, nil)
	if err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	// Cancel the transfer on Ctrl-C. The context is attached to the requests of a separate client used by the stream
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	streamCl := tusgo.NewClient(endpoint)
	streamCl.DoRequest = func(req *http.Request) (*http.Response, error) {
		return http.DefaultClient.Do(req.WithContext(ctx))
	}

	stream, err := tusgo.NewStream(streamCl, u)
	if err != nil {
		log.Fatalf("Failed to create upload stream: %s", err)
	}
	err = UploadWithRetry(stream, f)
	switch {
	case err == nil:
		fmt.Println("Uploading complete")
	case errors.Is(err, context.Canceled):
		fmt.Println("Transfer interrupted, deleting the upload")
		// The client does not use the canceled context, so the request is sent
		if err = cl.DeleteUpload(u.Location); err != nil && !errors.Is(err, tusgo.ErrUploadDoesNotExist) {
			log.Fatalf("Failed to delete upload: %s", err)
		}
		fmt.Println("Upload deleted")
	default:
		log.Fatalf("Failed to upload: %s", err)
	}
}

// Example demonstrates the "concatenation" extension. A file is split into parts, which are transferred in parallel
// to partial uploads, and then concatenated into a final upload.
func ExampleClient_ConcatenatePartialUploads_parallel() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	// Split the file into 4 parts, each one is read independently from its own offset in the file
	parts := SplitFile(f, finfo.Size(), 4)
	locations := make([]string, len(parts))
	wg := &sync.WaitGroup{}
	wg.Add(len(parts))

	// Create and transfer partial uploads in parallel
	for ind, part := range parts {
		ind := ind
		part := part
		go func() {
			defer wg.Done()

			u, err := cl.CreateUpload(part.Size(), true, nil)
			if err != nil {
				log.Fatalf("Part #%d: failed to create upload: %s", ind, err)
			}

			fmt.Printf("Part #%d: transferring %d bytes to %s\n", ind, part.Size(), u.Location)
			stream, err := tusgo.NewStream(cl, u)
			if err != nil {
				log.Fatalf("Part #%d: failed to create upload stream: %s", ind, err)
			}
			if err := UploadWithRetry(stream, part); err != nil {
				log.Fatalf("Part #%d: failed to upload: %s", ind, err)
			}
			locations[ind] = u.Location
		}()
	}

	wg.Wait()
	fmt.Println("Uploading complete, starting concatenation...")

	// Concatenate partial uploads into a final upload
	final, err := cl.ConcatenatePartialUploads(locations, nil)
	if err != nil {
		log.Fatalf("Failed to concatenate uploads: %s", err)
	}
	fmt.Printf("Final upload location: %s\n", final)
}

// Example demonstrates the "concatenation-unfinished" extension. A file is split into parts, and the final upload is
// requested before the parts are transferred. The server concatenates the partial uploads once all of them are
// finished.
func ExampleClient_ConcatenatePartialUploads_unfinished() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	// Split the file into consecutive parts
	parts := SplitFile(f, finfo.Size(), 4)
	streams := make([]*tusgo.Stream, len(parts))
	locations := make([]string, len(parts))
	for ind, part := range parts {
		u, err := cl.CreateUpload(part.Size(), true, nil)
		if err != nil {
			log.Fatalf("Part #%d: failed to create upload: %s", ind, err)
		}
		fmt.Printf("Part #%d: location %s\n", ind, u.Location)
		if streams[ind], err = tusgo.NewStream(cl, u); err != nil {
			log.Fatalf("Part #%d: failed to create upload stream: %s", ind, err)
		}
		locations[ind] = u.Location
	}

	// Asking the server to concatenate the partial uploads into a final upload once they are finished.
	final, err := cl.ConcatenatePartialUploads(locations, nil)
	if err != nil {
		log.Fatalf("Failed to concatenate uploads: %s", err)
	}
	fmt.Printf("Final upload location: %s\n", final)

	// Transfer the parts in parallel
	wg := &sync.WaitGroup{}
	wg.Add(len(streams))
	for ind := range streams {
		ind := ind
		go func() {
			defer wg.Done()
			if err := UploadWithRetry(streams[ind], parts[ind]); err != nil {
				log.Fatalf("Part #%d: failed to upload: %s", ind, err)
			}
		}()
	}
	wg.Wait()
	fmt.Println("Uploading complete")
}

// Example demonstrates the "expiration" extension. The upload created in a previous run is resumed if it has not
// expired yet, otherwise a new upload is created.
func Example_expiration() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u, err := cl.CreateUpload(finfo.Size(), false, nil)
	if err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}

	if u.Expires != nil {
		fmt.Printf("Upload expires at %s\n", u.Expires.Format(time.RFC1123))
	} else {
		fmt.Printf("Upload does not expire\n")
	}
}

// rateLimitedBody limits the speed at which the HTTP transport reads the request body, i.e. the upload speed.
type rateLimitedBody struct {
	io.ReadCloser
	bytesPerSecond int
}

func (b *rateLimitedBody) Read(p []byte) (int, error) {
	// Read small portions, 0.1 second worth of data at most, to keep the speed smooth
	if maxLen := b.bytesPerSecond / 10; len(p) > maxLen {
		p = p[:maxLen]
	}
	n, err := b.ReadCloser.Read(p)
	// Sleep for the time that n bytes take at the given speed. The time spent on reading and sending is not taken
	// into account, so the actual speed is slightly lower than the limit.
	time.Sleep(time.Duration(n) * time.Second / time.Duration(b.bytesPerSecond))
	return n, err
}

// Example demonstrates the data transfer with speed limit in one goroutine. The speed is shaped in
// [tusgo.Client.DoRequest] by throttling the request body, so it applies to every request the client sends,
// without wrapping the source reader.
func Example_speedLimit() {
	endpoint, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(endpoint)

	// Limit upload speed to 512 KiB/s
	const bytesPerSecond = 512 * 1024
	cl.DoRequest = func(req *http.Request) (*http.Response, error) {
		// Leave requests without a body untouched: replacing http.NoBody would make the transport use chunked encoding
		if req.Body != nil && req.Body != http.NoBody {
			req.Body = &rateLimitedBody{ReadCloser: req.Body, bytesPerSecond: bytesPerSecond}
		}
		return http.DefaultClient.Do(req)
	}

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()
	u, err := cl.CreateUpload(finfo.Size(), false, nil)
	if err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}

	stream, err := tusgo.NewStream(cl, u)
	if err != nil {
		log.Fatalf("Failed to create upload stream: %s", err)
	}

	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Printf("Uploading complete. Offset: %d, Size: %d\n", stream.Tell(), stream.Size())
}
