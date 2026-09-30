package tusgo_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/bdragon300/tusgo"
)

// UploadWithRetry transfers the data from src to dst, resuming the transfer after temporary errors.
func UploadWithRetry(dst *tusgo.UploadStream, src io.ReadSeeker) error {
	// Set stream and file pointer to be equal to the remote pointer
	// (if we resume the upload that was interrupted earlier)
	if _, err := dst.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	if _, err := src.Seek(dst.Tell(), io.SeekStart); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	_, err := io.Copy(dst, src)
	attempts := 10
	for err != nil && attempts > 0 {
		if _, ok := err.(net.Error); !ok && !errors.Is(err, tusgo.ErrChecksumMismatch) {
			return fmt.Errorf("permanent error: %w", err) // Permanent error, no luck
		}
		time.Sleep(5 * time.Second)
		attempts--
		_, err = io.Copy(dst, src)
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
	baseURL, _ := url.Parse("http://example.com/files")
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	// Assume that the upload has already been created on the server with a size of 1 MiB
	u := tusgo.Upload{
		Location:   "http://example.com/files/foo/bar",
		RemoteSize: 1024 * 1024,
	}

	// Open the file to upload
	f, err := os.Open("/tmp/file.txt")
	if err != nil {
		log.Fatalf("Failed to open file: %s", err)
	}
	defer f.Close()

	s := tusgo.NewUploadStream(cl, &u)

	// Align the stream and file offsets with the offset reported by the server
	if _, err = s.Sync(); err != nil {
		log.Fatalf("Failed to sync upload stream: %s", err)
	}
	if _, err = f.Seek(s.Tell(), io.SeekStart); err != nil {
		log.Fatalf("Failed to seek file: %s", err)
	}

	written, err := io.Copy(s, f)
	if err != nil {
		log.Fatalf("Written %d bytes, error: %s, last response: %v", written, err, s.LastResponse)
	}
	log.Printf("Written %d bytes\n", written)
}

// Example demonstrates the "creation" extension.
func ExampleClient_CreateUpload() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	stream := tusgo.NewUploadStream(cl, &u)
	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Printf("Uploading complete. Offset: %d, Size: %d\n", u.RemoteOffset, u.RemoteSize)
}

// Example demonstrates the "creation-with-upload" extension. The whole data is sent within the creation request,
// which is suitable for small files.
func ExampleClient_CreateUploadWithData() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	data, err := os.ReadFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("Failed to read file: %s", err)
	}

	u := tusgo.Upload{}
	uploaded, _, err := cl.CreateUploadWithData(&u, data, int64(len(data)), false, map[string]string{"filename": "file.txt"})
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
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	// Cancel the transfer on Ctrl-C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	stream := tusgo.NewUploadStream(cl, &u).WithContext(ctx)
	err = UploadWithRetry(stream, f)
	switch {
	case err == nil:
		fmt.Println("Uploading complete")
	case errors.Is(err, context.Canceled):
		fmt.Println("Transfer interrupted, deleting the upload")
		// The client does not use the canceled context, so the request is sent
		if _, err = cl.DeleteUpload(u); err != nil && !errors.Is(err, tusgo.ErrUploadDoesNotExist) {
			log.Fatalf("Failed to delete upload: %s", err)
		}
		fmt.Println("Upload deleted")
	default:
		log.Fatalf("Failed to upload: %s", err)
	}
}

// Example demonstrates the "concatenation" extension. A file is split into parts, which are transferred in parallel
// to partial uploads, and then concatenated into a final upload.
func ExampleClient_ConcatenateUploads_parallel() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	// Split the file into 4 parts, each one is read independently from its own offset in the file
	parts := SplitFile(f, finfo.Size(), 4)
	uploads := make([]tusgo.Upload, len(parts))
	wg := &sync.WaitGroup{}
	wg.Add(len(parts))

	// Create and transfer partial uploads in parallel
	for ind, part := range parts {
		ind := ind
		part := part
		go func() {
			defer wg.Done()

			u := tusgo.Upload{}
			if _, err := cl.CreateUpload(&u, part.Size(), true, nil); err != nil {
				log.Fatalf("Part #%d: failed to create upload: %s", ind, err)
			}

			fmt.Printf("Part #%d: transferring %d bytes to %s\n", ind, part.Size(), u.Location)
			stream := tusgo.NewUploadStream(cl, &u)
			if err := UploadWithRetry(stream, part); err != nil {
				log.Fatalf("Part #%d: failed to upload: %s", ind, err)
			}
			uploads[ind] = u
		}()
	}

	wg.Wait()
	fmt.Println("Uploading complete, starting concatenation...")

	// Concatenate partial uploads into a final upload
	final := tusgo.Upload{}
	if _, err = cl.ConcatenateUploads(&final, uploads, nil); err != nil {
		log.Fatalf("Failed to concatenate uploads: %s", err)
	}
	fmt.Printf("Final upload location: %s\n", final.Location)
}

