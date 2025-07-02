package httpchecker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHttpcheckerSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HttpChecker Suite")
}
