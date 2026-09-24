package api2devdock

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

type TusConformanceObservedRequest struct {
	AbsentHeaderPresence map[string]bool
	BodySize             int
	BodyStart            *int
	Headers              map[string]string
	Method               string
	URL                  string
}

type tusConformanceRequestGate struct {
	GateID                     string
	HeldRequestIndexes         map[int]bool
	ReleaseAfterRequestIndexes []int
	Timeout                    time.Duration
}

type TusConformancePlanServer struct {
	endpointOrigin *url.URL
	errs           []error
	gates          []tusConformanceRequestGate
	mu             sync.Mutex
	observed       []*TusConformanceObservedRequest
	observedCount  int
	requests       []interface{}
	server         *httptest.Server
	sourceContent  string
}

func NewTusConformancePlanServer(
	conformanceScenario map[string]interface{},
	endpointOrigin *url.URL,
) (*TusConformancePlanServer, error) {
	requests, err := ArrayValue(
		conformanceScenario["requests"],
		"conformanceScenario.requests",
	)
	if err != nil {
		return nil, err
	}
	gates, err := conformanceServerRequestGates(conformanceScenario)
	if err != nil {
		return nil, err
	}
	sourceContent, err := conformanceInputSourceContent(conformanceScenario)
	if err != nil {
		return nil, err
	}

	conformanceServer := &TusConformancePlanServer{
		endpointOrigin: endpointOrigin,
		gates:          gates,
		observed:       make([]*TusConformanceObservedRequest, len(requests)),
		requests:       requests,
		sourceContent:  sourceContent,
	}
	conformanceServer.server = httptest.NewServer(conformanceServer)

	return conformanceServer, nil
}

func (conformanceServer *TusConformancePlanServer) Close() {
	conformanceServer.server.Close()
}

func (conformanceServer *TusConformancePlanServer) EndpointURL() (*url.URL, error) {
	localEndpointURL, err := conformanceServer.LocalURL(conformanceServer.endpointOrigin.String())
	if err != nil {
		return nil, err
	}

	return url.Parse(localEndpointURL)
}

func (conformanceServer *TusConformancePlanServer) LocalURL(canonicalURL string) (string, error) {
	parsedCanonical, err := url.Parse(canonicalURL)
	if err != nil {
		return "", err
	}
	if parsedCanonical.Scheme != conformanceServer.endpointOrigin.Scheme ||
		parsedCanonical.Host != conformanceServer.endpointOrigin.Host {
		return canonicalURL, nil
	}

	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return "", err
	}
	localURL := *parsedCanonical
	localURL.Scheme = serverURL.Scheme
	localURL.Host = serverURL.Host

	return localURL.String(), nil
}

func (conformanceServer *TusConformancePlanServer) LocalValue(canonicalValue string) (string, error) {
	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(
		canonicalValue,
		conformanceServer.endpointOrigin.Scheme+"://"+conformanceServer.endpointOrigin.Host,
		serverURL.Scheme+"://"+serverURL.Host,
	), nil
}

func (conformanceServer *TusConformancePlanServer) CanonicalURL(actualURL string) (string, error) {
	parsedActual, err := url.Parse(actualURL)
	if err != nil {
		return "", err
	}
	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return "", err
	}
	if parsedActual.Scheme != serverURL.Scheme || parsedActual.Host != serverURL.Host {
		return actualURL, nil
	}

	canonical := *parsedActual
	canonical.Scheme = conformanceServer.endpointOrigin.Scheme
	canonical.Host = conformanceServer.endpointOrigin.Host

	return canonical.String(), nil
}

func (conformanceServer *TusConformancePlanServer) CanonicalValue(actualValue string) (string, error) {
	serverURL, err := url.Parse(conformanceServer.server.URL)
	if err != nil {
		return "", err
	}

	return strings.ReplaceAll(
		actualValue,
		serverURL.Scheme+"://"+serverURL.Host,
		conformanceServer.endpointOrigin.Scheme+"://"+conformanceServer.endpointOrigin.Host,
	), nil
}

func (conformanceServer *TusConformancePlanServer) AssertExhausted() error {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	if conformanceServer.observedCount != len(conformanceServer.requests) {
		return fmt.Errorf(
			"expected %d conformance request(s), got %d",
			len(conformanceServer.requests),
			conformanceServer.observedCount,
		)
	}

	return nil
}

