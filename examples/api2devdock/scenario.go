package api2devdock

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type TerminationPlan struct {
	ExpectedVerificationStatus int
	MinimumDeleteRequestCount  int
	StopAfterAcceptedBytes     int
	VerificationMethod         string
}

type ResumePlan struct {
	ExpectedPreviousUploadCount          int
	ExpectedRemainingPreviousUploadCount int
	Fingerprint                          string
	RemoveFingerprintOnSuccess           bool
	StopAfterAcceptedBytes               int
}

type URLStorageBackendPlan struct {
	ExpectedStoredUploadKeyPrefix string
	Kind                          string
}

type RetryOffsetRecoveryResponsePlan struct {
	Method       string
	OffsetHeader string
}

type RetryOffsetRecoveryFailurePlan struct {
	Message    string
	Method     string
	Occurrence int
}

type RetryOffsetRecoveryPlan struct {
	ExpectedFailureCount         int
	ExpectedRecoveredOffset      int
	ExpectedRecoveryRequestCount int
	ExpectedRequestMethods       []string
	FailAfterResponse            RetryOffsetRecoveryFailurePlan
	RecoveryResponse             RetryOffsetRecoveryResponsePlan
}

type TusConformanceRetryDecision struct {
	Decision     bool
	RetryAttempt int
}

type RequestLifecycleHooksPlan struct {
	ExpectedAfterResponseMethods     []string
	ExpectedAfterResponseStatusCodes []int
	ExpectedBeforeRequestMethods     []string
	IgnoredRequestMethods            []string
}

type UploadCallbackEventKinds struct {
	ChunkComplete      string
	Progress           string
	SourceClose        string
	Success            string
	UploadURLAvailable string
}

type UploadCallbacksPlan struct {
	AllowedExtraEventKeyPrefixes []string
	EventKeyAlternativeGroups    [][]string
	EventKinds                   UploadCallbackEventKinds
	EventKeyPartSeparator        string
	EventKeys                    []string
	EventPolicyMatching          string
}

type TusConformanceServerCapabilitiesPlan struct {
	ExtensionNames   []string
	ProtocolVersions []string
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

func StringMapValue(value interface{}, label string) (map[string]string, error) {
	rawObject, err := ObjectValue(value, label)
	if err != nil {
		return nil, err
	}

	object := map[string]string{}
	for name, rawValue := range rawObject {
		text, err := StringValue(rawValue, label+"."+name)
		if err != nil {
			return nil, err
		}
		object[name] = text
	}

	return object, nil
}

func BoolValue(value interface{}, label string) (bool, error) {
	boolean, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", label)
	}

	return boolean, nil
}

func IntValue(value interface{}, label string) (int, error) {
	number, ok := value.(float64)
	if !ok || math.Trunc(number) != number {
		return 0, fmt.Errorf("%s must be an integer", label)
	}

	return int(number), nil
}

func StringArrayValue(value interface{}, label string) ([]string, error) {
	array, err := ArrayValue(value, label)
	if err != nil {
		return nil, err
	}

	strings := make([]string, 0, len(array))
	for index, item := range array {
		text, err := StringValue(item, fmt.Sprintf("%s[%d]", label, index))
		if err != nil {
			return nil, err
		}
		strings = append(strings, text)
	}

	return strings, nil
}

func StringArrayArrayValue(value interface{}, label string) ([][]string, error) {
	array, err := ArrayValue(value, label)
	if err != nil {
		return nil, err
	}

	arrays := make([][]string, 0, len(array))
	for index, item := range array {
		strings, err := StringArrayValue(item, fmt.Sprintf("%s[%d]", label, index))
		if err != nil {
			return nil, err
		}
		arrays = append(arrays, strings)
	}

	return arrays, nil
}

