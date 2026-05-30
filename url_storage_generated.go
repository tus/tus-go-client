// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	generatedTusNodeFileFingerprintPath      = "absolute"
	generatedTusNodeFileFingerprintPrefix    = "node-file"
	generatedTusNodeFileFingerprintSeparator = "-"
	generatedTusRetryClientErrorStatus       = 400
	generatedTusRetryStatusCategoryDivisor   = 100
	generatedTusURLStorageIDMultiplier       = 1000000000000
	generatedTusURLStorageIDStrategy         = "rounded-random-number"
	generatedTusURLStorageNamespace          = "tus"
	generatedTusURLStorageSeparator          = "::"
	generatedTusURLStorageCreationTime       = "sdk-current-date-string"
)

var generatedTusNodeFileFingerprintFields = []string{"prefix", "absolutePath", "size", "mtimeMs", "endpoint"}
var generatedTusDefaultRetryDelays = []time.Duration{0 * time.Millisecond, 1000 * time.Millisecond, 3000 * time.Millisecond, 5000 * time.Millisecond}
var generatedTusRetryableClientStatusCodes = []int{409, 423}

type FileFingerprintInput struct {
	AbsolutePath string
	Endpoint     string
	MtimeMs      int64
	Size         int64
}

type URLStorageUpload map[string]any

type URLStorage interface {
	FindAllUploads() ([]URLStorageUpload, error)
	FindUploadsByFingerprint(fingerprint string) ([]URLStorageUpload, error)
	RemoveUpload(urlStorageKey string) error
	AddUpload(fingerprint string, upload URLStorageUpload) (string, error)
}

type URLStorageUploadOptions struct {
	Storage                    URLStorage
	Source                     io.ReadSeeker
	Fingerprint                string
	Size                       int64
	Metadata                   map[string]string
	RemoveFingerprintOnSuccess bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
}

type URLStorageFileUploadOptions struct {
	Storage                    URLStorage
	Path                       string
	Metadata                   map[string]string
	RemoveFingerprintOnSuccess bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
}

type FileBackedURLStorageUploadOptions struct {
	URLStoragePath             string
	Path                       string
	Metadata                   map[string]string
	RemoveFingerprintOnSuccess bool
	ChunkSize                  int64
	RetryDelays                []time.Duration
	OnShouldRetry              func(error, int) bool
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
		Storage:                    NewFileURLStorage(options.URLStoragePath),
		Path:                       options.Path,
		Metadata:                   options.Metadata,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		ChunkSize:                  options.ChunkSize,
		RetryDelays:                options.RetryDelays,
		OnShouldRetry:              options.OnShouldRetry,
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
		Storage:                    options.Storage,
		Source:                     file,
		Fingerprint:                fingerprint,
		Size:                       info.Size(),
		Metadata:                   options.Metadata,
		RemoveFingerprintOnSuccess: options.RemoveFingerprintOnSuccess,
		ChunkSize:                  options.ChunkSize,
		RetryDelays:                options.RetryDelays,
		OnShouldRetry:              options.OnShouldRetry,
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

	upload, storageKey, err := c.resumeUploadFromURLStorage(options)
	if err != nil {
		return upload, err
	}
	if upload == nil {
		upload, storageKey, err = c.createUploadForURLStorage(options)
		if err != nil {
			return upload, err
		}
	}

	stream := NewUploadStream(c, upload)
	if options.ChunkSize != 0 {
		stream.ChunkSize = options.ChunkSize
	}
	if err := c.uploadURLStorageSource(options, stream); err != nil {
		return upload, err
	}
	if options.RemoveFingerprintOnSuccess && storageKey != "" {
		if err := options.Storage.RemoveUpload(storageKey); err != nil {
			return upload, err
		}
	}

	return upload, nil
}

func (c *Client) uploadURLStorageSource(
	options URLStorageUploadOptions,
	stream *UploadStream,
) error {
	retryDelays := urlStorageRetryDelays(options)
	retryAttempt := 0
	offsetBeforeRetry := stream.Upload.RemoteOffset

	for {
		if _, err := options.Source.Seek(stream.Upload.RemoteOffset, io.SeekStart); err != nil {
			return err
		}
		if _, err := stream.ReadFrom(options.Source); err != nil {
			effectiveRetryAttempt := urlStorageRetryAttempt(
				stream.Upload.RemoteOffset,
				offsetBeforeRetry,
				retryAttempt,
			)
			if !urlStorageShouldScheduleRetry(options, err, stream.lastResponseStatus(), effectiveRetryAttempt, retryDelays) {
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

		return nil
	}
}

func (us *UploadStream) lastResponseStatus() int {
	if us.LastResponse == nil {
		return 0
	}

	return us.LastResponse.StatusCode
}

func urlStorageRetryDelays(options URLStorageUploadOptions) []time.Duration {
	if options.RetryDelays == nil {
		return append([]time.Duration(nil), generatedTusDefaultRetryDelays...)
	}

	return options.RetryDelays
}

func urlStorageRetryAttempt(offset int64, offsetBeforeRetry int64, retryAttempt int) int {
	if offset > offsetBeforeRetry {
		return 0
	}

	return retryAttempt
}

func urlStorageShouldScheduleRetry(
	options URLStorageUploadOptions,
	err error,
	statusCode int,
	retryAttempt int,
	retryDelays []time.Duration,
) bool {
	if retryAttempt >= len(retryDelays) || !urlStorageShouldRetryStatus(statusCode) {
		return false
	}
	if options.OnShouldRetry != nil {
		return options.OnShouldRetry(err, retryAttempt)
	}

	return true
}

func urlStorageShouldRetryStatus(statusCode int) bool {
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