func (conformanceServer *TusConformancePlanServer) Result() (map[string]interface{}, error) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	if len(conformanceServer.errs) > 0 {
		return nil, conformanceServer.errs[0]
	}

	absentHeaderPresence := make([]map[string]bool, 0, len(conformanceServer.observed))
	requestBodySizes := make([]int, 0, len(conformanceServer.observed))
	requestBodyStarts := make([]interface{}, 0, len(conformanceServer.observed))
	requestHeaders := make([]map[string]string, 0, len(conformanceServer.observed))
	requestMethods := make([]string, 0, len(conformanceServer.observed))
	requestURLs := make([]string, 0, len(conformanceServer.observed))
	for _, request := range conformanceServer.observed {
		if request == nil {
			requestBodySizes = append(requestBodySizes, 0)
			requestBodyStarts = append(requestBodyStarts, nil)
			requestHeaders = append(requestHeaders, map[string]string{})
			requestMethods = append(requestMethods, "")
			requestURLs = append(requestURLs, "")
			absentHeaderPresence = append(absentHeaderPresence, map[string]bool{})
			continue
		}

		requestBodySizes = append(requestBodySizes, request.BodySize)
		if request.BodyStart == nil {
			requestBodyStarts = append(requestBodyStarts, nil)
		} else {
			requestBodyStarts = append(requestBodyStarts, *request.BodyStart)
		}
		requestHeaders = append(requestHeaders, request.Headers)
		requestMethods = append(requestMethods, request.Method)
		requestURLs = append(requestURLs, request.URL)
		absentHeaderPresence = append(absentHeaderPresence, request.AbsentHeaderPresence)
	}

	return map[string]interface{}{
		"absentHeaderPresence": absentHeaderPresence,
		"requestBodySizes":     requestBodySizes,
		"requestBodyStarts":    requestBodyStarts,
		"requestCount":         conformanceServer.observedCount,
		"requestHeaders":       requestHeaders,
		"requestMethods":       requestMethods,
		"requestUrls":          requestURLs,
	}, nil
}

func (conformanceServer *TusConformancePlanServer) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	requestIndex, requestPlan, err := conformanceServer.observeMatchingRequest(request, body)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := conformanceServer.waitForRequestGate(requestIndex); err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := conformanceServer.writeResponse(responseWriter, requestIndex, requestPlan); err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}
}

func (conformanceServer *TusConformancePlanServer) observeMatchingRequest(
	request *http.Request,
	body []byte,
) (
	int,
	map[string]interface{},
	error,
) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	mismatches := []string{}
	for requestIndex, rawRequestPlan := range conformanceServer.requests {
		if conformanceServer.observed[requestIndex] != nil {
			continue
		}
		requestPlan, err := ObjectValue(
			rawRequestPlan,
			fmt.Sprintf("conformanceScenario.requests[%d]", requestIndex),
		)
		if err != nil {
			return 0, nil, err
		}
		observed, err := conformanceServer.observedRequest(requestIndex, requestPlan, request, body)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("request %d: %v", requestIndex, err))
			continue
		}
		if err := conformanceServer.validateRequest(requestIndex, requestPlan, observed); err != nil {
			mismatches = append(mismatches, fmt.Sprintf("request %d: %v", requestIndex, err))
			continue
		}
		conformanceServer.observed[requestIndex] = &observed
		conformanceServer.observedCount += 1

		return requestIndex, requestPlan, nil
	}

	return 0, nil, fmt.Errorf(
		"unexpected request %s %s after %d observed request(s): %s",
		request.Method,
		conformanceServer.endpointOrigin.ResolveReference(request.URL).String(),
		conformanceServer.observedCount,
		strings.Join(mismatches, "; "),
	)
}

func (conformanceServer *TusConformancePlanServer) observedRequest(
	requestIndex int,
	requestPlan map[string]interface{},
	request *http.Request,
	body []byte,
) (TusConformanceObservedRequest, error) {
	expectedHeaders, err := conformanceRequestHeaders(requestIndex, requestPlan)
	if err != nil {
		return TusConformanceObservedRequest{}, err
	}
	absentHeaders, err := conformanceAbsentRequestHeaders(requestIndex, requestPlan)
	if err != nil {
		return TusConformanceObservedRequest{}, err
	}

	requestHeaders := map[string]string{}
	for name := range expectedHeaders {
		value, err := conformanceServer.CanonicalValue(request.Header.Get(name))
		if err != nil {
			return TusConformanceObservedRequest{}, err
		}
		requestHeaders[name] = value
	}
	absentHeaderPresence := map[string]bool{}
	for _, name := range absentHeaders {
		absentHeaderPresence[name] = request.Header.Get(name) != ""
	}
	bodyStart, err := conformanceServer.requestBodyStart(requestIndex, requestPlan, body)
	if err != nil {
		return TusConformanceObservedRequest{}, err
	}

	return TusConformanceObservedRequest{
		AbsentHeaderPresence: absentHeaderPresence,
		BodySize:             len(body),
		BodyStart:            bodyStart,
		Headers:              requestHeaders,
		Method:               request.Method,
		URL:                  conformanceServer.endpointOrigin.ResolveReference(request.URL).String(),
	}, nil
}

