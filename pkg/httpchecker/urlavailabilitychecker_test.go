package httpchecker_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/ocm-agent/pkg/httpchecker"
)

var (
	checker httpchecker.HTTPChecker
)

type MockRoundTripper struct {
	// RoundTripFunc is a function that will be called when RoundTrip is invoked.
	// You set this function in your test case to define the mock's behavior.
	RoundTripFunc func(req *http.Request) (*http.Response, error)

	// CallCount can be used to verify how many times RoundTrip was called.
	CallCount int
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.CallCount++ // Increment the counter each time it's called
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req) // Execute the function defined by the test
	}
	// Default behavior if RoundTripFunc is not set: return a 200 OK with empty body
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString("")),
		Request:    req, // It's good practice to set the request on the response
	}, nil
}

var _ = Describe("urlAvailabilityChecker", func() {
	var (
		httpClient *http.Client
		test_url   string
	)
	BeforeEach(func() {

	})

	Context("NewHTTPChecker", func() {
		It("should return a new HTTPChecker instance with a default client if none is provided", func() {

			checker := httpchecker.NewHTTPChecker(nil)
			Expect(checker).ShouldNot(BeNil())

			urlchecker, asset_ok := checker.(*httpchecker.UrlHTTPChecker)
			Expect(asset_ok).Should(BeTrue())
			Expect(urlchecker.Client.Timeout).Should(Equal(10 * time.Second))

		})

		It("should return a new HTTPChecker instance with provided client", func() {
			httpClient = &http.Client{Timeout: 30 * time.Second}
			checker := httpchecker.NewHTTPChecker(httpClient)
			Expect(checker).ShouldNot(BeNil())

			urlchecker, asset_ok := checker.(*httpchecker.UrlHTTPChecker)
			Expect(asset_ok).Should(BeTrue())
			Expect(urlchecker.Client.Timeout).Should(Equal(30 * time.Second))

		})

	})

	Context("UrlHTTPChecker.UrlAvailabilityCheck", func() {
		var (
			mockTripper *MockRoundTripper
		)
		BeforeEach(func() {
			mockTripper = &MockRoundTripper{}
			checker = httpchecker.NewHTTPChecker(&http.Client{
				Transport: mockTripper,
			})

		})
		It("it not return any error when status code >= 200 and < 300", func() {
			mockTripper.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString(""))}, nil
			}
			test_url = "http://test.com/status/200"
			err := checker.UrlAvailabilityCheck(test_url)
			Expect(err).Should(BeNil())
		})

		It("should return an error for 301 Moved Permanently", func() {
			mockTripper.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusMovedPermanently, Body: io.NopCloser(bytes.NewBufferString(""))}, nil
			}
			err := checker.UrlAvailabilityCheck("http://test.com/status/301")
			Ω(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("failed to connect to http://test.com/status/301 with http response code: 301"))
		})

		When("an underlying network/client error occurs during the GET request", func() {
			It("should return that underlying error", func() {
				expectedErr := errors.New("network unreachable")
				mockTripper.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					return nil, expectedErr // Simulate a network error
				}
				err := checker.UrlAvailabilityCheck("http://test.com/unreachable")
				Expect(err).Should(HaveOccurred())
				Expect(err).Should(MatchError(expectedErr)) // Check if it's the exact error
			})

			It("should return a timeout error if the client times out", func() {
				expectedErr := errors.New("Get \"http://test.com/timeout\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
				mockTripper.RoundTripFunc = func(req *http.Request) (*http.Response, error) {
					// Simulate a timeout occurring within the HTTP client
					return nil, expectedErr
				}
				err := checker.UrlAvailabilityCheck("http://test.com/timeout")
				Expect(err).Should(HaveOccurred())
				Expect(err).Should(MatchError(expectedErr))
			})
		})

	})
})
