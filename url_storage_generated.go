// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"bytes"
	"context"
	cryptoRand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	generatedTusNodeFileFingerprintPrefix       = "node-file"
	generatedTusNodeFileFingerprintSeparator    = "-"
	generatedTusAbortTerminateRequiresRequest   = true
	generatedTusAbortTerminateRequiresUploadURL = true
	generatedTusAbortTerminateRemovesStoredURL  = true
	generatedTusDetailedCauseStringTemplate     = "Error: {message}"
	generatedTusDetailedCausedByTemplate        = ", caused by {cause}"
	generatedTusDetailedEmptyResponseBody       = ""
	generatedTusDetailedMissingValue            = "n/a"
	generatedTusDetailedRequestContextTemplate  = ", originated from request (method: {method}, url: {url}, response code: {status}, response text: {body}, request id: {requestId})"
	generatedTusCreateUploadRequestFailed       = "tus: failed to create upload"
	generatedTusUnexpectedCreateResponse        = "tus: unexpected response while creating upload"
	generatedTusCreationWithUploadExtension     = "creation-with-upload"
	generatedTusDeferredLengthExtension         = "creation-defer-length"
	generatedTusDefaultParallelUploads          = 1
	generatedTusMinimumParallelUploads          = 2
	generatedTusValidationParallelDeferred      = "tus: cannot use the `uploadLengthDeferred` option when parallelUploads is enabled"
	generatedTusValidationParallelCreateData    = "tus: cannot use the `uploadDataDuringCreation` option when parallelUploads is enabled"
	generatedTusParallelCleanupCreatedPartials  = true
	generatedTusParallelCleanupRequiresAbort    = true
	generatedTusParallelExecutionCancelOnError  = true
	generatedTusRetryClientErrorStatus          = 400
	generatedTusRetryStatusCategoryDivisor      = 100
	generatedTusRequestIDHeaderName             = "X-Request-ID"
	generatedTusSuccessCloseSourceAfterHook     = true
	generatedTusSuccessCloseSourceRequiresSrc   = true
	generatedTusSuccessEmitAfterUploadComplete  = true
	generatedTusSuccessRemoveStoredBeforeHook   = true
	generatedTusSuccessRemoveStoredRequiresOpt  = true
	generatedTusURLStorageRemoveOnSuccessEnable = true
	generatedTusURLStorageRemoveRequiresOpt     = true
	generatedTusUploadURLAvailableCreate        = "after-url-known-before-storage"
	generatedTusUploadURLAvailableParallel      = "not-emitted"
	generatedTusUploadURLAvailableResume        = "after-url-known-before-storage"
	generatedTusURLStorageIDMultiplier          = 1000000000000
	generatedTusURLStorageNamespace             = "tus"
	generatedTusURLStorageSeparator             = "::"
)

type generatedTusMethodOverride struct {
	HeaderName   string
	HeaderValue  string
	InputFlag    string
	Method       string
	OperationID  string
	SourceMethod string
}

var generatedTusMethodOverrides = []generatedTusMethodOverride{
	{
		HeaderName:   "X-HTTP-Method-Override",
		HeaderValue:  "PATCH",
		InputFlag:    "overridePatchMethod",
		Method:       "POST",
		OperationID:  "patchTusUpload",
		SourceMethod: "PATCH",
	},
}
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
	AddRequestID               bool
	Headers                    map[string]string
	Metadata                   map[string]string
	MetadataForPartialUploads  map[string]string
	OverridePatchMethod        bool
	ParallelUploads            int
	RemoveFingerprintOnSuccess bool
	TerminateUploadOnAbort     bool
	UploadDataDuringCreation   bool
	UploadLengthDeferred       bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
	EventHooks                 UploadEventHooks
}