// Example demonstrates the "concatenation-unfinished" extension. A file is split into parts, and the final upload is
// requested before the parts are transferred. The server concatenates the partial uploads once all of them are
// finished.
func ExampleClient_ConcatenateStreams_parallel() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	// Split the file into consecutive parts
	parts := SplitFile(f, finfo.Size(), 4)
	streams := make([]*tusgo.UploadStream, len(parts))
	for ind, part := range parts {
		u := tusgo.Upload{}
		if _, err = cl.CreateUpload(&u, part.Size(), true, nil); err != nil {
			log.Fatalf("Part #%d: failed to create upload: %s", ind, err)
		}
		fmt.Printf("Part #%d: location %s\n", ind, u.Location)
		streams[ind] = tusgo.NewUploadStream(cl, &u)
	}

	// Asking the server to concatenate the partial uploads into a final upload once they are finished.
	final := tusgo.Upload{}
	if _, err = cl.ConcatenateStreams(&final, streams, nil); err != nil {
		log.Fatalf("Failed to concatenate uploads: %s", err)
	}
	fmt.Printf("Final upload location: %s\n", final.Location)

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

// Example demonstrates the "creation-defer-length" extension. The upload is created without a size, which is
// reported to the server with the first data transfer.
func ExampleClient_CreateUpload_deferredSize() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, tusgo.SizeUnknown, false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()
	u.RemoteSize = finfo.Size() // Set size after the upload has been created on server

	stream := tusgo.NewUploadStream(cl, &u)
	stream.SetUploadSize = true
	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Printf("Uploading complete. Offset: %d, Size: %d\n", u.RemoteOffset, u.RemoteSize)
}

// Example demonstrates the "expiration" extension. The upload created in a previous run is resumed if it has not
// expired yet, otherwise a new upload is created.
func Example_expiration() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}

	if u.UploadExpired != nil {
		fmt.Printf("Upload expires at %s\n", u.UploadExpired.Format(time.RFC1123))
	} else {
		fmt.Printf("Upload does not expire\n")
	}
}

// Example demonstrates the "checksum" extension. The data is sent in chunks, and each chunk is verified by the server
// using the checksum sent in the request header.
func ExampleUploadStream_WithChecksumAlgorithm() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()
	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}

	// We want to use sha1
	stream := tusgo.NewUploadStream(cl, &u).WithChecksumAlgorithm("sha1")
	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Println("Uploading complete")
}

// Example demonstrates the "checksum-trailer" extension. The data is streamed in a single request without
// chunking, and the checksum of the whole data is sent in the HTTP trailer.
func ExampleUploadStream_WithChecksumAlgorithm_streamed() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)
	if _, err = cl.UpdateCapabilities(); err != nil {
		log.Fatalf("Failed to get server capabilities: %s", err)
	}

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()
	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	stream := tusgo.NewUploadStream(cl, &u).WithChecksumAlgorithm("sha256")
	stream.ChunkSize = tusgo.NoChunked // Checksum is sent in trailer only when chunking is disabled
	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Println("Uploading complete")
}

// rateLimitedReader limits the speed of reading from the underlying reader.
type rateLimitedReader struct {
	io.ReadSeeker
	bytesPerSecond int
}

func (r *rateLimitedReader) Read(p []byte) (int, error) {
	// Read small portions, 0.1 second worth of data at most, to keep the speed smooth
	if maxLen := r.bytesPerSecond / 10; len(p) > maxLen {
		p = p[:maxLen]
	}
	n, err := r.ReadSeeker.Read(p)
	// Sleep for the time that n bytes take at the given speed. The time spent on reading and sending is not taken
	// into account, so the actual speed is slightly lower than the limit.
	time.Sleep(time.Duration(n) * time.Second / time.Duration(r.bytesPerSecond))
	return n, err
}

// Example demonstrates the data transfer with speed limit.
func Example_transferWithSpeedControl() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)
	if _, err = cl.UpdateCapabilities(); err != nil {
		log.Fatalf("Failed to get server capabilities: %s", err)
	}

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()
	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}

	// Limit speed to 512 KiB/s
	src := &rateLimitedReader{ReadSeeker: f, bytesPerSecond: 512 * 1024}
	stream := tusgo.NewUploadStream(cl, &u)

	if err = UploadWithRetry(stream, src); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Printf("Uploading complete. Offset: %d, Size: %d\n", u.RemoteOffset, u.RemoteSize)
}

// Example demonstrates how to add the Authorization header to all requests by replacing [tusgo.Client.GetRequest].
// The function is called for every request the library sends, including the ones issued by [tusgo.UploadStream].
func ExampleClient_headers() {
	baseURL, err := url.Parse("http://example.com/files")
	if err != nil {
		log.Fatalf("Failed to parse URL: %s", err)
	}
	cl := tusgo.NewClient(http.DefaultClient, baseURL)
	cl.GetRequest = func(method, url string, body io.Reader, _ *tusgo.Client, _ *http.Client) (*http.Request, error) {
		req, err := http.NewRequest(method, url, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+os.Getenv("TUS_TOKEN"))
		return req, nil
	}

	f, finfo, err := OpenFile("/tmp/file.txt")
	if err != nil {
		log.Fatalf("%s", err)
	}
	defer f.Close()

	u := tusgo.Upload{}
	if _, err = cl.CreateUpload(&u, finfo.Size(), false, nil); err != nil {
		log.Fatalf("Failed to create upload: %s", err)
	}
	fmt.Printf("Location: %s\n", u.Location)

	stream := tusgo.NewUploadStream(cl, &u)
	if err = UploadWithRetry(stream, f); err != nil {
		log.Fatalf("Failed to upload: %s", err)
	}
	fmt.Println("Uploading complete")
}
