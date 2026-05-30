// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	generatedTusNodeFileFingerprintPath         = "absolute"
	generatedTusNodeFileFingerprintPrefix       = "node-file"
	generatedTusNodeFileFingerprintSeparator    = "-"
	generatedTusChunkCompleteAfterChunkAccepted = "accepted-chunk-size-and-offset"
	generatedTusProgressAfterChunkAccepted      = "accepted-offset"
	generatedTusProgressAfterResumeComplete     = "upload-length"
	generatedTusProgressBeforeRequestBody       = "current-offset"
	generatedTusProgressDuringRequest           = "start-offset-plus-transmitted-bytes"
	generatedTusProgressParallelPart            = "aggregated-part-progress"
	generatedTusAbortErrorMessage               = "Request was aborted"
	generatedTusAbortRemoveStoredURLAfterTerm   = "after-successful-termination"
	generatedTusAbortSuppressErrorAfterAbort    = true
	generatedTusAbortTerminateUpload            = "when-requested-and-upload-url-known"
	generatedTusDefaultParallelUploads          = 1
	generatedTusMinimumParallelUploads          = 2
	generatedTusParallelPartialMetadata         = "metadataForPartialUploads"
	generatedTusParallelPartialNestedUploads    = "disabled"
	generatedTusParallelPartialURLStorage       = "parent-managed"
	generatedTusParallelUploadSplit             = "contiguous-floor-size-last-remainder"
	generatedTusRetryClientErrorStatus          = 400
	generatedTusRetryStatusCategoryDivisor      = 100
	generatedTusSuccessCloseSource              = "after-hook-when-source-open"
	generatedTusSuccessEmit                     = "after-upload-complete"
	generatedTusSuccessRemoveStoredURL          = "before-hook-when-option-enabled"
	generatedTusUploadURLAvailableCreate        = "after-url-known-before-storage"
	generatedTusUploadURLAvailableParallel      = "not-emitted"
	generatedTusUploadURLAvailableResume        = "after-url-known-before-storage"
	generatedTusURLStorageIDMultiplier          = 1000000000000
	generatedTusURLStorageIDStrategy            = "rounded-random-number"
	generatedTusURLStorageNamespace             = "tus"
	generatedTusURLStorageSeparator             = "::"
	generatedTusURLStorageCreationTime          = "sdk-current-date-string"
)

var generatedTusNodeFileFingerprintFields = []string{"prefix", "absolutePath", "size", "mtimeMs", "endpoint"}
var generatedTusAbortSequence = []string{"mark-aborted", "abort-parallel-uploads", "abort-current-request", "clear-retry-timer", "terminate-upload-if-requested"}
var generatedTusDefaultRetryDelays = []time.Duration{0 * time.Millisecond, 1000 * time.Millisecond, 3000 * time.Millisecond, 5000 * time.Millisecond}
var generatedTusRetryableClientStatusCodes = []int{409, 423}

type FileFingerprintInput struct {
	AbsolutePath string
	Endpoint     string
	MtimeMs      int64
	Size         int64
}

type URLStorageUpload map[string]any

type UploadSuccessPayload struct {
	Upload       *Upload
	LastResponse *http.Response
}

type UploadEventHooks struct {
	OnProgress           func(bytesSent int64, bytesTotal *int64) error
	OnChunkComplete      func(chunkSize int64, bytesAccepted int64, bytesTotal *int64) error
	OnSuccess            func(UploadSuccessPayload) error
	OnUploadURLAvailable func() error
}

type URLStorage interface {
	FindAllUploads() ([]URLStorageUpload, error)
	FindUploadsByFingerprint(fingerprint string) ([]URLStorageUpload, error)
	RemoveUpload(urlStorageKey string) error
	AddUpload(fingerprint string, upload URLStorageUpload) (string, error)
}

type URLStorageUploadOptions struct {
	Context                    context.Context
	Storage                    URLStorage
	Source                     io.ReadSeeker
	Fingerprint                string
	Size                       int64
	Metadata                   map[string]string
	MetadataForPartialUploads  map[string]string
	ParallelUploads            int
	RemoveFingerprintOnSuccess bool
	TerminateUploadOnAbort     bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
	EventHooks                 UploadEventHooks
}

type URLStorageFileUploadOptions struct {
	Context                    context.Context
	Storage                    URLStorage
	Path                       string
	Metadata                   map[string]string
	MetadataForPartialUploads  map[string]string
	ParallelUploads            int
	RemoveFingerprintOnSuccess bool
	TerminateUploadOnAbort     bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
	EventHooks                 UploadEventHooks
}

