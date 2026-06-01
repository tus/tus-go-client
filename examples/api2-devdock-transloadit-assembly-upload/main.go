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
	"strconv"
	"time"

	tusgo "github.com/bdragon300/tusgo"
)

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

func createResponseFromScenario(scenario map[string]interface{}) (map[string]interface{}, error) {
	prepared, err := objectValue(scenario["prepared"], "prepared")
	if err != nil {
		return nil, err
	}

	return objectValue(prepared["createResponse"], "prepared.createResponse")
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

func writeResult(uploadURL string) error {
	resultPath := os.Getenv("API2_SDK_EXAMPLE_RESULT")
	if resultPath == "" {
		return nil
	}

	contents, err := json.MarshalIndent(
		map[string]string{
			"uploadUrl": uploadURL,
		},
		"",
		"  ",
	)
	if err != nil {
		return err
	}

	return os.WriteFile(resultPath, append(contents, '\n'), 0o644)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	scenario, err := loadScenario()
	if err != nil {
		fail("load scenario: %v", err)
	}

	createResponse, err := createResponseFromScenario(scenario)
	if err != nil {
		fail("read prepared create response: %v", err)
	}

	uploadURL, err := uploadWithTus(ctx, scenario, createResponse)
	if err != nil {
		fail("upload: %v", err)
	}
	if err := writeResult(uploadURL); err != nil {
		fail("write result: %v", err)
	}

	scenarioID, err := stringValue(scenario["scenarioId"], "scenarioId")
	if err != nil {
		fail("read scenario id: %v", err)
	}
	fmt.Printf("Go TUS SDK devdock scenario %s uploaded to %s\n", scenarioID, uploadURL)
}