func IntArrayValue(value interface{}, label string) ([]int, error) {
	array, err := ArrayValue(value, label)
	if err != nil {
		return nil, err
	}

	ints := make([]int, 0, len(array))
	for index, item := range array {
		number, err := IntValue(item, fmt.Sprintf("%s[%d]", label, index))
		if err != nil {
			return nil, err
		}
		ints = append(ints, number)
	}

	return ints, nil
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

func UploadHeaders(scenario map[string]interface{}) (map[string]string, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}

	return StringMapValue(upload["headers"], "upload.headers")
}

func UploadBodyHeadersByMethod(scenario map[string]interface{}) (map[string]map[string]string, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	rawByMethod, err := ObjectValue(upload["bodyHeadersByMethod"], "upload.bodyHeadersByMethod")
	if err != nil {
		return nil, err
	}

	byMethod := map[string]map[string]string{}
	for method, rawHeaders := range rawByMethod {
		headers, err := StringMapValue(rawHeaders, "upload.bodyHeadersByMethod."+method)
		if err != nil {
			return nil, err
		}
		byMethod[method] = headers
	}

	return byMethod, nil
}

func UploadAddRequestID(scenario map[string]interface{}) (bool, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return false, err
	}

	return BoolValue(upload["addRequestId"], "upload.addRequestId")
}

func UploadRequestIDHeaderName(scenario map[string]interface{}) (string, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return "", err
	}

	return StringValue(upload["requestIdHeaderName"], "upload.requestIdHeaderName")
}

func UploadLengthDeferred(scenario map[string]interface{}) (bool, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return false, err
	}

	return BoolValue(upload["uploadLengthDeferred"], "upload.uploadLengthDeferred")
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

func Resume(scenario map[string]interface{}) (ResumePlan, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return ResumePlan{}, err
	}
	resume, err := ObjectValue(upload["resume"], "upload.resume")
	if err != nil {
		return ResumePlan{}, err
	}
	expectedPreviousUploadCount, err := IntValue(
		resume["expectedPreviousUploadCount"],
		"upload.resume.expectedPreviousUploadCount",
	)
	if err != nil {
		return ResumePlan{}, err
	}
	expectedRemainingPreviousUploadCount, err := IntValue(
		resume["expectedRemainingPreviousUploadCount"],
		"upload.resume.expectedRemainingPreviousUploadCount",
	)
	if err != nil {
		return ResumePlan{}, err
	}
	fingerprint, err := StringValue(resume["fingerprint"], "upload.resume.fingerprint")
	if err != nil {
		return ResumePlan{}, err
	}
	removeFingerprintOnSuccess, err := BoolValue(
		resume["removeFingerprintOnSuccess"],
		"upload.resume.removeFingerprintOnSuccess",
	)
	if err != nil {
		return ResumePlan{}, err
	}
	stopAfterAcceptedBytes, err := IntValue(
		resume["stopAfterAcceptedBytes"],
		"upload.resume.stopAfterAcceptedBytes",
	)
	if err != nil {
		return ResumePlan{}, err
	}

	return ResumePlan{
		ExpectedPreviousUploadCount:          expectedPreviousUploadCount,
		ExpectedRemainingPreviousUploadCount: expectedRemainingPreviousUploadCount,
		Fingerprint:                          fingerprint,
		RemoveFingerprintOnSuccess:           removeFingerprintOnSuccess,
		StopAfterAcceptedBytes:               stopAfterAcceptedBytes,
	}, nil
}

func URLStorageBackend(scenario map[string]interface{}) (*URLStorageBackendPlan, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	rawBackend, ok := upload["urlStorageBackend"]
	if !ok {
		return nil, nil
	}
	backend, err := ObjectValue(rawBackend, "upload.urlStorageBackend")
	if err != nil {
		return nil, err
	}
	expectedStoredUploadKeyPrefix, err := StringValue(
		backend["expectedStoredUploadKeyPrefix"],
		"upload.urlStorageBackend.expectedStoredUploadKeyPrefix",
	)
	if err != nil {
		return nil, err
	}
	kind, err := StringValue(backend["kind"], "upload.urlStorageBackend.kind")
	if err != nil {
		return nil, err
	}

	return &URLStorageBackendPlan{
		ExpectedStoredUploadKeyPrefix: expectedStoredUploadKeyPrefix,
		Kind:                          kind,
	}, nil
}