type FileBackedURLStorageUploadOptions struct {
	Context                    context.Context
	URLStoragePath             string
	Path                       string
	Metadata                   map[string]string
	MetadataForPartialUploads  map[string]string
	ParallelUploads            int
	RemoveFingerprintOnSuccess bool
	TerminateUploadOnAbort     bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
	EventHooks                 UploadEventHooks
}

type generatedTusSuccessInput struct {
	EventHooks                 UploadEventHooks
	LastResponse               *http.Response
	RemoveFingerprintOnSuccess bool
	Source                     io.ReadSeeker
	Storage                    URLStorage
	StorageKey                 string
	Upload                     *Upload
}

type MemoryURLStorage struct {
	mu      sync.Mutex
	records map[string]URLStorageUpload
}

func NewMemoryURLStorage() *MemoryURLStorage {
	return &MemoryURLStorage{
		records: map[string]URLStorageUpload{},
	}
}

func (storage *MemoryURLStorage) FindAllUploads() ([]URLStorageUpload, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	return urlStorageUploadsWithPrefix(storage.records, URLStorageAllUploadsPrefix())
}

func (storage *MemoryURLStorage) FindUploadsByFingerprint(fingerprint string) ([]URLStorageUpload, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	return urlStorageUploadsWithPrefix(storage.records, URLStorageFingerprintPrefix(fingerprint))
}

func (storage *MemoryURLStorage) RemoveUpload(urlStorageKey string) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	delete(storage.records, urlStorageKey)
	return nil
}

func (storage *MemoryURLStorage) AddUpload(fingerprint string, upload URLStorageUpload) (string, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	key := newURLStorageKey(fingerprint)
	cloned, err := cloneURLStorageUpload(upload)
	if err != nil {
		return "", err
	}

	storage.records[key] = cloned
	return key, nil
}

type FileURLStorage struct {
	path string
	mu   sync.Mutex
}

func NewFileURLStorage(path string) *FileURLStorage {
	return &FileURLStorage{path: path}
}

func (storage *FileURLStorage) FindAllUploads() ([]URLStorageUpload, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	records, err := storage.readRecords()
	if err != nil {
		return nil, err
	}

	return urlStorageUploadsWithPrefix(records, URLStorageAllUploadsPrefix())
}

func (storage *FileURLStorage) FindUploadsByFingerprint(fingerprint string) ([]URLStorageUpload, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	records, err := storage.readRecords()
	if err != nil {
		return nil, err
	}

	return urlStorageUploadsWithPrefix(records, URLStorageFingerprintPrefix(fingerprint))
}

func (storage *FileURLStorage) RemoveUpload(urlStorageKey string) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	records, err := storage.readRecords()
	if err != nil {
		return err
	}

	delete(records, urlStorageKey)
	return storage.writeRecords(records)
}

func (storage *FileURLStorage) AddUpload(fingerprint string, upload URLStorageUpload) (string, error) {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	records, err := storage.readRecords()
	if err != nil {
		return "", err
	}

	key := newURLStorageKey(fingerprint)
	cloned, err := cloneURLStorageUpload(upload)
	if err != nil {
		return "", err
	}

	records[key] = cloned
	return key, storage.writeRecords(records)
}

func (storage *FileURLStorage) readRecords() (map[string]URLStorageUpload, error) {
	data, err := os.ReadFile(storage.path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]URLStorageUpload{}, nil
	}
	if err != nil {
		return nil, err
	}

	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]URLStorageUpload{}, nil
	}

	records := map[string]URLStorageUpload{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&records); err != nil {
		return nil, err
	}

	return records, nil
}

func (storage *FileURLStorage) writeRecords(records map[string]URLStorageUpload) error {
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}

	return os.WriteFile(storage.path, data, 0o660)
}

func URLStorageAllUploadsPrefix() string {
	return generatedTusURLStorageNamespace + generatedTusURLStorageSeparator
}

func URLStorageFingerprintPrefix(fingerprint string) string {
	return URLStorageAllUploadsPrefix() + fingerprint + generatedTusURLStorageSeparator
}

func URLStorageKey(fingerprint string, id int64) string {
	return fmt.Sprintf("%s%d", URLStorageFingerprintPrefix(fingerprint), id)
}

func URLStorageID(randomValue float64) int64 {
	if generatedTusURLStorageIDStrategy != "rounded-random-number" {
		panic(fmt.Sprintf("tus: unsupported URL storage ID policy %s", generatedTusURLStorageIDStrategy))
	}

	return int64(math.Round(randomValue * generatedTusURLStorageIDMultiplier))
}