type URLStorageFileUploadOptions struct {
	Context                    context.Context
	Storage                    URLStorage
	Path                       string
	AddRequestID               bool
	Headers                    map[string]string
	Metadata                   map[string]string
	MetadataForPartialUploads  map[string]string
	OverridePatchMethod        bool
	ParallelUploads            int
	RemoveFingerprintOnSuccess bool
	TerminateUploadOnAbort     bool
	UploadDataDuringCreation   bool
	UploadLengthDeferred       bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
	EventHooks                 UploadEventHooks
}

type FileBackedURLStorageUploadOptions struct {
	Context                    context.Context
	URLStoragePath             string
	Path                       string
	AddRequestID               bool
	Headers                    map[string]string
	Metadata                   map[string]string
	MetadataForPartialUploads  map[string]string
	OverridePatchMethod        bool
	ParallelUploads            int
	RemoveFingerprintOnSuccess bool
	TerminateUploadOnAbort     bool
	UploadDataDuringCreation   bool
	UploadLengthDeferred       bool
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

type generatedTusParallelPartInput struct {
	Bytes []byte
	Index int
	Size  int64
}

type generatedTusParallelPartResult struct {
	Err          error
	Index        int
	LastResponse *http.Response
	Size         int64
	Upload       Upload
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
		AddRequestID:               options.AddRequestID,
		Headers:                    options.Headers,
		Metadata:                   options.Metadata,
		MetadataForPartialUploads:  options.MetadataForPartialUploads,
		OverridePatchMethod:        options.OverridePatchMethod,
		ParallelUploads:            options.ParallelUploads,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		TerminateUploadOnAbort:     options.TerminateUploadOnAbort,
		UploadDataDuringCreation:   options.UploadDataDuringCreation,
		UploadLengthDeferred:       options.UploadLengthDeferred,
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
		AddRequestID:               options.AddRequestID,
		Headers:                    options.Headers,
		Metadata:                   options.Metadata,
		MetadataForPartialUploads:  options.MetadataForPartialUploads,
		OverridePatchMethod:        options.OverridePatchMethod,
		ParallelUploads:            options.ParallelUploads,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		TerminateUploadOnAbort:     options.TerminateUploadOnAbort,
		UploadDataDuringCreation:   options.UploadDataDuringCreation,
		UploadLengthDeferred:       options.UploadLengthDeferred,
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

	uploadClient := generatedTusClientWithUploadContext(c, options.Context)
	parallelUploads, err := generatedTusParallelUploadCount(options.ParallelUploads)
	if err != nil {
		return nil, err
	}
	if err := generatedTusValidateURLStorageUploadOptions(options, parallelUploads); err != nil {
		return nil, err
	}
	uploadClient, detailedErrorRecorder := generatedTusClientWithURLStorageRequestPolicy(
		uploadClient,
		options,
	)
	if parallelUploads > 1 {
		return c.uploadParallelWithURLStorage(options, uploadClient, parallelUploads)
	}

	upload, storageKey, err := uploadClient.resumeUploadFromURLStorage(options)
	if err != nil {
		return upload, err
	}
	var lastResponse *http.Response
	if upload == nil {
		upload, storageKey, lastResponse, err = uploadClient.createUploadForURLStorage(
			options,
			detailedErrorRecorder,
		)
		if err != nil {
			return upload, err
		}
	}

	stream := NewUploadStream(uploadClient, upload)
	if options.ChunkSize != 0 {
		stream.ChunkSize = options.ChunkSize
	}
	if options.UploadLengthDeferred {
		stream.SetUploadSize = true
	}
	if err := uploadClient.uploadURLStorageSource(options, stream); err != nil {
		return upload, uploadClient.generatedTusHandleURLStorageUploadAbort(
			options,
			upload,
			storageKey,
			err,
		)
	}
	if stream.LastResponse != nil {
		lastResponse = stream.LastResponse
	}
	if err := generatedTusEmitSuccess(generatedTusSuccessInput{
		EventHooks:                 options.EventHooks,
		LastResponse:               lastResponse,
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
	uploadClient *Client,
	parallelUploads int,
) (*Upload, error) {
	partSizes, err := generatedTusParallelUploadPartSizes(options.Size, parallelUploads)
	if err != nil {
		return nil, err
	}
	partInputs, err := generatedTusParallelUploadPartInputs(options.Source, partSizes)
	if err != nil {
		return nil, err
	}

	parallelCtx, cancelParallelUploads := generatedTusParallelUploadContext(options.Context)
	defer cancelParallelUploads()
	parallelClient := uploadClient.WithContext(parallelCtx)
	results := make([]generatedTusParallelPartResult, len(partInputs))
	resultCh := make(chan generatedTusParallelPartResult, len(partInputs))
	var workers sync.WaitGroup
	for _, partInput := range partInputs {
		workers.Add(1)
		go func(partInput generatedTusParallelPartInput) {
			defer workers.Done()
			result := parallelClient.uploadParallelPartWithURLStorage(options, partInput)
			if result.Err != nil && generatedTusParallelExecutionCancelOnError {
				cancelParallelUploads()
			}
			resultCh <- result
		}(partInput)
	}
	workers.Wait()
	close(resultCh)

	for result := range resultCh {
		results[result.Index] = result
	}
	if err := generatedTusParallelUploadError(results); err != nil {
		return generatedTusFirstCreatedParallelPartialUpload(results),
			uploadClient.generatedTusCleanupParallelPartialUploads(options, results, err)
	}

	partials := make([]Upload, 0, len(results))
	acceptedBytes := int64(0)
	for _, result := range results {
		acceptedBytes += result.Size
		if err := generatedTusEmitProgressAfterChunkAccepted(
			options.EventHooks,
			acceptedBytes,
			options.Size,
		); err != nil {
			return &result.Upload,
				uploadClient.generatedTusCleanupParallelPartialUploads(options, results, err)
		}
		if err := generatedTusEmitChunkCompleteAfterChunkAccepted(
			options.EventHooks,
			result.Size,
			acceptedBytes,
			options.Size,
		); err != nil {
			return &result.Upload,
				uploadClient.generatedTusCleanupParallelPartialUploads(options, results, err)
		}
		partials = append(partials, result.Upload)
	}

	finalUpload := &Upload{}
	response, err := uploadClient.ConcatenateUploads(finalUpload, partials, options.Metadata)
	if err != nil {
		return finalUpload, uploadClient.generatedTusCleanupParallelPartialUploads(
			options,
			results,
			err,
		)
	}
	if err := uploadClient.generatedTusResolveCreatedUploadLocation(finalUpload); err != nil {
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

func (c *Client) uploadParallelPartWithURLStorage(
	options URLStorageUploadOptions,
	partInput generatedTusParallelPartInput,
) generatedTusParallelPartResult {
	result := generatedTusParallelPartResult{
		Index: partInput.Index,
		Size:  partInput.Size,
	}
	partialUpload := Upload{}
	response, err := c.CreateUpload(
		&partialUpload,
		partInput.Size,
		true,
		generatedTusParallelPartialUploadMetadata(options),
	)
	result.LastResponse = response
	result.Upload = partialUpload
	if err != nil {
		result.Err = err
		return result
	}
	if err := c.generatedTusResolveCreatedUploadLocation(&partialUpload); err != nil {
		result.Err = err
		return result
	}

	stream := NewUploadStream(c, &partialUpload)
	stream.ChunkSize = partInput.Size
	written, err := stream.Write(partInput.Bytes)
	result.LastResponse = stream.LastResponse
	result.Upload = partialUpload
	if err != nil {
		result.Err = err
		return result
	}
	if int64(written) != partInput.Size {
		result.Err = fmt.Errorf(
			"tus: expected to upload %d parallel bytes, wrote %d",
			partInput.Size,
			written,
		)
	}

	return result
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
			effectiveRetryAttempt := generatedTusEffectiveRetryAttempt(
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
			retryAttempt = generatedTusNextRetryAttempt(effectiveRetryAttempt)
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

func generatedTusClientWithUploadContext(client *Client, ctx context.Context) *Client {
	if ctx == nil {
		return client
	}

	return client.WithContext(ctx)
}

func generatedTusClientWithURLStorageRequestPolicy(
	client *Client,
	options URLStorageUploadOptions,
) (*Client, *generatedTusDetailedErrorRecorder) {
	result := *client
	httpClient := http.DefaultClient
	if client.client != nil {
		httpClient = client.client
	}
	resultHTTPClient := *httpClient
	baseTransport := resultHTTPClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	detailedErrorRecorder := &generatedTusDetailedErrorRecorder{
		Base: baseTransport,
	}
	if len(options.Headers) == 0 && !options.OverridePatchMethod && !options.AddRequestID {
		resultHTTPClient.Transport = detailedErrorRecorder
	} else {
		resultHTTPClient.Transport = generatedTusURLStorageRequestPolicyTransport{
			AddRequestID:        options.AddRequestID,
			Base:                detailedErrorRecorder,
			Headers:             cloneStringMap(options.Headers),
			OverridePatchMethod: options.OverridePatchMethod,
		}
	}
	result.client = &resultHTTPClient

	return &result, detailedErrorRecorder
}

func generatedTusClientWithAbortCleanupContext(client *Client) *Client {
	return client.WithContext(context.Background())
}

type generatedTusURLStorageRequestPolicyTransport struct {
	AddRequestID        bool
	Base                http.RoundTripper
	Headers             map[string]string
	OverridePatchMethod bool
}

// DetailedError preserves the request/response context for a failed TUS request.
type DetailedError struct {
	CausingError         error
	Err                  error
	Message              string
	OriginalRequest      *http.Request
	OriginalResponse     *http.Response
	OriginalResponseBody string
}

func (err *DetailedError) Error() string {
	return err.Message
}

func (err *DetailedError) Unwrap() error {
	if err.Err != nil {
		return err.Err
	}

	return err.CausingError
}

type generatedTusDetailedErrorRecorder struct {
	Base         http.RoundTripper
	Err          error
	Request      *http.Request
	Response     *http.Response
	ResponseBody string
	mu           sync.Mutex
}

func (recorder *generatedTusDetailedErrorRecorder) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	response, err := recorder.Base.RoundTrip(request)
	if err != nil {
		recorder.record(request, nil, "", err)
		return response, err
	}
	if response == nil || response.Body == nil {
		recorder.record(request, response, "", nil)
		return response, nil
	}

	bodyBytes, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil {
		recorder.record(request, response, "", readErr)
		return response, readErr
	}

	body := string(bodyBytes)
	response.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	storedResponse := *response
	storedResponse.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	recorder.record(request, &storedResponse, body, nil)

	return response, nil
}

func (recorder *generatedTusDetailedErrorRecorder) record(
	request *http.Request,
	response *http.Response,
	body string,
	err error,
) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	recorder.Request = request.Clone(request.Context())
	recorder.Response = response
	recorder.ResponseBody = body
	recorder.Err = err
}

func (recorder *generatedTusDetailedErrorRecorder) snapshot() generatedTusDetailedErrorSnapshot {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	return generatedTusDetailedErrorSnapshot{
		Err:          recorder.Err,
		Request:      recorder.Request,
		Response:     recorder.Response,
		ResponseBody: recorder.ResponseBody,
	}
}

type generatedTusDetailedErrorSnapshot struct {
	Err          error
	Request      *http.Request
	Response     *http.Response
	ResponseBody string
}

func generatedTusFormatDetailedErrorMessage(
	template string,
	values map[string]string,
) string {
	message := template
	for name, value := range values {
		message = strings.ReplaceAll(message, "{"+name+"}", value)
	}

	return message
}

func generatedTusDetailedErrorCause(cause error) string {
	return generatedTusFormatDetailedErrorMessage(
		generatedTusDetailedCauseStringTemplate,
		map[string]string{"message": cause.Error()},
	)
}

func generatedTusDetailedErrorResponseBody(snapshot generatedTusDetailedErrorSnapshot) string {
	if snapshot.Response == nil {
		return generatedTusDetailedMissingValue
	}
	if snapshot.ResponseBody == "" {
		return generatedTusDetailedEmptyResponseBody
	}

	return snapshot.ResponseBody
}

func generatedTusDetailedErrorResponseStatus(snapshot generatedTusDetailedErrorSnapshot) string {
	if snapshot.Response == nil {
		return generatedTusDetailedMissingValue
	}

	return strconv.Itoa(snapshot.Response.StatusCode)
}

func generatedTusDetailedErrorRequestID(snapshot generatedTusDetailedErrorSnapshot) string {
	if snapshot.Request == nil {
		return generatedTusDetailedMissingValue
	}
	requestID := snapshot.Request.Header.Get(generatedTusRequestIDHeaderName)
	if requestID == "" {
		return generatedTusDetailedMissingValue
	}

	return requestID
}

func generatedTusDetailedErrorRequestMethod(snapshot generatedTusDetailedErrorSnapshot) string {
	if snapshot.Request == nil {
		return generatedTusDetailedMissingValue
	}

	return snapshot.Request.Method
}

func generatedTusDetailedErrorRequestURL(snapshot generatedTusDetailedErrorSnapshot) string {
	if snapshot.Request == nil || snapshot.Request.URL == nil {
		return generatedTusDetailedMissingValue
	}

	return snapshot.Request.URL.String()
}

func generatedTusDetailedErrorMessage(
	baseMessage string,
	snapshot generatedTusDetailedErrorSnapshot,
) string {
	message := baseMessage
	if snapshot.Err != nil {
		message += generatedTusFormatDetailedErrorMessage(
			generatedTusDetailedCausedByTemplate,
			map[string]string{"cause": generatedTusDetailedErrorCause(snapshot.Err)},
		)
	}
	message += generatedTusFormatDetailedErrorMessage(
		generatedTusDetailedRequestContextTemplate,
		map[string]string{
			"body":      generatedTusDetailedErrorResponseBody(snapshot),
			"method":    generatedTusDetailedErrorRequestMethod(snapshot),
			"requestId": generatedTusDetailedErrorRequestID(snapshot),
			"status":    generatedTusDetailedErrorResponseStatus(snapshot),
			"url":       generatedTusDetailedErrorRequestURL(snapshot),
		},
	)

	return message
}

func generatedTusCreateUploadDetailedError(
	recorder *generatedTusDetailedErrorRecorder,
	err error,
) error {
	if err == nil || recorder == nil {
		return err
	}

	snapshot := recorder.snapshot()
	if snapshot.Request == nil {
		return err
	}

	baseMessage := generatedTusUnexpectedCreateResponse
	if snapshot.Err != nil {
		baseMessage = generatedTusCreateUploadRequestFailed
	}

	return &DetailedError{
		CausingError:         snapshot.Err,
		Err:                  err,
		Message:              generatedTusDetailedErrorMessage(baseMessage, snapshot),
		OriginalRequest:      snapshot.Request,
		OriginalResponse:     snapshot.Response,
		OriginalResponseBody: snapshot.ResponseBody,
	}
}

func (transport generatedTusURLStorageRequestPolicyTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	for _, methodOverride := range generatedTusMethodOverrides {
		enabled, err := transport.methodOverrideEnabled(methodOverride)
		if err != nil {
			return nil, err
		}
		if !enabled || cloned.Method != methodOverride.SourceMethod {
			continue
		}

		cloned.Method = methodOverride.Method
		cloned.Header.Set(
			methodOverride.HeaderName,
			methodOverride.HeaderValue,
		)
		break
	}
	for key, value := range transport.Headers {
		cloned.Header.Set(key, value)
	}
	if transport.AddRequestID {
		requestID, err := generatedTusRequestID()
		if err != nil {
			return nil, err
		}
		cloned.Header.Set(
			generatedTusRequestIDHeaderName,
			requestID,
		)
	}

	return transport.Base.RoundTrip(cloned)
}

func generatedTusRequestID() (string, error) {
	var bytes [16]byte
	if _, err := cryptoRand.Read(bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:]), nil
}

func (c *Client) generatedTusResolveCreatedUploadLocation(upload *Upload) error {
	if upload == nil || upload.Location == "" {
		return nil
	}
	locationURL, err := url.Parse(upload.Location)
	if err != nil {
		return err
	}
	upload.Location = c.BaseURL.ResolveReference(locationURL).String()
	return nil
}

func (transport generatedTusURLStorageRequestPolicyTransport) methodOverrideEnabled(
	methodOverride generatedTusMethodOverride,
) (bool, error) {
	switch methodOverride.InputFlag {
	case "overridePatchMethod":
		return transport.OverridePatchMethod, nil
	default:
		return false, fmt.Errorf("tus: unsupported method override input flag %s", methodOverride.InputFlag)
	}
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

func generatedTusValidateURLStorageUploadOptions(
	options URLStorageUploadOptions,
	parallelUploads int,
) error {
	if parallelUploads <= 1 {
		return nil
	}
	if options.UploadLengthDeferred {
		return errors.New(generatedTusValidationParallelDeferred)
	}
	if options.UploadDataDuringCreation {
		return errors.New(generatedTusValidationParallelCreateData)
	}

	return nil
}

func generatedTusCreationWithUploadChunkSize(options URLStorageUploadOptions) int64 {
	if options.ChunkSize > 0 && options.ChunkSize < options.Size {
		return options.ChunkSize
	}

	return options.Size
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

func generatedTusParallelUploadPartInputs(
	source io.ReadSeeker,
	partSizes []int64,
) ([]generatedTusParallelPartInput, error) {
	partInputs := make([]generatedTusParallelPartInput, len(partSizes))
	offset := int64(0)
	for index, partSize := range partSizes {
		if _, err := source.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
		partBytes, err := readURLStorageUploadChunk(source, partSize, partSize)
		if err != nil {
			return nil, err
		}
		partInputs[index] = generatedTusParallelPartInput{
			Bytes: partBytes,
			Index: index,
			Size:  partSize,
		}
		offset += partSize
	}

	return partInputs, nil
}

func generatedTusParallelUploadContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithCancel(ctx)
}

func generatedTusParallelPartialUploadMetadata(options URLStorageUploadOptions) map[string]string {
	return cloneStringMap(options.MetadataForPartialUploads)
}

func generatedTusParallelUploadError(results []generatedTusParallelPartResult) error {
	for _, result := range results {
		if result.Err == nil || IsUploadAbortError(result.Err) {
			continue
		}

		return result.Err
	}
	for _, result := range results {
		if result.Err != nil {
			return result.Err
		}
	}

	return nil
}

func generatedTusFirstCreatedParallelPartialUpload(
	results []generatedTusParallelPartResult,
) *Upload {
	for _, result := range results {
		if result.Upload.Location == "" {
			continue
		}

		upload := result.Upload
		return &upload
	}

	return nil
}

func (c *Client) generatedTusCleanupParallelPartialUploads(
	options URLStorageUploadOptions,
	results []generatedTusParallelPartResult,
	originalErr error,
) error {
	shouldCleanup := generatedTusShouldCleanupParallelPartialUploads(
		options.TerminateUploadOnAbort,
	)
	if !shouldCleanup {
		return originalErr
	}
	cleanupClient := generatedTusClientWithAbortCleanupContext(c)

	for _, result := range results {
		if result.Upload.Location == "" {
			continue
		}
		if _, err := cleanupClient.TerminateUploadWithRetry(result.Upload, TerminateUploadOptions{
			RetryDelays:   options.RetryDelays,
			OnShouldRetry: options.OnShouldRetry,
		}); err != nil {
			return err
		}
	}

	return originalErr
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
	shouldTerminate := generatedTusShouldTerminateKnownUploadOnAbort(
		options.TerminateUploadOnAbort,
		upload,
	)
	if !shouldTerminate {
		return err
	}

	cleanupClient := generatedTusClientWithAbortCleanupContext(c)

	if _, terminateErr := cleanupClient.TerminateUploadWithRetry(*upload, TerminateUploadOptions{
		RetryDelays:   options.RetryDelays,
		OnShouldRetry: options.OnShouldRetry,
	}); terminateErr != nil {
		return terminateErr
	}
	if generatedTusAbortTerminateRemovesStoredURL && storageKey != "" {
		if err := options.Storage.RemoveUpload(storageKey); err != nil {
			return err
		}
	}

	return err
}

func generatedTusShouldTerminateKnownUploadOnAbort(
	terminateUploadOnAbort bool,
	upload *Upload,
) bool {
	if generatedTusAbortTerminateRequiresRequest && !terminateUploadOnAbort {
		return false
	}
	if generatedTusAbortTerminateRequiresUploadURL && (upload == nil || upload.Location == "") {
		return false
	}
	if upload == nil || upload.Location == "" {
		return false
	}
	return true
}

func generatedTusRetryDelays(retryDelays []time.Duration) []time.Duration {
	if retryDelays == nil {
		return append([]time.Duration(nil), generatedTusDefaultRetryDelays...)
	}

	return retryDelays
}

func generatedTusEffectiveRetryAttempt(
	offset int64,
	offsetBeforeRetry int64,
	retryAttempt int,
) int {
	if offset > offsetBeforeRetry {
		return 0
	}

	return retryAttempt
}

func generatedTusNextRetryAttempt(retryAttempt int) int {
	return retryAttempt + 1
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

	return hooks.OnChunkComplete(
		chunkSize,
		bytesAccepted,
		generatedTusInt64Pointer(bytesTotal),
	)
}

func generatedTusEmitSuccess(input generatedTusSuccessInput) error {
	shouldRemoveStoredUpload := generatedTusShouldRemoveStoredUploadOnSuccess(
		input.RemoveFingerprintOnSuccess,
	)
	if shouldRemoveStoredUpload && input.StorageKey != "" {
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

	shouldCloseSource := generatedTusShouldCloseSourceOnSuccess(input.Source)
	if shouldCloseSource {
		closer, ok := input.Source.(io.Closer)
		if ok {
			return closer.Close()
		}
	}

	return nil
}

func generatedTusShouldCloseSourceOnSuccess(source io.ReadSeeker) bool {
	if !generatedTusSuccessCloseSourceAfterHook {
		return false
	}
	if generatedTusSuccessCloseSourceRequiresSrc {
		return source != nil
	}
	return true
}

func generatedTusShouldRemoveStoredUploadOnSuccess(
	removeFingerprintOnSuccess bool,
) bool {
	if !generatedTusSuccessRemoveStoredBeforeHook {
		return false
	}
	if !generatedTusURLStorageRemoveOnSuccessEnable {
		return false
	}
	if generatedTusSuccessRemoveStoredRequiresOpt || generatedTusURLStorageRemoveRequiresOpt {
		return removeFingerprintOnSuccess
	}
	return true
}

func generatedTusInt64Pointer(value int64) *int64 {
	return &value
}

func generatedTusShouldCleanupParallelPartialUploads(
	terminateUploadOnAbort bool,
) bool {
	if !generatedTusParallelCleanupCreatedPartials {
		return false
	}
	if generatedTusParallelCleanupRequiresAbort {
		return terminateUploadOnAbort
	}
	return true
}

func FileFingerprint(input FileFingerprintInput) string {
	parts := []string{
		generatedTusNodeFileFingerprintPrefix,
		input.AbsolutePath,
		strconv.FormatInt(input.Size, 10),
		strconv.FormatInt(input.MtimeMs, 10),
		input.Endpoint,
	}

	return strings.Join(parts, generatedTusNodeFileFingerprintSeparator)
}

func nodeFileFingerprintPath(path string) (string, error) {
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
	detailedErrorRecorder *generatedTusDetailedErrorRecorder,
) (*Upload, string, *http.Response, error) {
	if options.UploadDataDuringCreation {
		return c.createUploadWithDataForURLStorage(options, detailedErrorRecorder)
	}

	upload := &Upload{}
	remoteSize := options.Size
	if options.UploadLengthDeferred {
		remoteSize = SizeUnknown
	}
	response, err := c.CreateUpload(upload, remoteSize, false, options.Metadata)
	if err != nil {
		return upload, "", response, generatedTusCreateUploadDetailedError(
			detailedErrorRecorder,
			err,
		)
	}
	if err := c.generatedTusResolveCreatedUploadLocation(upload); err != nil {
		return upload, "", response, err
	}
	if options.UploadLengthDeferred {
		upload.RemoteSize = options.Size
	}
	if err := generatedTusEmitUploadURLAvailable(options.EventHooks, "createUpload"); err != nil {
		return upload, "", response, err
	}

	storageKey, err := options.Storage.AddUpload(
		options.Fingerprint,
		URLStorageUploadFromUpload(*upload),
	)
	if err != nil {
		return upload, "", response, err
	}

	return upload, storageKey, response, nil
}

func (c *Client) createUploadWithDataForURLStorage(
	options URLStorageUploadOptions,
	detailedErrorRecorder *generatedTusDetailedErrorRecorder,
) (*Upload, string, *http.Response, error) {
	if _, err := options.Source.Seek(0, io.SeekStart); err != nil {
		return nil, "", nil, err
	}
	chunkSize := generatedTusCreationWithUploadChunkSize(options)
	chunk, err := readURLStorageUploadChunk(options.Source, chunkSize, chunkSize)
	if err != nil {
		return nil, "", nil, err
	}
	if err := generatedTusEmitProgressBeforeRequestBody(
		options.EventHooks,
		0,
		options.Size,
	); err != nil {
		return nil, "", nil, err
	}

	upload := &Upload{}
	uploadedBytes, response, err := c.CreateUploadWithData(
		upload,
		chunk,
		options.Size,
		false,
		options.Metadata,
	)
	if err != nil {
		return upload, "", response, generatedTusCreateUploadDetailedError(
			detailedErrorRecorder,
			err,
		)
	}
	upload.RemoteSize = options.Size
	upload.RemoteOffset = uploadedBytes
	if err := c.generatedTusResolveCreatedUploadLocation(upload); err != nil {
		return upload, "", response, err
	}
	if err := generatedTusEmitProgressAfterChunkAccepted(
		options.EventHooks,
		uploadedBytes,
		options.Size,
	); err != nil {
		return upload, "", response, err
	}
	if err := generatedTusEmitUploadURLAvailable(options.EventHooks, "createUpload"); err != nil {
		return upload, "", response, err
	}
	if err := generatedTusEmitChunkCompleteAfterChunkAccepted(
		options.EventHooks,
		uploadedBytes,
		uploadedBytes,
		options.Size,
	); err != nil {
		return upload, "", response, err
	}

	storageKey, err := options.Storage.AddUpload(
		options.Fingerprint,
		URLStorageUploadFromUpload(*upload),
	)
	if err != nil {
		return upload, "", response, err
	}

	return upload, storageKey, response, nil
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
