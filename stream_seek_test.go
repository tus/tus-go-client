package tusgo

import (
	"io"
	"net/http"
	"net/url"
	"testing"
)

func newSeekTestStream(size int64) *UploadStream {
	base, _ := url.Parse("http://example.com/files")
	u := Upload{Location: "/foo/bar", RemoteSize: size}
	return NewUploadStream(NewClient(http.DefaultClient, base), &u)
}

func TestUploadStreamSeekClampsToRemoteSize(t *testing.T) {
	s := newSeekTestStream(1000)

	// SeekEnd used RemoteSize-1+offset and the old bound checked the relative
	// offset, so Seek(500, SeekEnd) stored 1499. Clamp to RemoteSize.
	offset, err := s.Seek(500, io.SeekEnd)
	if err != nil {
		t.Fatalf("Seek(500, SeekEnd): %v", err)
	}
	if offset != 1000 || s.Upload.RemoteOffset != 1000 {
		t.Fatalf("Seek(500, SeekEnd) = (%d, RemoteOffset=%d), want 1000", offset, s.Upload.RemoteOffset)
	}

	s = newSeekTestStream(1000)
	offset, err = s.Seek(1500, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek(1500, SeekStart): %v", err)
	}
	if offset != 1000 || s.Upload.RemoteOffset != 1000 {
		t.Fatalf("Seek(1500, SeekStart) = (%d, RemoteOffset=%d), want 1000", offset, s.Upload.RemoteOffset)
	}

	s = newSeekTestStream(1000)
	if _, err = s.Seek(999, io.SeekStart); err != nil {
		t.Fatalf("Seek(999, SeekStart): %v", err)
	}
	offset, err = s.Seek(500, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek(500, SeekCurrent): %v", err)
	}
	if offset != 1000 || s.Upload.RemoteOffset != 1000 {
		t.Fatalf("Seek(500, SeekCurrent) from 999 = (%d, RemoteOffset=%d), want 1000", offset, s.Upload.RemoteOffset)
	}
}

func TestUploadStreamSeekInRange(t *testing.T) {
	s := newSeekTestStream(1000)

	offset, err := s.Seek(500, io.SeekCurrent)
	if err != nil {
		t.Fatalf("Seek(500, SeekCurrent): %v", err)
	}
	if offset != 500 || s.Upload.RemoteOffset != 500 {
		t.Fatalf("Seek(500, SeekCurrent) = (%d, RemoteOffset=%d), want 500", offset, s.Upload.RemoteOffset)
	}

	offset, err = s.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek(0, SeekStart): %v", err)
	}
	if offset != 0 {
		t.Fatalf("Seek(0, SeekStart) = %d, want 0", offset)
	}

	offset, err = s.Seek(1000, io.SeekStart)
	if err != nil {
		t.Fatalf("Seek(1000, SeekStart): %v", err)
	}
	if offset != 1000 || s.Upload.RemoteOffset != 1000 {
		t.Fatalf("Seek(1000, SeekStart) = (%d, RemoteOffset=%d), want 1000", offset, s.Upload.RemoteOffset)
	}
}

func TestUploadStreamSeekNegative(t *testing.T) {
	s := newSeekTestStream(1000)
	offset, err := s.Seek(-1, io.SeekStart)
	if err == nil {
		t.Fatalf("Seek(-1, SeekStart) succeeded with offset %d, want error", offset)
	}
	if s.Upload.RemoteOffset != 0 {
		t.Fatalf("failed Seek mutated RemoteOffset to %d", s.Upload.RemoteOffset)
	}
}
