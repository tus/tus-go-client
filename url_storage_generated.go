// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	generatedTusURLStorageIDMultiplier = 1000000000000
	generatedTusURLStorageIDStrategy   = "rounded-random-number"
	generatedTusURLStorageNamespace    = "tus"
	generatedTusURLStorageSeparator    = "::"
)

type URLStorageUpload map[string]any

type URLStorage interface {
	FindAllUploads() ([]URLStorageUpload, error)
	FindUploadsByFingerprint(fingerprint string) ([]URLStorageUpload, error)
	RemoveUpload(urlStorageKey string) error
	AddUpload(fingerprint string, upload URLStorageUpload) (string, error)
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