func newURLStorageKey(fingerprint string) string {
	return URLStorageKey(fingerprint, URLStorageID(rand.Float64()))
}

func (c *Client) UploadFileWithFileBackedURLStorage(options FileBackedURLStorageUploadOptions) (*Upload, error) {
	return c.UploadFileWithURLStorage(URLStorageFileUploadOptions{
		Context:                    options.Context,
		Storage:                    NewFileURLStorage(options.URLStoragePath),
		Path:                       options.Path,
		Metadata:                   options.Metadata,
		MetadataForPartialUploads:  options.MetadataForPartialUploads,
		ParallelUploads:            options.ParallelUploads,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		TerminateUploadOnAbort:     options.TerminateUploadOnAbort,
		ChunkSize:                  options.ChunkSize,
		RetryDelays:                options.RetryDelays,
		OnShouldRetry:              options.OnShouldRetry,
		EventHooks:                 options.EventHooks,
	})
}

func (c *Client) UploadFileWithURLStorage(options URLStorageFileUploadOptions) (*Upload, error) {
	absolutePath, err := nodeFileFingerprintPath(options.Path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(absolutePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fingerprint := FileFingerprint(FileFingerprintInput{
		AbsolutePath: absolutePath,
		Endpoint:     c.BaseURL.String(),
		MtimeMs:      fileModTimeMilliseconds(info),
		Size:         info.Size(),
	})

	return c.UploadWithURLStorage(URLStorageUploadOptions{
		Context:                    options.Context,
		Storage:                    options.Storage,
		Source:                     file,
		Fingerprint:                fingerprint,
		Size:                       info.Size(),
		Metadata:                   options.Metadata,
		MetadataForPartialUploads:  options.MetadataForPartialUploads,
		ParallelUploads:            options.ParallelUploads,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		TerminateUploadOnAbort:     options.TerminateUploadOnAbort,
		ChunkSize:                  options.ChunkSize,
		RetryDelays:                options.RetryDelays,
		OnShouldRetry:              options.OnShouldRetry,
		EventHooks:                 options.EventHooks,
	})
}

func (c *Client) UploadWithURLStorage(options URLStorageUploadOptions) (*Upload, error) {
	if options.Storage == nil {
		return nil, errors.New("tus: URL storage is required")
	}
	if options.Source == nil {
		return nil, errors.New("tus: upload source is required")
	}
	if options.Fingerprint == "" {
		return nil, errors.New("tus: unable to calculate fingerprint for this input file")
	}

	uploadClient, err := generatedTusClientWithUploadContext(c, options.Context)
	if err != nil {
		return nil, err
	}
	parallelUploads, err := generatedTusParallelUploadCount(options.ParallelUploads)
	if err != nil {
		return nil, err
	}
	if parallelUploads > 1 {
		return uploadClient.uploadParallelWithURLStorage(options, parallelUploads)
	}

	upload, storageKey, err := uploadClient.resumeUploadFromURLStorage(options)
	if err != nil {
		return upload, err
	}
	if upload == nil {
		upload, storageKey, err = uploadClient.createUploadForURLStorage(options)
		if err != nil {
			return upload, err
		}
	}

	stream := NewUploadStream(uploadClient, upload)
	if options.ChunkSize != 0 {
		stream.ChunkSize = options.ChunkSize
	}
	if err := uploadClient.uploadURLStorageSource(options, stream); err != nil {
		return upload, c.generatedTusHandleURLStorageUploadAbort(options, upload, storageKey, err)
	}
	if err := generatedTusEmitSuccess(generatedTusSuccessInput{
		EventHooks:                 options.EventHooks,
		LastResponse:               stream.LastResponse,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		Source:                     options.Source,
		Storage:                    options.Storage,
		StorageKey:                 storageKey,
		Upload:                     upload,
	}); err != nil {
		return upload, err
	}

	return upload, nil
}

func (c *Client) uploadParallelWithURLStorage(
	options URLStorageUploadOptions,
	parallelUploads int,
) (*Upload, error) {
	if err := generatedTusAssertParallelUploadPolicySupported(); err != nil {
		return nil, err
	}
	partSizes, err := generatedTusParallelUploadPartSizes(options.Size, parallelUploads)
	if err != nil {
		return nil, err
	}

	partials := make([]Upload, 0, len(partSizes))
	acceptedBytes := int64(0)
	for _, partSize := range partSizes {
		if _, err := options.Source.Seek(acceptedBytes, io.SeekStart); err != nil {
			return nil, err
		}
		partBytes, err := readURLStorageUploadChunk(options.Source, partSize, partSize)
		if err != nil {
			return nil, err
		}

		partialUpload := Upload{}
		if _, err := c.CreateUpload(
			&partialUpload,
			partSize,
			true,
			generatedTusParallelPartialUploadMetadata(options),
		); err != nil {
			return &partialUpload, err
		}
		stream := NewUploadStream(c, &partialUpload)
		stream.ChunkSize = partSize
		written, err := stream.Write(partBytes)
		if err != nil {
			return &partialUpload, c.generatedTusHandleURLStorageUploadAbort(options, &partialUpload, "", err)
		}
		if int64(written) != partSize {
			return &partialUpload, fmt.Errorf("tus: expected to upload %d parallel bytes, wrote %d", partSize, written)
		}

		acceptedBytes += partSize
		if err := generatedTusEmitProgressAfterChunkAccepted(
			options.EventHooks,
			acceptedBytes,
			options.Size,
		); err != nil {
			return &partialUpload, err
		}
		if err := generatedTusEmitChunkCompleteAfterChunkAccepted(
			options.EventHooks,
			partSize,
			acceptedBytes,
			options.Size,
		); err != nil {
			return &partialUpload, err
		}
		partials = append(partials, partialUpload)
	}

	finalUpload := &Upload{}
	response, err := c.ConcatenateUploads(finalUpload, partials, options.Metadata)
	if err != nil {
		return finalUpload, err
	}
	if err := generatedTusEmitUploadURLAvailable(options.EventHooks, "parallelFinalUpload"); err != nil {
		return finalUpload, err
	}
	storageKey, err := options.Storage.AddUpload(
		options.Fingerprint,
		URLStorageUploadFromUpload(*finalUpload),
	)
	if err != nil {
		return finalUpload, err
	}
	if err := generatedTusEmitSuccess(generatedTusSuccessInput{
		EventHooks:                 options.EventHooks,
		LastResponse:               response,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		Source:                     options.Source,
		Storage:                    options.Storage,
		StorageKey:                 storageKey,
		Upload:                     finalUpload,
	}); err != nil {
		return finalUpload, err
	}

	return finalUpload, nil
}

func (c *Client) uploadURLStorageSource(
	options URLStorageUploadOptions,
	stream *UploadStream,
) error {
	retryDelays := generatedTusRetryDelays(options.RetryDelays)
	retryAttempt := 0
	offsetBeforeRetry := stream.Upload.RemoteOffset

	for {
		if _, err := options.Source.Seek(stream.Upload.RemoteOffset, io.SeekStart); err != nil {
			return err
		}
		chunk, err := readURLStorageUploadChunk(
			options.Source,
			stream.ChunkSize,
			options.Size-stream.Upload.RemoteOffset,
		)
		if err != nil {
			return err
		}
		if len(chunk) == 0 {
			return nil
		}

		startOffset := stream.Upload.RemoteOffset
		if err := generatedTusEmitProgressBeforeRequestBody(
			options.EventHooks,
			startOffset,
			options.Size,
		); err != nil {
			return err
		}
		if _, err := stream.Write(chunk); err != nil {
			effectiveRetryAttempt := generatedTusRetryAttempt(
				stream.Upload.RemoteOffset,
				offsetBeforeRetry,
				retryAttempt,
			)
			if !generatedTusShouldScheduleRetry(
				options.OnShouldRetry,
				err,
				stream.lastResponseStatus(),
				effectiveRetryAttempt,
				retryDelays,
			) {
				return err
			}
			delay := retryDelays[effectiveRetryAttempt]
			if delay > 0 {
				time.Sleep(delay)
			}
			retryAttempt = effectiveRetryAttempt + 1
			offsetBeforeRetry = stream.Upload.RemoteOffset
			if _, err := stream.Sync(); err != nil {
				return err
			}
			stream.ForceClean()
			continue
		}
		if stream.Upload.RemoteOffset > startOffset {
			chunkSize := stream.Upload.RemoteOffset - startOffset
			if err := generatedTusEmitProgressAfterChunkAccepted(
				options.EventHooks,
				stream.Upload.RemoteOffset,
				options.Size,
			); err != nil {
				return err
			}
			if err := generatedTusEmitChunkCompleteAfterChunkAccepted(
				options.EventHooks,
				chunkSize,
				stream.Upload.RemoteOffset,
				options.Size,
			); err != nil {
				return err
			}
		}

		if stream.Upload.RemoteOffset >= options.Size {
			return nil
		}
	}
}

func readURLStorageUploadChunk(
	source io.Reader,
	chunkSize int64,
	remaining int64,
) ([]byte, error) {
	if remaining <= 0 {
		return nil, nil
	}

	bytesToRead := remaining
	if chunkSize > 0 && chunkSize < bytesToRead {
		bytesToRead = chunkSize
	}
	if bytesToRead > int64(int(bytesToRead)) {
		return nil, fmt.Errorf("tus: upload chunk size %d is too large for this platform", bytesToRead)
	}

	chunk := make([]byte, int(bytesToRead))
	if _, err := io.ReadFull(source, chunk); err != nil {
		return nil, err
	}

	return chunk, nil
}

func (us *UploadStream) lastResponseStatus() int {
	if us.LastResponse == nil {
		return 0
	}

	return us.LastResponse.StatusCode
}

func generatedTusClientWithUploadContext(client *Client, ctx context.Context) (*Client, error) {
	if ctx == nil {
		return client, nil
	}
	if err := generatedTusAssertAbortPolicySupported(); err != nil {
		return nil, err
	}

	return client.WithContext(ctx), nil
}

func IsUploadAbortError(err error) bool {
	return errors.Is(err, context.Canceled)
}

func generatedTusParallelUploadCount(parallelUploads int) (int, error) {
	if parallelUploads == 0 {
		parallelUploads = generatedTusDefaultParallelUploads
	}
	if parallelUploads == 1 {
		return parallelUploads, nil
	}
	if parallelUploads < generatedTusMinimumParallelUploads {
		return 0, fmt.Errorf(
			"tus: parallel uploads must be at least %d",
			generatedTusMinimumParallelUploads,
		)
	}

	return parallelUploads, nil
}

func generatedTusParallelUploadPartSizes(uploadSize int64, parallelUploads int) ([]int64, error) {
	if uploadSize < 0 {
		return nil, fmt.Errorf("tus: parallel upload size must be known")
	}
	if parallelUploads <= 0 {
		return nil, fmt.Errorf("tus: parallel upload count must be positive")
	}
	partSize := uploadSize / int64(parallelUploads)
	if partSize <= 0 {
		return nil, fmt.Errorf("tus: parallel upload parts must not be empty")
	}

	partSizes := make([]int64, parallelUploads)
	for index := range partSizes {
		partSizes[index] = partSize
	}
	partSizes[len(partSizes)-1] += uploadSize - partSize*int64(parallelUploads)

	return partSizes, nil
}

func generatedTusParallelPartialUploadMetadata(options URLStorageUploadOptions) map[string]string {
	if generatedTusParallelPartialMetadata != "metadataForPartialUploads" {
		panic(fmt.Sprintf(
			"tus: unsupported parallel partial metadata policy %s",
			generatedTusParallelPartialMetadata,
		))
	}

	return cloneStringMap(options.MetadataForPartialUploads)
}

func (c *Client) generatedTusHandleURLStorageUploadAbort(
	options URLStorageUploadOptions,
	upload *Upload,
	storageKey string,
	err error,
) error {
	if !IsUploadAbortError(err) {
		return err
	}
	if err := generatedTusAssertAbortPolicySupported(); err != nil {
		return err
	}
	if !options.TerminateUploadOnAbort || upload == nil || upload.Location == "" {
		return err
	}

	if _, terminateErr := c.TerminateUploadWithRetry(*upload, TerminateUploadOptions{
		RetryDelays:   options.RetryDelays,
		OnShouldRetry: options.OnShouldRetry,
	}); terminateErr != nil {
		return terminateErr
	}
	if storageKey != "" {
		if err := options.Storage.RemoveUpload(storageKey); err != nil {
			return err
		}
	}

	return err
}

func generatedTusRetryDelays(retryDelays []time.Duration) []time.Duration {
	if retryDelays == nil {
		return append([]time.Duration(nil), generatedTusDefaultRetryDelays...)
	}

	return retryDelays
}

func generatedTusRetryAttempt(offset int64, offsetBeforeRetry int64, retryAttempt int) int {
	if offset > offsetBeforeRetry {
		return 0
	}

	return retryAttempt
}

func generatedTusShouldScheduleRetry(
	onShouldRetry func(error, int) bool,
	err error,
	statusCode int,
	retryAttempt int,
	retryDelays []time.Duration,
) bool {
	if retryAttempt >= len(retryDelays) || !generatedTusShouldRetryStatus(statusCode) {
		return false
	}
	if onShouldRetry != nil {
		return onShouldRetry(err, retryAttempt)
	}

	return true
}

func generatedTusShouldRetryStatus(statusCode int) bool {
	if statusCode == 0 {
		return false
	}
	if statusCode/generatedTusRetryStatusCategoryDivisor != generatedTusRetryClientErrorStatus/generatedTusRetryStatusCategoryDivisor {
		return true
	}
	for _, retryableStatusCode := range generatedTusRetryableClientStatusCodes {
		if statusCode == retryableStatusCode {
			return true
		}
	}

	return false
}

func generatedTusEmitUploadURLAvailable(hooks UploadEventHooks, context string) error {
	if hooks.OnUploadURLAvailable == nil {
		return nil
	}
	if err := generatedTusAssertUploadURLAvailableHookPolicySupported(); err != nil {
		return err
	}

	switch context {
	case "createUpload":
		if generatedTusUploadURLAvailableCreate == "not-emitted" {
			return nil
		}
	case "resumeUpload":
		if generatedTusUploadURLAvailableResume == "not-emitted" {
			return nil
		}
	case "parallelFinalUpload":
		if generatedTusUploadURLAvailableParallel == "not-emitted" {
			return nil
		}
	default:
		return fmt.Errorf("tus: unsupported upload URL available hook context %s", context)
	}

	return hooks.OnUploadURLAvailable()
}

func generatedTusEmitProgressBeforeRequestBody(
	hooks UploadEventHooks,
	currentOffset int64,
	bytesTotal int64,
) error {
	if hooks.OnProgress == nil {
		return nil
	}
	if err := generatedTusAssertEventHookPolicySupported(); err != nil {
		return err
	}

	return hooks.OnProgress(currentOffset, generatedTusInt64Pointer(bytesTotal))
}

func generatedTusEmitProgressAfterChunkAccepted(
	hooks UploadEventHooks,
	uploadOffset int64,
	bytesTotal int64,
) error {
	if hooks.OnProgress == nil {
		return nil
	}
	if err := generatedTusAssertEventHookPolicySupported(); err != nil {
		return err
	}

	return hooks.OnProgress(uploadOffset, generatedTusInt64Pointer(bytesTotal))
}

func generatedTusEmitChunkCompleteAfterChunkAccepted(
	hooks UploadEventHooks,
	chunkSize int64,
	bytesAccepted int64,
	bytesTotal int64,
) error {
	if hooks.OnChunkComplete == nil {
		return nil
	}
	if err := generatedTusAssertEventHookPolicySupported(); err != nil {
		return err
	}

	return hooks.OnChunkComplete(
		chunkSize,
		bytesAccepted,
		generatedTusInt64Pointer(bytesTotal),
	)
}

func generatedTusEmitSuccess(input generatedTusSuccessInput) error {
	if err := generatedTusAssertEventHookPolicySupported(); err != nil {
		return err
	}
	if input.RemoveFingerprintOnSuccess && input.StorageKey != "" {
		if err := input.Storage.RemoveUpload(input.StorageKey); err != nil {
			return err
		}
	}
	if input.EventHooks.OnSuccess != nil {
		if err := input.EventHooks.OnSuccess(UploadSuccessPayload{
			Upload:       input.Upload,
			LastResponse: input.LastResponse,
		}); err != nil {
			return err
		}
	}

	closer, ok := input.Source.(io.Closer)
	if ok {
		return closer.Close()
	}

	return nil
}

func generatedTusInt64Pointer(value int64) *int64 {
	return &value
}

func generatedTusAssertUploadURLAvailableHookPolicySupported() error {
	if generatedTusUploadURLAvailableCreate != "after-url-known-before-storage" {
		return fmt.Errorf(
			"tus: unsupported create upload URL hook policy %s",
			generatedTusUploadURLAvailableCreate,
		)
	}
	if generatedTusUploadURLAvailableResume != "after-url-known-before-storage" {
		return fmt.Errorf(
			"tus: unsupported resume upload URL hook policy %s",
			generatedTusUploadURLAvailableResume,
		)
	}
	if generatedTusUploadURLAvailableParallel != "not-emitted" {
		return fmt.Errorf(
			"tus: unsupported parallel final upload URL hook policy %s",
			generatedTusUploadURLAvailableParallel,
		)
	}

	return nil
}

func generatedTusAssertEventHookPolicySupported() error {
	if err := generatedTusAssertUploadURLAvailableHookPolicySupported(); err != nil {
		return err
	}
	if generatedTusProgressAfterChunkAccepted != "accepted-offset" {
		return fmt.Errorf(
			"tus: unsupported chunk-accepted progress hook policy %s",
			generatedTusProgressAfterChunkAccepted,
		)
	}
	if generatedTusProgressAfterResumeComplete != "upload-length" {
		return fmt.Errorf(
			"tus: unsupported completed-resume progress hook policy %s",
			generatedTusProgressAfterResumeComplete,
		)
	}
	if generatedTusProgressBeforeRequestBody != "current-offset" {
		return fmt.Errorf(
			"tus: unsupported request-body progress hook policy %s",
			generatedTusProgressBeforeRequestBody,
		)
	}
	if generatedTusProgressDuringRequest != "start-offset-plus-transmitted-bytes" {
		return fmt.Errorf(
			"tus: unsupported request progress hook policy %s",
			generatedTusProgressDuringRequest,
		)
	}
	if generatedTusProgressParallelPart != "aggregated-part-progress" {
		return fmt.Errorf(
			"tus: unsupported parallel progress hook policy %s",
			generatedTusProgressParallelPart,
		)
	}
	if generatedTusChunkCompleteAfterChunkAccepted != "accepted-chunk-size-and-offset" {
		return fmt.Errorf(
			"tus: unsupported chunk-complete hook policy %s",
			generatedTusChunkCompleteAfterChunkAccepted,
		)
	}
	if generatedTusSuccessCloseSource != "after-hook-when-source-open" {
		return fmt.Errorf(
			"tus: unsupported success source-close policy %s",
			generatedTusSuccessCloseSource,
		)
	}
	if generatedTusSuccessEmit != "after-upload-complete" {
		return fmt.Errorf(
			"tus: unsupported success hook policy %s",
			generatedTusSuccessEmit,
		)
	}
	if generatedTusSuccessRemoveStoredURL != "before-hook-when-option-enabled" {
		return fmt.Errorf(
			"tus: unsupported success storage cleanup policy %s",
			generatedTusSuccessRemoveStoredURL,
		)
	}

	return nil
}

func generatedTusAssertParallelUploadPolicySupported() error {
	if generatedTusParallelUploadSplit != "contiguous-floor-size-last-remainder" {
		return fmt.Errorf(
			"tus: unsupported parallel upload split policy %s",
			generatedTusParallelUploadSplit,
		)
	}
	if generatedTusParallelPartialMetadata != "metadataForPartialUploads" {
		return fmt.Errorf(
			"tus: unsupported parallel partial metadata policy %s",
			generatedTusParallelPartialMetadata,
		)
	}
	if generatedTusParallelPartialNestedUploads != "disabled" {
		return fmt.Errorf(
			"tus: unsupported nested parallel upload policy %s",
			generatedTusParallelPartialNestedUploads,
		)
	}
	if generatedTusParallelPartialURLStorage != "parent-managed" {
		return fmt.Errorf(
			"tus: unsupported parallel URL storage policy %s",
			generatedTusParallelPartialURLStorage,
		)
	}
	if generatedTusProgressParallelPart != "aggregated-part-progress" {
		return fmt.Errorf(
			"tus: unsupported parallel progress hook policy %s",
			generatedTusProgressParallelPart,
		)
	}

	return nil
}

func generatedTusAssertAbortPolicySupported() error {
	supportedActions := map[string]bool{
		"abort-current-request":         true,
		"abort-parallel-uploads":        true,
		"clear-retry-timer":             true,
		"mark-aborted":                  true,
		"terminate-upload-if-requested": true,
	}
	for _, action := range generatedTusAbortSequence {
		if !supportedActions[action] {
			return fmt.Errorf("tus: unsupported abort sequence action %s", action)
		}
	}
	if generatedTusAbortTerminateUpload != "when-requested-and-upload-url-known" {
		return fmt.Errorf(
			"tus: unsupported abort termination policy %s",
			generatedTusAbortTerminateUpload,
		)
	}
	if generatedTusAbortRemoveStoredURLAfterTerm != "after-successful-termination" {
		return fmt.Errorf(
			"tus: unsupported abort storage cleanup policy %s",
			generatedTusAbortRemoveStoredURLAfterTerm,
		)
	}

	return nil
}

func FileFingerprint(input FileFingerprintInput) string {
	parts := make([]string, 0, len(generatedTusNodeFileFingerprintFields))
	for _, field := range generatedTusNodeFileFingerprintFields {
		switch field {
		case "prefix":
			parts = append(parts, generatedTusNodeFileFingerprintPrefix)
		case "absolutePath":
			parts = append(parts, input.AbsolutePath)
		case "size":
			parts = append(parts, strconv.FormatInt(input.Size, 10))
		case "mtimeMs":
			parts = append(parts, strconv.FormatInt(input.MtimeMs, 10))
		case "endpoint":
			parts = append(parts, input.Endpoint)
		default:
			panic(fmt.Sprintf("tus: unsupported Node file fingerprint field %s", field))
		}
	}

	return strings.Join(parts, generatedTusNodeFileFingerprintSeparator)
}

func nodeFileFingerprintPath(path string) (string, error) {
	if generatedTusNodeFileFingerprintPath != "absolute" {
		return "", fmt.Errorf("tus: unsupported Node file fingerprint path policy %s", generatedTusNodeFileFingerprintPath)
	}

	return filepath.Abs(path)
}

func fileModTimeMilliseconds(info os.FileInfo) int64 {
	return info.ModTime().UnixNano() / int64(time.Millisecond)
}

func (c *Client) resumeUploadFromURLStorage(
	options URLStorageUploadOptions,
) (*Upload, string, error) {
	storedUploads, err := options.Storage.FindUploadsByFingerprint(options.Fingerprint)
	if err != nil {
		return nil, "", err
	}

	for _, storedUpload := range storedUploads {
		location, ok := stringFromURLStorageUpload(storedUpload, "uploadUrl")
		if !ok || location == "" {
			continue
		}

		upload := &Upload{
			Location:   location,
			Metadata:   cloneStringMap(options.Metadata),
			RemoteSize: options.Size,
		}
		if _, err := c.GetUpload(upload, location); err != nil {
			return upload, "", err
		}
		if upload.RemoteSize == 0 && options.Size > 0 {
			upload.RemoteSize = options.Size
		}
		if upload.Metadata == nil {
			upload.Metadata = cloneStringMap(options.Metadata)
		}
		if err := generatedTusEmitUploadURLAvailable(options.EventHooks, "resumeUpload"); err != nil {
			return upload, "", err
		}

		storageKey, _ := stringFromURLStorageUpload(storedUpload, "urlStorageKey")
		return upload, storageKey, nil
	}

	return nil, "", nil
}

func (c *Client) createUploadForURLStorage(
	options URLStorageUploadOptions,
) (*Upload, string, error) {
	upload := &Upload{}
	if _, err := c.CreateUpload(upload, options.Size, false, options.Metadata); err != nil {
		return upload, "", err
	}
	if err := generatedTusEmitUploadURLAvailable(options.EventHooks, "createUpload"); err != nil {
		return upload, "", err
	}

	storageKey, err := options.Storage.AddUpload(
		options.Fingerprint,
		URLStorageUploadFromUpload(*upload),
	)
	if err != nil {
		return upload, "", err
	}

	return upload, storageKey, nil
}

func URLStorageUploadFromUpload(upload Upload) URLStorageUpload {
	record := URLStorageUpload{
		"creationTime": urlStorageCreationTime(),
		"metadata":     stringMapToAnyMap(upload.Metadata),
		"size":         upload.RemoteSize,
	}
	if upload.Location != "" {
		record["uploadUrl"] = upload.Location
	}

	return record
}

func urlStorageCreationTime() string {
	if generatedTusURLStorageCreationTime != "sdk-current-date-string" {
		panic(fmt.Sprintf("tus: unsupported URL storage creation time policy %s", generatedTusURLStorageCreationTime))
	}

	return time.Now().String()
}

func stringFromURLStorageUpload(upload URLStorageUpload, key string) (string, bool) {
	value, ok := upload[key]
	if !ok {
		return "", false
	}

	stringValue, ok := value.(string)
	return stringValue, ok
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}

	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}

	return result
}

func stringMapToAnyMap(input map[string]string) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}

	return result
}

func urlStorageUploadsWithPrefix(
	records map[string]URLStorageUpload,
	prefix string,
) ([]URLStorageUpload, error) {
	keys := make([]string, 0, len(records))
	for key := range records {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	result := make([]URLStorageUpload, 0, len(keys))
	for _, key := range keys {
		upload, err := cloneURLStorageUpload(records[key])
		if err != nil {
			return nil, err
		}
		upload["urlStorageKey"] = key
		result = append(result, upload)
	}

	return result, nil
}

func cloneURLStorageUpload(upload URLStorageUpload) (URLStorageUpload, error) {
	data, err := json.Marshal(upload)
	if err != nil {
		return nil, err
	}

	cloned := URLStorageUpload{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&cloned); err != nil {
		return nil, err
	}
	if cloned == nil {
		cloned = URLStorageUpload{}
	}

	return cloned, nil
}
