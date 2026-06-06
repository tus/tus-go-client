package api2devdock

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

type TerminationPlan struct {
	ExpectedVerificationStatus int
	MinimumDeleteRequestCount  int
	StopAfterAcceptedBytes     int
	VerificationMethod         string
}

func Fail(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

func LoadScenario(defaultPath string) (map[string]interface{}, error) {
	scenarioPath := os.Getenv("API2_SDK_EXAMPLE_SCENARIO")
	if scenarioPath == "" {
		scenarioPath = filepath.FromSlash(defaultPath)
	}

	contents, err := os.ReadFile(scenarioPath)
	if err != nil {
		return nil, err
	}

	var scenario map[string]interface{}
	if err := json.Unmarshal(contents, &scenario); err != nil {
		return nil, err
	}

	return scenario, nil
}

func ObjectValue(value interface{}, label string) (map[string]interface{}, error) {
	object, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an object", label)
	}

	return object, nil
}

func ArrayValue(value interface{}, label string) ([]interface{}, error) {
	array, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array", label)
	}

	return array, nil
}

func StringValue(value interface{}, label string) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", label)
	}

	return text, nil
}

func IntValue(value interface{}, label string) (int, error) {
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number {
		return 0, fmt.Errorf("%s must be an integer", label)
	}

	return int(number), nil
}

func ScalarString(value interface{}) string {
	switch typed := value.(type) {
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case string:
		return typed
	default:
		serialized, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}

		return string(serialized)
	}
}

func ReadPath(value interface{}, pathParts []interface{}, label string) (interface{}, error) {
	current := value
	for _, part := range pathParts {
		if object, ok := current.(map[string]interface{}); ok {
			key, ok := part.(string)
			if !ok {
				return nil, fmt.Errorf("%s path cannot read non-string key %v from object", label, part)
			}
			next, ok := object[key]
			if !ok {
				return nil, fmt.Errorf("%s path is missing key %q", label, key)
			}
			current = next
			continue
		}

		if array, ok := current.([]interface{}); ok {
			index, err := IntValue(part, label)
			if err != nil {
				return nil, err
			}
			if index < 0 || index >= len(array) {
				return nil, fmt.Errorf("%s path index %d is out of range", label, index)
			}
			current = array[index]
			continue
		}

		return nil, fmt.Errorf("%s path cannot read %v from %v", label, part, current)
	}

	return current, nil
}

func ResolveValue(
	valueSpec interface{},
	context map[string]interface{},
	label string,
) (interface{}, error) {
	spec, err := ObjectValue(valueSpec, label)
	if err != nil {
		return nil, err
	}
	if literal, ok := spec["value"]; ok {
		return literal, nil
	}

	source, err := ObjectValue(spec["source"], label+".source")
	if err != nil {
		return nil, err
	}
	root, err := StringValue(source["root"], label+".source.root")
	if err != nil {
		return nil, err
	}
	rootValue, ok := context[root]
	if !ok {
		return nil, fmt.Errorf("%s source root %q is unavailable", label, root)
	}
	pathParts, err := ArrayValue(source["path"], label+".source.path")
	if err != nil {
		return nil, err
	}

	return ReadPath(rootValue, pathParts, label)
}

func CreateResponseFromScenario(scenario map[string]interface{}) (map[string]interface{}, error) {
	prepared, err := ObjectValue(scenario["prepared"], "prepared")
	if err != nil {
		return nil, err
	}

	return ObjectValue(prepared["createResponse"], "prepared.createResponse")
}

func ScenarioBytes(scenario map[string]interface{}) ([]byte, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	source, err := ObjectValue(upload["source"], "upload.source")
	if err != nil {
		return nil, err
	}
	kind, err := StringValue(source["kind"], "upload.source.kind")
	if err != nil {
		return nil, err
	}
	if kind != "bytes" {
		return nil, fmt.Errorf("unsupported scenario source kind %q", kind)
	}
	encoding, err := StringValue(source["encoding"], "upload.source.encoding")
	if err != nil {
		return nil, err
	}
	if encoding != "utf8" {
		return nil, fmt.Errorf("unsupported scenario source encoding %q", encoding)
	}
	value, err := StringValue(source["value"], "upload.source.value")
	if err != nil {
		return nil, err
	}

	return []byte(value), nil
}

