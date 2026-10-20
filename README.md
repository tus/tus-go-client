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

- **Resumable upload streams**: `ChunkedStream` implements `io.ReaderFrom` and transfers the data in chunks, one
  request per chunk, so it works with standard library helpers such as `io.Copy`. `RawStream` implements `io.Writer`
  and sends each write in a single request.
- **Upload management client** for creating, deleting, concatenating, and otherwise manipulating uploads.
- **Protocol extensions** support (see below).

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

	// Transfer the file in chunks of 2 MiB, one request per chunk
	w := tusgo.NewChunkedWriter(s, 2*1024*1024)
	written, err := io.Copy(w, f)
	if err != nil {
		log.Fatalf("Written %d bytes, error: %s", written, err)
	}
	log.Printf("Written %d bytes\n", written)
}
```

More examples are available in the [package documentation](https://pkg.go.dev/github.com/bdragon300/tusgo/v2#pkg-examples).
