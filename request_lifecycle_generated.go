// Code generated from Transloadit API2 TUS protocol contracts; DO NOT EDIT.
// If it looks wrong, please report the issue instead of editing this file by hand;
// the source fix belongs in the protocol contract generator so all TUS clients stay in sync.

package tusgo

import (
	"fmt"
	"net/http"
)

const (
	generatedTusAfterResponseHookPolicy = "after-successful-transport-response"
	generatedTusBeforeRequestHookPolicy = "before-transport-send"
)

type RequestLifecycleHooks struct {
	BeforeRequest func(*http.Request) error
	AfterResponse func(*http.Request, *http.Response) error
}

type generatedTusRequestLifecycleHookPlan struct {
	BeforeRequestHook bool
	AfterResponseHook bool
}

type generatedTusRequestLifecycleTransport struct {
	base  http.RoundTripper
	hooks RequestLifecycleHooks
}

func (c *Client) WithRequestLifecycleHooks(hooks RequestLifecycleHooks) *Client {
	clone := *c
	httpClient := c.client
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	httpClientClone := *httpClient
	httpClientClone.Transport = generatedTusRequestLifecycleTransport{
		base:  generatedTusRequestLifecycleBaseTransport(httpClient.Transport),
		hooks: hooks,
	}
	clone.client = &httpClientClone

	return &clone
}

func (transport generatedTusRequestLifecycleTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	plan, err := generatedTusPlanRequestLifecycleHooks(transport.hooks)
	if err != nil {
		return nil, err
	}
	if plan.BeforeRequestHook {
		if err := transport.hooks.BeforeRequest(request); err != nil {
			return nil, err
		}
	}

	response, err := transport.base.RoundTrip(request)
	if err != nil {
		return response, err
	}
	if plan.AfterResponseHook {
		if err := transport.hooks.AfterResponse(request, response); err != nil {
			if response.Body != nil {
				response.Body.Close()
			}
			return nil, err
		}
	}

	return response, nil
}

func generatedTusRequestLifecycleBaseTransport(base http.RoundTripper) http.RoundTripper {
	if base != nil {
		return base
	}

	return http.DefaultTransport
}

func generatedTusPlanRequestLifecycleHooks(
	hooks RequestLifecycleHooks,
) (generatedTusRequestLifecycleHookPlan, error) {
	if err := generatedTusAssertRequestLifecyclePolicySupported(); err != nil {
		return generatedTusRequestLifecycleHookPlan{}, err
	}

	return generatedTusRequestLifecycleHookPlan{
		BeforeRequestHook: hooks.BeforeRequest != nil,
		AfterResponseHook: hooks.AfterResponse != nil,
	}, nil
}

func generatedTusAssertRequestLifecyclePolicySupported() error {
	if generatedTusBeforeRequestHookPolicy != "before-transport-send" {
		return fmt.Errorf(
			"tus: unsupported before-request hook policy %s",
			generatedTusBeforeRequestHookPolicy,
		)
	}
	if generatedTusAfterResponseHookPolicy != "after-successful-transport-response" {
		return fmt.Errorf(
			"tus: unsupported after-response hook policy %s",
			generatedTusAfterResponseHookPolicy,
		)
	}

	return nil
}
