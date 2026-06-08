package api2devdock

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
)

type TusConformanceObservedRequest struct {
	BodySize int
	Headers  map[string]string
	Method   string
	URL      string
}

type TusConformancePlanServer struct {
	endpointOrigin *url.URL
	errs           []error
	mu             sync.Mutex
	observed       []TusConformanceObservedRequest
	requests       []interface{}
	server         *httptest.Server
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

	conformanceServer := &TusConformancePlanServer{
		endpointOrigin: endpointOrigin,
		requests:       requests,
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

func (conformanceServer *TusConformancePlanServer) AssertExhausted() error {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	if len(conformanceServer.observed) != len(conformanceServer.requests) {
		return fmt.Errorf(
			"expected %d conformance request(s), got %d",
			len(conformanceServer.requests),
			len(conformanceServer.observed),
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

	requestBodySizes := make([]int, 0, len(conformanceServer.observed))
	requestHeaders := make([]map[string]string, 0, len(conformanceServer.observed))
	requestMethods := make([]string, 0, len(conformanceServer.observed))
	requestURLs := make([]string, 0, len(conformanceServer.observed))
	for _, request := range conformanceServer.observed {
		requestBodySizes = append(requestBodySizes, request.BodySize)
		requestHeaders = append(requestHeaders, request.Headers)
		requestMethods = append(requestMethods, request.Method)
		requestURLs = append(requestURLs, request.URL)
	}

	return map[string]interface{}{
		"requestBodySizes": requestBodySizes,
		"requestCount":     len(conformanceServer.observed),
		"requestHeaders":   requestHeaders,
		"requestMethods":   requestMethods,
		"requestUrls":      requestURLs,
	}, nil
}

func (conformanceServer *TusConformancePlanServer) ServeHTTP(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	requestIndex, requestPlan, err := conformanceServer.nextRequestPlan()
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	observed, err := conformanceServer.observedRequest(requestIndex, requestPlan, request, body)
	if err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}
	conformanceServer.observeRequest(observed)

	if err := conformanceServer.validateRequest(requestIndex, requestPlan, observed); err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := conformanceServer.writeResponse(responseWriter, requestIndex, requestPlan); err != nil {
		conformanceServer.recordErr(err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
	}
}

func (conformanceServer *TusConformancePlanServer) nextRequestPlan() (
	int,
	map[string]interface{},
	error,
) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	requestIndex := len(conformanceServer.observed)
	if requestIndex >= len(conformanceServer.requests) {
		return 0, nil, fmt.Errorf("unexpected request %d", requestIndex)
	}
	requestPlan, err := ObjectValue(
		conformanceServer.requests[requestIndex],
		fmt.Sprintf("conformanceScenario.requests[%d]", requestIndex),
	)
	if err != nil {
		return 0, nil, err
	}

	return requestIndex, requestPlan, nil
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

	requestHeaders := map[string]string{}
	for name := range expectedHeaders {
		requestHeaders[name] = request.Header.Get(name)
	}

	return TusConformanceObservedRequest{
		BodySize: len(body),
		Headers:  requestHeaders,
		Method:   request.Method,
		URL:      conformanceServer.endpointOrigin.ResolveReference(request.URL).String(),
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

func (conformanceServer *TusConformancePlanServer) observeRequest(
	request TusConformanceObservedRequest,
) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	conformanceServer.observed = append(conformanceServer.observed, request)
}

func (conformanceServer *TusConformancePlanServer) recordErr(err error) {
	conformanceServer.mu.Lock()
	defer conformanceServer.mu.Unlock()

	conformanceServer.errs = append(conformanceServer.errs, err)
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
		localValue, err := conformanceServer.LocalURL(value)
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