func (conformanceServer *TusConformancePlanServer) validateRequest(
	requestIndex int,
	requestPlan map[string]interface{},
	request TusConformanceObservedRequest,
) error {
	expectedURL, err := StringValue(
		requestPlan["expectedUrl"],
		fmt.Sprintf("conformanceScenario.requests[%d].expectedUrl", requestIndex),
	)
	if err != nil {
		return err
	}
	expectedMethod, err := StringValue(
		requestPlan["effectiveMethod"],
		fmt.Sprintf("conformanceScenario.requests[%d].effectiveMethod", requestIndex),
	)
	if err != nil {
		return err
	}
	if request.URL != expectedURL {
		return fmt.Errorf("request %d expected URL %s, got %s", requestIndex, expectedURL, request.URL)
	}
	if request.Method != expectedMethod {
		return fmt.Errorf(
			"request %d expected method %s, got %s",
			requestIndex,
			expectedMethod,
			request.Method,
		)
	}

	expectedHeaders, err := conformanceRequestHeaders(requestIndex, requestPlan)
	if err != nil {
		return err
	}
	for name, expectedValue := range expectedHeaders {
		if request.Headers[name] == expectedValue {
			continue
		}

		return fmt.Errorf(
			"request %d expected header %s=%q, got %q",
			requestIndex,
			name,
			expectedValue,
			request.Headers[name],
		)
	}
	absentHeaders, err := conformanceAbsentRequestHeaders(requestIndex, requestPlan)
	if err != nil {
		return err
	}
	for _, name := range absentHeaders {
		if !request.AbsentHeaderPresence[name] {
			continue
		}

		return fmt.Errorf("request %d expected header %s to be absent", requestIndex, name)
	}

	rawBodySize, ok := requestPlan["bodySize"]
	if !ok || rawBodySize == nil {
		return nil
	}
	expectedBodySize, err := IntValue(
		rawBodySize,
		fmt.Sprintf("conformanceScenario.requests[%d].bodySize", requestIndex),
	)
	if err != nil {
		return err
	}
	if request.BodySize != expectedBodySize {
		return fmt.Errorf(
			"request %d expected body size %d, got %d",
			requestIndex,
			expectedBodySize,
			request.BodySize,
		)
	}

	return nil
}

func (conformanceServer *TusConformancePlanServer) recordErr(err error) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	conformanceServer.errs = append(conformanceServer.errs, err)
}