func RetryDelays(scenario map[string]interface{}) ([]time.Duration, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return nil, err
	}
	retries, err := IntValue(upload["retries"], "upload.retries")
	if err != nil {
		return nil, err
	}
	if retries < 0 {
		return nil, fmt.Errorf("upload.retries must not be negative")
	}

	return make([]time.Duration, retries), nil
}

func RetryOffsetRecovery(scenario map[string]interface{}) (RetryOffsetRecoveryPlan, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	retryOffsetRecovery, err := ObjectValue(
		upload["retryOffsetRecovery"],
		"upload.retryOffsetRecovery",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	failAfterResponse, err := ObjectValue(
		retryOffsetRecovery["failAfterResponse"],
		"upload.retryOffsetRecovery.failAfterResponse",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	recoveryResponse, err := ObjectValue(
		retryOffsetRecovery["recoveryResponse"],
		"upload.retryOffsetRecovery.recoveryResponse",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}

	expectedFailureCount, err := IntValue(
		retryOffsetRecovery["expectedFailureCount"],
		"upload.retryOffsetRecovery.expectedFailureCount",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	expectedRecoveredOffset, err := IntValue(
		retryOffsetRecovery["expectedRecoveredOffset"],
		"upload.retryOffsetRecovery.expectedRecoveredOffset",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	expectedRecoveryRequestCount, err := IntValue(
		retryOffsetRecovery["expectedRecoveryRequestCount"],
		"upload.retryOffsetRecovery.expectedRecoveryRequestCount",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	expectedRequestMethods, err := StringArrayValue(
		retryOffsetRecovery["expectedRequestMethods"],
		"upload.retryOffsetRecovery.expectedRequestMethods",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	failAfterResponseMessage, err := StringValue(
		failAfterResponse["message"],
		"upload.retryOffsetRecovery.failAfterResponse.message",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	failAfterResponseMethod, err := StringValue(
		failAfterResponse["method"],
		"upload.retryOffsetRecovery.failAfterResponse.method",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	failAfterResponseOccurrence, err := IntValue(
		failAfterResponse["occurrence"],
		"upload.retryOffsetRecovery.failAfterResponse.occurrence",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	recoveryResponseMethod, err := StringValue(
		recoveryResponse["method"],
		"upload.retryOffsetRecovery.recoveryResponse.method",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}
	recoveryResponseOffsetHeader, err := StringValue(
		recoveryResponse["offsetHeader"],
		"upload.retryOffsetRecovery.recoveryResponse.offsetHeader",
	)
	if err != nil {
		return RetryOffsetRecoveryPlan{}, err
	}

	return RetryOffsetRecoveryPlan{
		ExpectedFailureCount:         expectedFailureCount,
		ExpectedRecoveredOffset:      expectedRecoveredOffset,
		ExpectedRecoveryRequestCount: expectedRecoveryRequestCount,
		ExpectedRequestMethods:       expectedRequestMethods,
		FailAfterResponse: RetryOffsetRecoveryFailurePlan{
			Message:    failAfterResponseMessage,
			Method:     failAfterResponseMethod,
			Occurrence: failAfterResponseOccurrence,
		},
		RecoveryResponse: RetryOffsetRecoveryResponsePlan{
			Method:       recoveryResponseMethod,
			OffsetHeader: recoveryResponseOffsetHeader,
		},
	}, nil
}

func RequestLifecycleHooks(scenario map[string]interface{}) (RequestLifecycleHooksPlan, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return RequestLifecycleHooksPlan{}, err
	}
	requestLifecycleHooks, err := ObjectValue(
		upload["requestLifecycleHooks"],
		"upload.requestLifecycleHooks",
	)
	if err != nil {
		return RequestLifecycleHooksPlan{}, err
	}
	expectedAfterResponseMethods, err := StringArrayValue(
		requestLifecycleHooks["expectedAfterResponseMethods"],
		"upload.requestLifecycleHooks.expectedAfterResponseMethods",
	)
	if err != nil {
		return RequestLifecycleHooksPlan{}, err
	}
	expectedAfterResponseStatusCodes, err := IntArrayValue(
		requestLifecycleHooks["expectedAfterResponseStatusCodes"],
		"upload.requestLifecycleHooks.expectedAfterResponseStatusCodes",
	)
	if err != nil {
		return RequestLifecycleHooksPlan{}, err
	}
	expectedBeforeRequestMethods, err := StringArrayValue(
		requestLifecycleHooks["expectedBeforeRequestMethods"],
		"upload.requestLifecycleHooks.expectedBeforeRequestMethods",
	)
	if err != nil {
		return RequestLifecycleHooksPlan{}, err
	}
	ignoredRequestMethods, err := StringArrayValue(
		requestLifecycleHooks["ignoredRequestMethods"],
		"upload.requestLifecycleHooks.ignoredRequestMethods",
	)
	if err != nil {
		return RequestLifecycleHooksPlan{}, err
	}

	return RequestLifecycleHooksPlan{
		ExpectedAfterResponseMethods:     expectedAfterResponseMethods,
		ExpectedAfterResponseStatusCodes: expectedAfterResponseStatusCodes,
		ExpectedBeforeRequestMethods:     expectedBeforeRequestMethods,
		IgnoredRequestMethods:            ignoredRequestMethods,
	}, nil
}

func UploadCallbacks(scenario map[string]interface{}) (UploadCallbacksPlan, error) {
	upload, err := ObjectValue(scenario["upload"], "upload")
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	uploadCallbacks, err := ObjectValue(upload["uploadCallbacks"], "upload.uploadCallbacks")
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	eventKinds, err := ObjectValue(
		uploadCallbacks["eventKinds"],
		"upload.uploadCallbacks.eventKinds",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}

	allowedExtraEventKeyPrefixes, err := StringArrayValue(
		uploadCallbacks["allowedExtraEventKeyPrefixes"],
		"upload.uploadCallbacks.allowedExtraEventKeyPrefixes",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	eventKeyAlternativeGroups, err := StringArrayArrayValue(
		uploadCallbacks["eventKeyAlternativeGroups"],
		"upload.uploadCallbacks.eventKeyAlternativeGroups",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	chunkComplete, err := StringValue(
		eventKinds["chunkComplete"],
		"upload.uploadCallbacks.eventKinds.chunkComplete",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	progress, err := StringValue(eventKinds["progress"], "upload.uploadCallbacks.eventKinds.progress")
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	sourceClose, err := StringValue(
		eventKinds["sourceClose"],
		"upload.uploadCallbacks.eventKinds.sourceClose",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	success, err := StringValue(eventKinds["success"], "upload.uploadCallbacks.eventKinds.success")
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	uploadURLAvailable, err := StringValue(
		eventKinds["uploadUrlAvailable"],
		"upload.uploadCallbacks.eventKinds.uploadUrlAvailable",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	eventKeyPartSeparator, err := StringValue(
		uploadCallbacks["eventKeyPartSeparator"],
		"upload.uploadCallbacks.eventKeyPartSeparator",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	eventKeys, err := StringArrayValue(
		uploadCallbacks["eventKeys"],
		"upload.uploadCallbacks.eventKeys",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}
	eventPolicyMatching, err := StringValue(
		uploadCallbacks["eventPolicyMatching"],
		"upload.uploadCallbacks.eventPolicyMatching",
	)
	if err != nil {
		return UploadCallbacksPlan{}, err
	}

	return UploadCallbacksPlan{
		AllowedExtraEventKeyPrefixes: allowedExtraEventKeyPrefixes,
		EventKeyAlternativeGroups:    eventKeyAlternativeGroups,
		EventKinds: UploadCallbackEventKinds{
			ChunkComplete:      chunkComplete,
			Progress:           progress,
			SourceClose:        sourceClose,
			Success:            success,
			UploadURLAvailable: uploadURLAvailable,
		},
		EventKeyPartSeparator: eventKeyPartSeparator,
		EventKeys:             eventKeys,
		EventPolicyMatching:   eventPolicyMatching,
	}, nil
}

func UploadCallbackEventKey(plan UploadCallbacksPlan, parts ...string) string {
	return strings.Join(parts, plan.EventKeyPartSeparator)
}

func UploadCallbackEventKeyNumber(value int64) string {
	return strconv.FormatInt(value, 10)
}

func UploadCallbackEventKeyTotal(value *int64) string {
	if value == nil {
		return "null"
	}

	return UploadCallbackEventKeyNumber(*value)
}

func MatchUploadCallbackEventKeys(plan UploadCallbacksPlan, actual []string) ([]string, error) {
	allowedExtraPrefixes := []string{}
	switch plan.EventPolicyMatching {
	case "exact":
	case "exact-except-allowed-extra-events":
		allowedExtraPrefixes = plan.AllowedExtraEventKeyPrefixes
	default:
		return nil, fmt.Errorf("unsupported upload callback event policy %q", plan.EventPolicyMatching)
	}

	expectedIndex := 0
	matched := []string{}
	for _, event := range actual {
		if expectedIndex < len(plan.EventKeys) &&
			uploadCallbackEventMatchesExpected(plan, expectedIndex, event) {
			matched = append(matched, plan.EventKeys[expectedIndex])
			expectedIndex += 1
			continue
		}
		if hasAllowedUploadCallbackExtraEventPrefix(event, allowedExtraPrefixes) {
			continue
		}

		return nil, fmt.Errorf(
			"upload callback events emitted unexpected extra event %q; allowed prefixes %v; expected %v, got %v",
			event,
			allowedExtraPrefixes,
			plan.EventKeys,
			actual,
		)
	}
	if expectedIndex == len(plan.EventKeys) {
		return matched, nil
	}

	return nil, fmt.Errorf(
		"upload callback events did not emit every expected non-extra event; expected %v, got %v",
		plan.EventKeys,
		actual,
	)
}

func uploadCallbackEventMatchesExpected(
	plan UploadCallbacksPlan,
	expectedIndex int,
	actual string,
) bool {
	if actual == plan.EventKeys[expectedIndex] {
		return true
	}
	if expectedIndex >= len(plan.EventKeyAlternativeGroups) {
		return false
	}

	for _, alternative := range plan.EventKeyAlternativeGroups[expectedIndex] {
		if actual == alternative {
			return true
		}
	}

	return false
}

func hasAllowedUploadCallbackExtraEventPrefix(event string, allowedExtraPrefixes []string) bool {
	for _, prefix := range allowedExtraPrefixes {
		if strings.HasPrefix(event, prefix) {
			return true
		}
	}

	return false
}

func TusConformanceScenario(scenario map[string]interface{}) (map[string]interface{}, error) {
	return ObjectValue(scenario["conformanceScenario"], "conformanceScenario")
}

func TusConformanceInputOptions(
	conformanceScenario map[string]interface{},
) (map[string]interface{}, error) {
	rawEntries, err := ArrayValue(
		conformanceScenario["inputOptionEntries"],
		"conformanceScenario.inputOptionEntries",
	)
	if err != nil {
		return nil, err
	}

	options := map[string]interface{}{}
	for index, rawEntry := range rawEntries {
		label := fmt.Sprintf("conformanceScenario.inputOptionEntries[%d]", index)
		entry, err := ObjectValue(rawEntry, label)
		if err != nil {
			return nil, err
		}
		key, err := StringValue(entry["key"], label+".key")
		if err != nil {
			return nil, err
		}
		options[key] = entry["value"]
	}

	return options, nil
}

func TusConformanceInputStringOption(
	conformanceScenario map[string]interface{},
	key string,
) (string, error) {
	options, err := TusConformanceInputOptions(conformanceScenario)
	if err != nil {
		return "", err
	}

	return StringValue(options[key], "conformanceScenario.inputOptionEntries."+key)
}

func TusConformanceInputBoolOption(
	conformanceScenario map[string]interface{},
	key string,
	defaultValue bool,
) (bool, error) {
	options, err := TusConformanceInputOptions(conformanceScenario)
	if err != nil {
		return false, err
	}
	value, ok := options[key]
	if !ok {
		return defaultValue, nil
	}

	return BoolValue(value, "conformanceScenario.inputOptionEntries."+key)
}

func TusConformanceInputIntOption(
	conformanceScenario map[string]interface{},
	key string,
) (int, error) {
	options, err := TusConformanceInputOptions(conformanceScenario)
	if err != nil {
		return 0, err
	}

	return IntValue(options[key], "conformanceScenario.inputOptionEntries."+key)
}

func TusConformanceInputStringMapOption(
	conformanceScenario map[string]interface{},
	key string,
) (map[string]string, error) {
	options, err := TusConformanceInputOptions(conformanceScenario)
	if err != nil {
		return nil, err
	}
	value, ok := options[key]
	if !ok {
		return map[string]string{}, nil
	}

	return StringMapValue(value, "conformanceScenario.inputOptionEntries."+key)
}

func TusConformanceInputSourceBytes(
	conformanceScenario map[string]interface{},
) ([]byte, error) {
	source, err := ObjectValue(
		conformanceScenario["inputSource"],
		"conformanceScenario.inputSource",
	)
	if err != nil {
		return nil, err
	}
	content, err := StringValue(source["content"], "conformanceScenario.inputSource.content")
	if err != nil {
		return nil, err
	}

	return []byte(content), nil
}

func TusConformanceInputSourceKind(conformanceScenario map[string]interface{}) (string, error) {
	source, err := ObjectValue(
		conformanceScenario["inputSource"],
		"conformanceScenario.inputSource",
	)
	if err != nil {
		return "", err
	}

	return StringValue(source["kind"], "conformanceScenario.inputSource.kind")
}

func TusConformanceScenarioWantsEvent(
	conformanceScenario map[string]interface{},
	eventKind string,
) (bool, error) {
	rawEvents, ok := conformanceScenario["events"]
	if !ok || rawEvents == nil {
		return false, nil
	}
	events, err := ArrayValue(rawEvents, "conformanceScenario.events")
	if err != nil {
		return false, err
	}

	for index, rawEvent := range events {
		label := fmt.Sprintf("conformanceScenario.events[%d]", index)
		event, err := ObjectValue(rawEvent, label)
		if err != nil {
			return false, err
		}
		kind, err := StringValue(event["kind"], label+".kind")
		if err != nil {
			return false, err
		}
		if kind == eventKind {
			return true, nil
		}
	}

	return false, nil
}

func TusConformanceRetryDecisions(
	conformanceScenario map[string]interface{},
) ([]TusConformanceRetryDecision, error) {
	rawDecisions, ok := conformanceScenario["retryDecisions"]
	if !ok || rawDecisions == nil {
		return []TusConformanceRetryDecision{}, nil
	}
	decisions, err := ArrayValue(rawDecisions, "conformanceScenario.retryDecisions")
	if err != nil {
		return nil, err
	}

	parsedDecisions := make([]TusConformanceRetryDecision, 0, len(decisions))
	for index, rawDecision := range decisions {
		label := fmt.Sprintf("conformanceScenario.retryDecisions[%d]", index)
		decision, err := ObjectValue(rawDecision, label)
		if err != nil {
			return nil, err
		}
		decisionValue, err := BoolValue(decision["decision"], label+".decision")
		if err != nil {
			return nil, err
		}
		retryAttempt, err := IntValue(decision["retryAttempt"], label+".retryAttempt")
		if err != nil {
			return nil, err
		}
		parsedDecisions = append(parsedDecisions, TusConformanceRetryDecision{
			Decision:     decisionValue,
			RetryAttempt: retryAttempt,
		})
	}

	return parsedDecisions, nil
}

func TusConformanceRuntimeAbortTerminateUpload(
	conformanceScenario map[string]interface{},
) (bool, error) {
	runtimeSetup, err := ObjectValue(
		conformanceScenario["runtimeSetup"],
		"conformanceScenario.runtimeSetup",
	)
	if err != nil {
		return false, err
	}
	abort, err := ObjectValue(runtimeSetup["abort"], "conformanceScenario.runtimeSetup.abort")
	if err != nil {
		return false, err
	}

	return BoolValue(
		abort["terminateUpload"],
		"conformanceScenario.runtimeSetup.abort.terminateUpload",
	)
}

func TusConformanceRuntimeFingerprint(
	conformanceScenario map[string]interface{},
) (string, error) {
	runtimeSetup, err := ObjectValue(
		conformanceScenario["runtimeSetup"],
		"conformanceScenario.runtimeSetup",
	)
	if err != nil {
		return "", err
	}
	fingerprint, err := ObjectValue(
		runtimeSetup["fingerprint"],
		"conformanceScenario.runtimeSetup.fingerprint",
	)
	if err != nil {
		return "", err
	}
	install, err := BoolValue(
		fingerprint["install"],
		"conformanceScenario.runtimeSetup.fingerprint.install",
	)
	if err != nil {
		return "", err
	}
	if !install {
		return "", nil
	}

	return StringValue(
		fingerprint["value"],
		"conformanceScenario.runtimeSetup.fingerprint.value",
	)
}

func TusConformanceServerCapabilities(
	conformanceScenario map[string]interface{},
) (TusConformanceServerCapabilitiesPlan, error) {
	serverCapabilities, err := ObjectValue(
		conformanceScenario["serverCapabilities"],
		"conformanceScenario.serverCapabilities",
	)
	if err != nil {
		return TusConformanceServerCapabilitiesPlan{}, err
	}
	extensionNames, err := StringArrayValue(
		serverCapabilities["extensionNames"],
		"conformanceScenario.serverCapabilities.extensionNames",
	)
	if err != nil {
		return TusConformanceServerCapabilitiesPlan{}, err
	}
	protocolVersions, err := StringArrayValue(
		serverCapabilities["protocolVersions"],
		"conformanceScenario.serverCapabilities.protocolVersions",
	)
	if err != nil {
		return TusConformanceServerCapabilitiesPlan{}, err
	}

	return TusConformanceServerCapabilitiesPlan{
		ExtensionNames:   extensionNames,
		ProtocolVersions: protocolVersions,
	}, nil
}

func TusConformanceCancelRequestIndexes(
	conformanceScenario map[string]interface{},
) ([]int, error) {
	execution, err := ObjectValue(
		conformanceScenario["execution"],
		"conformanceScenario.execution",
	)
	if err != nil {
		return nil, err
	}
	actions, err := ArrayValue(
		execution["onRequestStart"],
		"conformanceScenario.execution.onRequestStart",
	)
	if err != nil {
		return nil, err
	}

	requestIndexes := []int{}
	for index, rawAction := range actions {
		label := fmt.Sprintf("conformanceScenario.execution.onRequestStart[%d]", index)
		action, err := ObjectValue(rawAction, label)
		if err != nil {
			return nil, err
		}
		kind, err := StringValue(action["kind"], label+".kind")
		if err != nil {
			return nil, err
		}
		if kind != "cancel-upload" {
			continue
		}
		requestIndex, err := IntValue(action["requestIndex"], label+".requestIndex")
		if err != nil {
			return nil, err
		}
		requestIndexes = append(requestIndexes, requestIndex)
	}

	return requestIndexes, nil
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
