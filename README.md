# tus-go-client

[![Codecov](https://img.shields.io/codecov/c/gh/tus/tus-go-client/master)](https://app.codecov.io/gh/tus/tus-go-client/tree/master)
![GitHub Workflow Status (with branch)](https://img.shields.io/github/actions/workflow/status/bdragon300/tusgo/ci.yml?branch=master)
[![Go reference](https://pkg.go.dev/badge/github.com/bdragon300/tusgo/v2)](https://pkg.go.dev/github.com/bdragon300/tusgo/v2)
![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/bdragon300/tusgo/v2)

> **tus** is a protocol based on HTTP for *resumable file uploads*. Resumable
> means that an upload can be interrupted at any moment and can be resumed without
> re-uploading the previous data again. An interruption may happen willingly, if
> the user wants to pause, or by accident in case of a network issue or server
> outage.

**tus-go-client** is a Go client library for the [tus](https://tus.io) resumable upload protocol. It provides complete
coverage of the [tus 1.0.0 specification](https://tus.io/protocols/resumable-upload), including the core protocol and
all officially defined extensions.

The API reference is available on [pkg.go.dev](https://pkg.go.dev/github.com/bdragon300/tusgo/v2).

## Features

- **Resumable chunked uploads that work with stdlib.** `ChunkedWriter` implements `io.Writer` and `io.ReaderFrom`
  and sends the data in chunks, one request per chunk. After a failure, the transfer may be resumed where it stopped.
- **Upload management.** `Client` creates, inspects, deletes and concatenates uploads on the server.
- **Low-level control when you need it.** `Stream` is a bare tus transfer sequence without buffering,
  so you can build your own chunking, buffering or retry strategy on top of it.
- **Complete protocol coverage.** All tus 1.0.0 extensions are supported, see below.

### Supported extensions

| Extension                  | Description                                                                                                             |
|----------------------------|-------------------------------------------------------------------------------------------------------------------------|
| `creation`                 | Create new uploads on the server                                                                                        |
| `creation-defer-length`    | Create an upload without specifying its size, which is declared later.                                                  |
| `creation-with-upload`     | Create an upload and transfer its data in a single HTTP request.                                                        |
| `expiration`               | Uploads with expiration date.                                                                                           |
| `checksum`                 | Verify data integrity of chunked uploads. Many checksum algorithms are supported.                                       |
| `checksum-trailer`         | Verify data integrity of streamed uploads. The checksum is computed over the entire stream and sent in an HTTP trailer. |
| `termination`              | Delete uploads from the server.                                                                                         |
| `concatenation`            | Upload the data in multiple parts in parallel and concatenate them into a single upload.                                |
| `concatenation-unfinished` | Same as `concatenation`, but asking the server to concatenate parts automatically once they will be finished.           |

## Installation

```shell
go get github.com/bdragon300/tusgo/v2
```

## Usage

The following example resumes an upload that already exists on the server and transfers the contents of a local
file to it.

```go
package main

import (
	"io"
	"log"
	"net/url"
	"os"
	"time"
	"errors"

	"github.com/bdragon300/tusgo/v2"
)

func main() {
	baseURL, _ := url.Parse("http://example.com/files")
	cl := tusgo.NewClient(baseURL)

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

	// Transfer the file in chunks of 2 MiB, one request per chunk with automatic retry on failure
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
```

More examples are available in the [package documentation](https://pkg.go.dev/github.com/bdragon300/tusgo/v2#pkg-examples).

## Migrating from 0.x to 2.x

2.x is a new major version with a new module path, so 0.x users keep getting 0.x until they switch explicitly.
Change the import paths from `github.com/bdragon300/tusgo` to `github.com/bdragon300/tusgo/v2` (and
`.../tusgo/checksum` to `.../tusgo/v2/checksum`), then update the code according to the key changes below.

### Key changes

1. **Values instead of out-parameters.** `Client` methods no longer fill a caller-provided `*Upload` and no longer
   return `*http.Response`. They return the result and an error in traditional Go style.
2. **The HTTP response is attached to the error.** To inspect a failed response, use `Response()` or `StatusCode()`
   methods. This replaces the responses returned from `Client` and `UploadStream.LastResponse`. To access all
   responses, use `Client.DoRequest`.
3. **Uploads are addressed by location.** `DeleteUpload` and `ConcatenatePartialUploads` take location strings instead of
   `Upload` values or streams.
4. **`UploadStream` is split into `Stream` and `ChunkedWriter`.** `Stream` is an `io.Writer`-only, that implements 
   the byte sending sequence transferring data in unbuffered requests. `ChunkedWriter` adds in-memory buffered 
   chunking and can resume the failed transmission. Any other buffering algorithm (e.g. using a database) can be 
   implemented atop of `Stream`.
5. **A stream no longer owns the `Upload`.** `NewStream` takes `Upload` by value and copies its state. The stream
   offset is available only via `Tell()`, and `Upload.RemoteOffset` is no longer updated.
6. **No context methods.** `WithContext` is removed from both `Client` and stream. You can attach a context to 
   requests in `Client.DoRequest`.
7. **Request customization.** `NewClient` takes only the endpoint URL. Set the HTTP client in the `Client.Client` field,
   or set `Client.DoRequest` to intercept every request (auth, tracing, rate limiting, etc). `GetRequest` is removed.
   `ExtraHeaders` are now always sent.
8. **Protocol version and capabilities are handled automatically.** The client queries the server on first use and
   picks a supported protocol version. Call `FetchCapabilities()` if you need the server capabilities yourself.
   `Client.Capabilities` is no longer available.
9. **Deferred size is declared explicitly.** Instead of setting `RemoteSize` and `SetUploadSize` before the
   first write, call `stream.DeclareSize(size)` at any time, for example after the source is exhausted.

See [package documentation](https://pkg.go.dev/github.com/bdragon300/tusgo/v2) for more details on API v2.