func UploadMetadata(
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]string, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	fields, err := ArrayValue(upload["metadata"], "upload.metadata")
	if err != nil {
		return nil, err
	}

	context := map[string]interface{}{
		"createResponse": createResponse,
		"scenario":       scenario,
	}
	metadata := map[string]string{}
	for index, rawField := range fields {
		label := fmt.Sprintf("upload.metadata[%d]", index)
		field, err := ObjectValue(rawField, label)
		if err != nil {
			return nil, err
		}
		name, err := StringValue(field["name"], label+".name")
		if err != nil {
			return nil, err
		}
		value, err := ResolveValue(field["value"], context, label+".value")
		if err != nil {
			return nil, err
		}
		metadata[name] = ScalarString(value)
	}

	return metadata, nil
}

func TusURL(
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (*url.URL, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	context := map[string]interface{}{
		"createResponse": createResponse,
		"scenario":       scenario,
	}
	endpointValue, err := ResolveValue(upload["tusUrl"], context, "upload.tusUrl")
	if err != nil {
		return nil, err
	}

	return url.Parse(ScalarString(endpointValue))
}

func RequireFullFileChunkSize(scenario map[string]interface{}) error {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return err
	}
	chunkSize, err := StringValue(upload["chunkSize"], "upload.chunkSize")
	if err != nil {
		return err
	}
	if chunkSize != "full-file" {
		return fmt.Errorf("unsupported chunk size policy %q", chunkSize)
	}

	return nil
}

func FixedChunkSizeBytes(scenario map[string]interface{}) (int64, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return 0, err
	}
	chunkSize, err := ObjectValue(upload["chunkSize"], "upload.chunkSize")
	if err != nil {
		return 0, err
	}
	kind, err := StringValue(chunkSize["kind"], "upload.chunkSize.kind")
	if err != nil {
		return 0, err
	}
	if kind != "fixed-bytes" {
		return 0, fmt.Errorf("unsupported chunk size kind %q", kind)
	}
	bytes, err := IntValue(chunkSize["bytes"], "upload.chunkSize.bytes")
	if err != nil {
		return 0, err
	}
	if bytes <= 0 {
		return 0, fmt.Errorf("upload.chunkSize.bytes must be positive")
	}

	return int64(bytes), nil
}

func Termination(scenario map[string]interface{}) (TerminationPlan, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return TerminationPlan{}, err
	}
	termination, err := ObjectValue(upload["termination"], "upload.termination")
	if err != nil {
		return TerminationPlan{}, err
	}
	expectedVerificationStatus, err := IntValue(
		termination["expectedVerificationStatus"],
		"upload.termination.expectedVerificationStatus",
	)
	if err != nil {
		return TerminationPlan{}, err
	}
	minimumDeleteRequestCount, err := IntValue(
		termination["minimumDeleteRequestCount"],
		"upload.termination.minimumDeleteRequestCount",
	)
	if err != nil {
		return TerminationPlan{}, err
	}
	stopAfterAcceptedBytes, err := IntValue(
		termination["stopAfterAcceptedBytes"],
		"upload.termination.stopAfterAcceptedBytes",
	)
	if err != nil {
		return TerminationPlan{}, err
	}
	verificationMethod, err := StringValue(
		termination["verificationMethod"],
		"upload.termination.verificationMethod",
	)
	if err != nil {
		return TerminationPlan{}, err
	}

	return TerminationPlan{
		ExpectedVerificationStatus: expectedVerificationStatus,
		MinimumDeleteRequestCount:  minimumDeleteRequestCount,
		StopAfterAcceptedBytes:     stopAfterAcceptedBytes,
		VerificationMethod:         verificationMethod,
	}, nil
}

func ScenarioID(scenario map[string]interface{}) (string, error) {
	return StringValue(scenario["scenarioId"], "scenarioId")
}

func WriteResult(result map[string]interface{}) error {
	resultPath := os.Getenv("API2_SDK_EXAMPLE_RESULT")
	if resultPath == "" {
		return nil
	}

	contents, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resultPath, append(contents, '\n'), 0o644)
}
