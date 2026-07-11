package tusgo

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestGeneratedURLStorageConformance(t *testing.T) {
	for _, scenario := range generatedTusClientUrlStorageConformanceScenarios {
		scenario := scenario
		if scenario.Backend != "file-storage" {
			continue
		}

		t.Run(scenario.ScenarioID, func(t *testing.T) {
			storage := NewFileURLStorage(filepath.Join(t.TempDir(), "url-storage.json"))
			generatedAssertURLStorage(t, storage, scenario)
		})
	}
}

func generatedAssertURLStorage(
	t *testing.T,
	storage URLStorage,
	scenario generatedTusClientUrlStorageConformanceScenario,
) {
	t.Helper()

	keyRefs := map[string]string{}
	expectedUploads := map[string]URLStorageUpload{}

	for _, action := range scenario.Actions {
		switch action.Kind {
		case "assert-empty":
			actual, err := storage.FindAllUploads()
			if err != nil {
				t.Fatal(err)
			}
			if len(actual) != 0 {
				t.Fatalf("scenario %s expected empty URL storage, got %#v", scenario.ScenarioID, actual)
			}

		case "add-upload":
			key, err := storage.AddUpload(action.Fingerprint, action.Upload)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(key, action.ExpectedKeyPrefix) {
				t.Fatalf(
					"scenario %s stored %s under %s, expected prefix %s",
					scenario.ScenarioID,
					action.KeyRef,
					key,
					action.ExpectedKeyPrefix,
				)
			}
			keyRefs[action.KeyRef] = key
			expectedUpload, err := cloneURLStorageUpload(action.Upload)
			if err != nil {
				t.Fatal(err)
			}
			expectedUpload["urlStorageKey"] = key
			expectedUploads[action.KeyRef] = expectedUpload

		case "find-by-fingerprint":
			actual, err := storage.FindUploadsByFingerprint(action.Fingerprint)
			if err != nil {
				t.Fatal(err)
			}
			generatedAssertStoredUploads(
				t,
				scenario,
				action,
				actual,
				generatedExpectedUploadsForRefs(t, scenario, action.ExpectedKeyRefs, expectedUploads),
			)

		case "find-all":
			actual, err := storage.FindAllUploads()
			if err != nil {
				t.Fatal(err)
			}
			generatedAssertStoredUploads(
				t,
				scenario,
				action,
				actual,
				generatedExpectedUploadsForRefs(t, scenario, action.ExpectedKeyRefs, expectedUploads),
			)

		case "remove-upload":
			key, ok := keyRefs[action.KeyRef]
			if !ok {
				t.Fatalf("scenario %s references unknown keyRef %s", scenario.ScenarioID, action.KeyRef)
			}
			if err := storage.RemoveUpload(key); err != nil {
				t.Fatal(err)
			}
			delete(expectedUploads, action.KeyRef)

		default:
			t.Fatalf(
				"scenario %s has unsupported URL-storage action %s",
				scenario.ScenarioID,
				action.Kind,
			)
		}
	}
}

func generatedExpectedUploadsForRefs(
	t *testing.T,
	scenario generatedTusClientUrlStorageConformanceScenario,
	refs []string,
	expectedUploads map[string]URLStorageUpload,
) []URLStorageUpload {
	t.Helper()

	uploads := make([]URLStorageUpload, 0, len(refs))
	for _, ref := range refs {
		upload, ok := expectedUploads[ref]
		if !ok {
			t.Fatalf("scenario %s references unknown expected upload %s", scenario.ScenarioID, ref)
		}
		uploads = append(uploads, upload)
	}

	return uploads
}

func generatedAssertStoredUploads(
	t *testing.T,
	scenario generatedTusClientUrlStorageConformanceScenario,
	action generatedTusClientUrlStorageConformanceAction,
	actual []URLStorageUpload,
	expected []URLStorageUpload,
) {
	t.Helper()

	actual = generatedNormalizeStoredUploads(t, actual)
	expected = generatedNormalizeStoredUploads(t, expected)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf(
			"scenario %s action %s returned %#v, expected %#v",
			scenario.ScenarioID,
			action.Kind,
			actual,
			expected,
		)
	}
}

func generatedNormalizeStoredUploads(t *testing.T, uploads []URLStorageUpload) []URLStorageUpload {
	t.Helper()

	normalized := make([]URLStorageUpload, 0, len(uploads))
	for _, upload := range uploads {
		cloned, err := cloneURLStorageUpload(upload)
		if err != nil {
			t.Fatal(err)
		}
		normalized = append(normalized, cloned)
	}

	sort.Slice(normalized, func(i int, j int) bool {
		leftID := fmt.Sprint(normalized[i]["id"])
		rightID := fmt.Sprint(normalized[j]["id"])
		if leftID != rightID {
			return leftID < rightID
		}

		return fmt.Sprint(normalized[i]["urlStorageKey"]) < fmt.Sprint(normalized[j]["urlStorageKey"])
	})

	return normalized
}
