package tusgo

import (
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vitorsalgado/mocha/v3"
	"github.com/vitorsalgado/mocha/v3/expect"
	"github.com/vitorsalgado/mocha/v3/reply"
)

func generatedDefaultTusWireVersion() string {
	versions := make([]generatedTusWireVersion, 0)
	for _, candidate := range generatedTusWireVersions {
		if candidate.Default {
			versions = append(versions, candidate)
		}
	}
	Ω(versions).Should(HaveLen(1))
	return versions[0].Value
}

func generatedProtocolOperation(operationID string) generatedTusProtocolOperation {
	for _, candidate := range generatedTusProtocolOperations {
		if candidate.OperationID == operationID {
			return candidate
		}
	}
	Fail("missing generated TUS protocol operation: " + operationID)
	return generatedTusProtocolOperation{}
}

func generatedClientFeature(featureID string) generatedTusClientFeature {
	for _, candidate := range generatedTusClientFeatures {
		if candidate.FeatureID == featureID {
			return candidate
		}
	}
	Fail("missing generated TUS client feature: " + featureID)
	return generatedTusClientFeature{}
}

func generatedResponseFor(
	operation generatedTusProtocolOperation,
	statusCode int,
) generatedTusResponseContract {
	for _, candidate := range operation.Responses {
		if candidate.StatusCode == statusCode {
			return candidate
		}
	}
	Fail("missing generated response status for " + operation.OperationID)
	return generatedTusResponseContract{}
}

func withGeneratedRequestHeaders(
	builder *mocha.MockBuilder,
	operation generatedTusProtocolOperation,
	values map[string]string,
) *mocha.MockBuilder {
	variant := operation.Request.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := values[field.DisplayName]
		if value == "" {
			value = generatedDefaultTusWireVersion()
		}
		builder = builder.Header(field.DisplayName, expect.ToEqual(value))
	}
	return builder
}

func generatedResponseHeaders(
	response generatedTusResponseContract,
	overrides map[string]string,
) map[string]string {
	headers := make(map[string]string)
	variant := response.HeaderVariants[0]
	for _, field := range variant.Fields {
		if !field.Required {
			continue
		}
		value := overrides[field.DisplayName]
		if value == "" {
			value = generatedDefaultTusWireVersion()
		}
		headers[field.DisplayName] = value
	}
	return headers
}

func withGeneratedResponseHeaders(
	response *reply.StdReply,
	headers map[string]string,
) *reply.StdReply {
	for key, value := range headers {
		response = response.Header(key, value)
	}
	return response
}

var _ = Describe("generated TUS protocol contract", func() {
	var srvMock *mocha.Mocha

	BeforeEach(func() {
		srvMock = mocha.New(GinkgoT())
		srvMock.Start()
	})

	AfterEach(func() {
		if srvMock != nil {
			Ω(srvMock.Close()).Should(Succeed())
			srvMock.AssertCalled(GinkgoT())
		}
	})

	It("drives create and patch lifecycle assertions from the generated contract", func() {
		Ω(DefaultProtocolVersion).Should(Equal(generatedDefaultTusWireVersion()))

		lifecycle := generatedClientFeature("singleUploadLifecycle")
		createOperation := generatedProtocolOperation(lifecycle.OperationIDs[0])
		patchOperation := generatedProtocolOperation(lifecycle.OperationIDs[2])

		baseURL, err := url.Parse(srvMock.URL() + createOperation.Path)
		Ω(err).Should(Succeed())
		client := NewClient(http.DefaultClient, baseURL)
		client.Capabilities = &ServerCapabilities{
			Extensions:       []string{"creation"},
			ProtocolVersions: []string{DefaultProtocolVersion},
		}

		createResponse := generatedResponseFor(createOperation, http.StatusCreated)
		createReply := withGeneratedResponseHeaders(
			reply.Status(createResponse.StatusCode),
			generatedResponseHeaders(createResponse, map[string]string{
				"Location": srvMock.URL() + "/resumable/files/generated-contract",
			}),
		)
		createRequest := withGeneratedRequestHeaders(
			mocha.Request().URL(expect.URLPath(createOperation.Path)).Method(createOperation.Method),
			createOperation,
			map[string]string{
				"Upload-Length":   "5",
				"Upload-Metadata": "filename aGVsbG8udHh0",
			},
		)
		srvMock.AddMocks(createRequest.Reply(createReply))

		patchResponse := generatedResponseFor(patchOperation, http.StatusNoContent)
		patchReply := withGeneratedResponseHeaders(
			reply.Status(patchResponse.StatusCode),
			generatedResponseHeaders(patchResponse, map[string]string{
				"Upload-Offset": "5",
			}),
		)
		patchRequest := withGeneratedRequestHeaders(
			mocha.Request().
				URL(expect.URLPath("/resumable/files/generated-contract")).
				Method(patchOperation.Method).
				Body(expect.ToEqual([]byte("hello"))),
			patchOperation,
			map[string]string{
				"Content-Type":  patchOperation.Request.ContentType,
				"Upload-Offset": "0",
			},
		)
		srvMock.AddMocks(patchRequest.Reply(patchReply))

		upload := Upload{}
		_, err = client.CreateUpload(&upload, 5, false, map[string]string{
			"filename": "hello.txt",
		})
		Ω(err).Should(Succeed())

		stream := NewUploadStream(client, &upload)
		stream.ChunkSize = 5
		written, err := stream.Write([]byte("hello"))
		Ω(err).Should(Succeed())
		Ω(written).Should(Equal(5))
		Ω(upload.RemoteOffset).Should(Equal(int64(5)))
	})
})
