//go:build osde2e
// +build osde2e

package osde2etests

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	testResultsDirectory = "/test-run-results"
	jUnitOutputFilename  = "junit-ocm-agent-operator.xml"
)

// Test entrypoint. osde2e runs this as a test suite on test pod.
func TestOcmAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	if _, ok := os.LookupEnv("DISABLE_JUNIT_REPORT"); !ok {
		reporterConfig.JUnitReport = filepath.Join(testResultsDirectory, jUnitOutputFilename)
	}
	RunSpecs(t, "Ocm Agent Operator", suiteConfig, reporterConfig)
}
