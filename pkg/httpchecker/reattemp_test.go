package httpchecker_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/ocm-agent/pkg/httpchecker"
)

// Define custom errors for testing scenarios
var (
	errTest = errors.New("a generic test error")
)

var _ = Describe("Reattempt", func() {
	var (
		functionCallCount int
		sleep             time.Duration
		attempts          int
	)

	BeforeEach(func() {
		functionCallCount = 0
		sleep = 10 * time.Millisecond

	})

	Context("when the function 'f' succeeds on the first attempt", func() {
		It("should not return an error and call the function exactly once", func() {
			attempts = 3

			err := httpchecker.Reattempt(attempts, sleep, func() error {
				functionCallCount++
				return nil //success
			})

			By("ensuring no error is returned")
			Expect(err).Should(BeNil())
			By("checking the function call count")
			Expect(functionCallCount).Should(Equal(1))
		})
	})

	Context("when the function 'f' fails and exhausts all attempts", func() {
		It("should return the last error and call the function multiple times with sleeps", func() {
			attempts = 3

			err := httpchecker.Reattempt(attempts, sleep, func() error {
				functionCallCount++
				return errTest //failure
			})

			By("ensuring the correct error is returned (the last one encountered)")
			Expect(err).Should(MatchError(errTest))
			By("checking the function call count")
			Expect(functionCallCount).Should(Equal(attempts))
		})
	})

	Context("when attempts is 1 and the function fails", func() {
		It("should call the function once and return the error without sleeping", func() {
			attempts = 1

			err := httpchecker.Reattempt(1, sleep, func() error {
				functionCallCount++
				return errTest // Simulate failure
			})

			By("ensuring the error is returned")
			Expect(err).Should(MatchError(errTest))
			By("checking the function call count, should be 1")
			Expect(functionCallCount).Should(Equal(attempts))

		})
	})

	Context("when the function 'f' fails then succeeds on a retry", func() {
		It("should return no error after succeeding and sleep appropriately", func() {
			attempts = 3

			err := httpchecker.Reattempt(attempts, sleep, func() error {
				functionCallCount++
				if functionCallCount < 2 { // Fail only on the first call
					return errTest
				}
				return nil // Succeed on the second call
			})

			By(`ensuring no error is returned after all attempts`)
			Expect(err).Should(BeNil())
			By("checking the function call count")
			Expect(functionCallCount).Should(Equal(attempts - 1))
		})
	})
})
