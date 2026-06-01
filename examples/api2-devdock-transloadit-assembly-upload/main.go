//go:build api2devdock

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"time"

	tusgo "github.com/bdragon300/tusgo"
	transloadit "github.com/transloadit/go-sdk"
)

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("%s must be set", name))
	}

	return value
}

func fail(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

func loadScenario() (map[string]interface{}, error) {
	scenarioPath := os.Getenv("API2_SDK_EXAMPLE_SCENARIO")
	if scenarioPath == "" {
		scenarioPath = filepath.Join(
			"examples",
			"api2-devdock-transloadit-assembly-upload",
			"api2-scenario.json",
		)
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

func objectValue(value interface{}, label string) (map[string]interface{}, error) {
	object, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an object", label)
	}

	return object, nil
}

func arrayValue(value interface{}, label string) ([]interface{}, error) {
	array, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array", label)
	}

	return array, nil
}

func stringValue(value interface{}, label string) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", label)
	}

	return text, nil
}

func intValue(value interface{}, label string) (int, error) {
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number {
		return 0, fmt.Errorf("%s must be an integer", label)
	}

	return int(number), nil
}

func scalarString(value interface{}) string {
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

func readPath(value interface{}, pathParts []interface{}, label string) (interface{}, error) {
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
			index, err := intValue(part, label)
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

func resolveValue(
	valueSpec interface{},
	context map[string]interface{},
	label string,
) (interface{}, error) {
	spec, err := objectValue(valueSpec, label)
	if err != nil {
		return nil, err
	}
	if literal, ok := spec["value"]; ok {
		return literal, nil
	}

	source, err := objectValue(spec["source"], label+".source")
	if err != nil {
		return nil, err
	}
	root, err := stringValue(source["root"], label+".source.root")
	if err != nil {
		return nil, err
	}
	rootValue, ok := context[root]
	if !ok {
		return nil, fmt.Errorf("%s source root %q is unavailable", label, root)
	}
	pathParts, err := arrayValue(source["path"], label+".source.path")
	if err != nil {
		return nil, err
	}

	return readPath(rootValue, pathParts, label)
}

func emptyValue(value interface{}) bool {
	return value == nil || value == ""
}

func newTransloaditClient() (*transloadit.Client, string) {
	endpoint := requiredEnv("TRANSLOADIT_ENDPOINT")
	config := transloadit.DefaultConfig
	config.AuthKey = requiredEnv("TRANSLOADIT_KEY")
	config.AuthSecret = requiredEnv("TRANSLOADIT_SECRET")
	config.Endpoint = endpoint
	client := transloadit.NewClient(config)

	return &client, endpoint
}

func assemblyInfoToMap(info *transloadit.AssemblyInfo, label string) (map[string]interface{}, error) {
	serialized, err := json.Marshal(info)
	if err != nil {
		return nil, fmt.Errorf("serialize %s: %w", label, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(serialized, &result); err != nil {
		return nil, fmt.Errorf("decode %s: %w", label, err)
	}

	return result, nil
}

func createAssemblyFileCount(scenario map[string]interface{}) (int, error) {
	feature, err := objectValue(scenario["createTusAssembly"], "createTusAssembly")
	if err != nil {
		return 0, err
	}
	input, err := objectValue(feature["input"], "createTusAssembly.input")
	if err != nil {
		return 0, err
	}
	if len(input) != 1 {
		return 0, fmt.Errorf("createTusAssembly.input must contain exactly one value")
	}

	for _, value := range input {
		return intValue(value, "createTusAssembly.input value")
	}

	return 0, fmt.Errorf("createTusAssembly.input did not contain a value")
}

func createAssembly(
	ctx context.Context,
	client *transloadit.Client,
	scenario map[string]interface{},
) (*transloadit.AssemblyInfo, map[string]interface{}, error) {
	fileCount, err := createAssemblyFileCount(scenario)
	if err != nil {
		return nil, nil, err
	}

	assembly, err := client.CreateTusAssembly(ctx, fileCount)
	if err != nil {
		return nil, nil, err
	}

	response, err := assemblyInfoToMap(assembly, "createTusAssembly response")
	if err != nil {
		return nil, nil, err
	}

	if errorText, ok := response["error"].(string); ok && errorText != "" {
		return nil, nil, fmt.Errorf("create assembly returned %s: %s", errorText, response["message"])
	}

	feature, err := objectValue(scenario["createTusAssembly"], "createTusAssembly")
	if err != nil {
		return nil, nil, err
	}

	requiredResponsePaths, err := arrayValue(
		feature["requiredResponsePaths"],
		"createTusAssembly.requiredResponsePaths",
	)
	if err != nil {
		return nil, nil, err
	}
	for index, rawPath := range requiredResponsePaths {
		pathParts, err := arrayValue(rawPath, fmt.Sprintf("createTusAssembly.requiredResponsePaths[%d]", index))
		if err != nil {
			return nil, nil, err
		}
		value, err := readPath(response, pathParts, fmt.Sprintf("createTusAssembly.requiredResponsePaths[%d]", index))
		if err != nil {
			return nil, nil, err
		}
		if emptyValue(value) {
			return nil, nil, fmt.Errorf("create assembly returned an empty value at path %v", pathParts)
		}
	}

	return assembly, response, nil
}

func scenarioBytes(scenario map[string]interface{}) ([]byte, error) {
	upload, err := objectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	source, err := objectValue(upload["source"], "upload.source")
	if err != nil {
		return nil, err
	}
	kind, err := stringValue(source["kind"], "upload.source.kind")
	if err != nil {
		return nil, err
	}
	if kind != "bytes" {
		return nil, fmt.Errorf("unsupported scenario source kind %q", kind)
	}
	encoding, err := stringValue(source["encoding"], "upload.source.encoding")
	if err != nil {
		return nil, err
	}
	if encoding != "utf8" {
		return nil, fmt.Errorf("unsupported scenario source encoding %q", encoding)
	}
	value, err := stringValue(source["value"], "upload.source.value")
	if err != nil {
		return nil, err
	}

	return []byte(value), nil
}

func uploadMetadata(
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (map[string]string, error) {
	upload, err := objectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	fields, err := arrayValue(upload["metadata"], "upload.metadata")
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
		field, err := objectValue(rawField, label)
		if err != nil {
			return nil, err
		}
		name, err := stringValue(field["name"], label+".name")
		if err != nil {
			return nil, err
		}
		value, err := resolveValue(field["value"], context, label+".value")
		if err != nil {
			return nil, err
		}
		metadata[name] = scalarString(value)
	}

	return metadata, nil
}

func uploadWithTus(
	ctx context.Context,
	scenario map[string]interface{},
	createResponse map[string]interface{},
) (string, error) {
	uploadConfig, err := objectValue(scenario["upload"], "upload")
	if err != nil {
		return "", err
	}
	context := map[string]interface{}{
		"createResponse": createResponse,
		"scenario":       scenario,
	}
	endpointValue, err := resolveValue(uploadConfig["tusUrl"], context, "upload.tusUrl")
	if err != nil {
		return "", err
	}
	endpointURL, err := url.Parse(scalarString(endpointValue))
	if err != nil {
		return "", err
	}
	content, err := scenarioBytes(scenario)
	if err != nil {
		return "", err
	}
	chunkSize, err := stringValue(uploadConfig["chunkSize"], "upload.chunkSize")
	if err != nil {
		return "", err
	}
	if chunkSize != "full-file" {
		return "", fmt.Errorf("unsupported chunk size policy %q", chunkSize)
	}
	metadata, err := uploadMetadata(scenario, createResponse)
	if err != nil {
		return "", err
	}

	client := tusgo.NewClient(http.DefaultClient, endpointURL).WithContext(ctx)
	upload := tusgo.Upload{}
	if _, err := client.CreateUpload(&upload, int64(len(content)), false, metadata); err != nil {
		return "", err
	}
	if upload.Location == "" {
		return "", fmt.Errorf("created upload did not include a Location")
	}

	stream := tusgo.NewUploadStream(client, &upload)
	stream.ChunkSize = tusgo.NoChunked
	written, err := stream.Write(content)
	if err != nil {
		return "", err
	}
	if written != len(content) {
		return "", fmt.Errorf("wrote %d bytes, expected %d", written, len(content))
	}
	if upload.RemoteOffset != int64(len(content)) {
		return "", fmt.Errorf("remote offset %d, expected %d", upload.RemoteOffset, len(content))
	}

	return upload.Location, nil
}

func valueLength(value interface{}, label string) (int, error) {
	switch typed := value.(type) {
	case []interface{}:
		return len(typed), nil
	case map[string]interface{}:
		return len(typed), nil
	case string:
		return len(typed), nil
	default:
		return 0, fmt.Errorf("%s has no length", label)
	}
}

func assertionsError(
	scenario map[string]interface{},
	createResponse map[string]interface{},
	status map[string]interface{},
	uploadURL string,
) error {
	rawAssertions, err := arrayValue(scenario["assertions"], "assertions")
	if err != nil {
		return err
	}
	context := map[string]interface{}{
		"captured": map[string]interface{}{
			"uploadUrl": uploadURL,
		},
		"createResponse": createResponse,
		"scenario":       scenario,
		"status":         status,
	}

	for index, rawAssertion := range rawAssertions {
		label := fmt.Sprintf("assertions[%d]", index)
		assertion, err := objectValue(rawAssertion, label)
		if err != nil {
			return err
		}
		actual, err := resolveValue(assertion["actual"], context, label+".actual")
		if err != nil {
			return err
		}
		expected, err := resolveValue(assertion["expected"], context, label+".expected")
		if err != nil {
			return err
		}
		kind, err := stringValue(assertion["kind"], label+".kind")
		if err != nil {
			return err
		}

		switch kind {
		case "equals":
			if !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("%s expected %v, got %v", label, expected, actual)
			}
		case "length":
			actualLength, err := valueLength(actual, label+".actual")
			if err != nil {
				return err
			}
			expectedLength, err := intValue(expected, label+".expected")
			if err != nil {
				return err
			}
			if actualLength != expectedLength {
				return fmt.Errorf("%s expected length %d, got %d", label, expectedLength, actualLength)
			}
		default:
			return fmt.Errorf("%s has unsupported assertion kind %q", label, kind)
		}
	}

	return nil
}

func waitForAssembly(
	ctx context.Context,
	client *transloadit.Client,
	assembly *transloadit.AssemblyInfo,
) (map[string]interface{}, error) {
	statusInfo, err := client.WaitForAssembly(ctx, assembly)
	if err != nil {
		return nil, err
	}

	status, err := assemblyInfoToMap(statusInfo, "waitForAssembly response")
	if err != nil {
		return nil, err
	}
	if errorText, ok := status["error"].(string); ok && errorText != "" {
		return status, fmt.Errorf("assembly failed with %s: %s", errorText, status["message"])
	}

	return status, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := loadScenario()
	if err != nil {
		fail("load scenario: %v", err)
	}

	client, endpoint := newTransloaditClient()
	assembly, createResponse, err := createAssembly(ctx, client, scenario)
	if err != nil {
		fail("create assembly: %v", err)
	}

	uploadURL, err := uploadWithTus(ctx, scenario, createResponse)
	if err != nil {
		fail("upload: %v", err)
	}

	status, err := waitForAssembly(ctx, client, assembly)
	if err != nil {
		fail("wait for assembly: %v", err)
	}
	if err := assertionsError(scenario, createResponse, status, uploadURL); err != nil {
		fail("assert scenario: %v", err)
	}

	scenarioID, err := stringValue(scenario["scenarioId"], "scenarioId")
	if err != nil {
		fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s passed for %s\n", scenarioID, endpoint)
}