func (conformanceServer *TusConformancePlanServer) waitForRequestGate(requestIndex int) error {
	for _, gate := range conformanceServer.gates {
		if !gate.HeldRequestIndexes[requestIndex] {
			continue
		}
		deadline := time.Now().Add(gate.Timeout)
		for {
			conformanceServer.mu.Lock()
			released := conformanceServer.requestGateReleased(gate)
			conformanceServer.mu.Unlock()
			if released {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf(
					"request %d timed out waiting for conformance gate %s",
					requestIndex,
					gate.GateID,
				)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}

func (conformanceServer *TusConformancePlanServer) requestGateReleased(
	gate tusConformanceRequestGate,
) bool {
	for _, requestIndex := range gate.ReleaseAfterRequestIndexes {
		if requestIndex < 0 || requestIndex >= len(conformanceServer.observed) {
			return false
		}
		if conformanceServer.observed[requestIndex] == nil {
			return false
		}
	}

	return true
}

func (conformanceServer *TusConformancePlanServer) writeResponse(
	responseWriter http.ResponseWriter,
	requestIndex int,
	requestPlan map[string]interface{},
) error {
	responsePlan, err := ObjectValue(
		requestPlan["response"],
		fmt.Sprintf("conformanceScenario.requests[%d].response", requestIndex),
	)
	if err != nil {
		return err
	}
	headers, err := StringMapValue(
		responsePlan["effectiveHeaders"],
		fmt.Sprintf("conformanceScenario.requests[%d].response.effectiveHeaders", requestIndex),
	)
	if err != nil {
		return err
	}
	for name, value := range headers {
		localValue, err := conformanceServer.LocalValue(value)
		if err != nil {
			return err
		}
		responseWriter.Header().Set(name, localValue)
	}
	statusCode, err := IntValue(
		responsePlan["statusCode"],
		fmt.Sprintf("conformanceScenario.requests[%d].response.statusCode", requestIndex),
	)
	if err != nil {
		return err
	}
	responseWriter.WriteHeader(statusCode)

	rawBody, ok := responsePlan["body"]
	if !ok || rawBody == nil {
		return nil
	}
	body, err := StringValue(
		rawBody,
		fmt.Sprintf("conformanceScenario.requests[%d].response.body", requestIndex),
	)
	if err != nil {
		return err
	}
	_, err = responseWriter.Write([]byte(body))

	return err
}

func conformanceRequestHeaders(
	requestIndex int,
	requestPlan map[string]interface{},
) (map[string]string, error) {
	rawHeaders, ok := requestPlan["effectiveHeaders"]
	if !ok {
		return map[string]string{}, nil
	}

	return StringMapValue(
		rawHeaders,
		fmt.Sprintf("conformanceScenario.requests[%d].effectiveHeaders", requestIndex),
	)
}

func conformanceAbsentRequestHeaders(
	requestIndex int,
	requestPlan map[string]interface{},
) ([]string, error) {
	rawHeaders, ok := requestPlan["absentHeaders"]
	if !ok {
		return []string{}, nil
	}

	return StringArrayValue(
		rawHeaders,
		fmt.Sprintf("conformanceScenario.requests[%d].absentHeaders", requestIndex),
	)
}

func conformanceInputSourceContent(conformanceScenario map[string]interface{}) (string, error) {
	rawSource, ok := conformanceScenario["inputSource"]
	if !ok || rawSource == nil {
		return "", nil
	}
	source, err := ObjectValue(rawSource, "conformanceScenario.inputSource")
	if err != nil {
		return "", err
	}
	rawContent, ok := source["content"]
	if !ok || rawContent == nil {
		return "", nil
	}

	return StringValue(rawContent, "conformanceScenario.inputSource.content")
}

func (conformanceServer *TusConformancePlanServer) requestBodyStart(
	requestIndex int,
	requestPlan map[string]interface{},
	body []byte,
) (*int, error) {
	rawBodyStart, ok := requestPlan["bodyStart"]
	if !ok || rawBodyStart == nil {
		return nil, nil
	}
	bodyStart, err := IntValue(
		rawBodyStart,
		fmt.Sprintf("conformanceScenario.requests[%d].bodyStart", requestIndex),
	)
	if err != nil {
		return nil, err
	}
	bodyEnd := bodyStart + len(body)
	if bodyStart < 0 || bodyEnd > len(conformanceServer.sourceContent) {
		return nil, fmt.Errorf(
			"request %d body range [%d:%d] exceeds source content length %d",
			requestIndex,
			bodyStart,
			bodyEnd,
			len(conformanceServer.sourceContent),
		)
	}
	expectedBody := conformanceServer.sourceContent[bodyStart:bodyEnd]
	if string(body) != expectedBody {
		return nil, fmt.Errorf(
			"request %d expected body slice %q, got %q",
			requestIndex,
			expectedBody,
			string(body),
		)
	}

	return &bodyStart, nil
}

func conformanceServerRequestGates(
	conformanceScenario map[string]interface{},
) ([]tusConformanceRequestGate, error) {
	rawExecution, ok := conformanceScenario["execution"]
	if !ok || rawExecution == nil {
		return []tusConformanceRequestGate{}, nil
	}
	execution, err := ObjectValue(rawExecution, "conformanceScenario.execution")
	if err != nil {
		return nil, err
	}
	rawGates, ok := execution["serverRequestGates"]
	if !ok || rawGates == nil {
		return []tusConformanceRequestGate{}, nil
	}
	gateItems, err := ArrayValue(rawGates, "conformanceScenario.execution.serverRequestGates")
	if err != nil {
		return nil, err
	}

	gates := make([]tusConformanceRequestGate, 0, len(gateItems))
	for index, rawGate := range gateItems {
		label := fmt.Sprintf("conformanceScenario.execution.serverRequestGates[%d]", index)
		gate, err := ObjectValue(rawGate, label)
		if err != nil {
			return nil, err
		}
		kind, err := StringValue(gate["kind"], label+".kind")
		if err != nil {
			return nil, err
		}
		if kind != "release-after-all-started" {
			return nil, fmt.Errorf("unsupported conformance server request gate kind %q", kind)
		}
		gateID, err := StringValue(gate["gateId"], label+".gateId")
		if err != nil {
			return nil, err
		}
		heldRequestIndexes, err := IntArrayValue(
			gate["heldRequestIndexes"],
			label+".heldRequestIndexes",
		)
		if err != nil {
			return nil, err
		}
		releaseAfterRequestIndexes, err := IntArrayValue(
			gate["releaseAfterRequestIndexes"],
			label+".releaseAfterRequestIndexes",
		)
		if err != nil {
			return nil, err
		}
		timeoutMs, err := IntValue(gate["timeoutMs"], label+".timeoutMs")
		if err != nil {
			return nil, err
		}
		held := map[int]bool{}
		for _, requestIndex := range heldRequestIndexes {
			held[requestIndex] = true
		}
		gates = append(gates, tusConformanceRequestGate{
			GateID:                     gateID,
			HeldRequestIndexes:         held,
			ReleaseAfterRequestIndexes: releaseAfterRequestIndexes,
			Timeout:                    time.Duration(timeoutMs) * time.Millisecond,
		})
	}

	return gates, nil
}
