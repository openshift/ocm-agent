package handlers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/alertmanager/template"

	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
)

var _ = Describe("Webhook Handler Helpers", func() {

	var (
		testAlert template.Alert
	)

	BeforeEach(func() {
		testAlert = testconst.TestAlert
	})

	Context("When checking if an alert is valid", func() {
		Context("When running in non-fleet mode", func() {
			It("should indicate a valid alert is valid", func() {
				r := isValidAlert(testconst.TestAlert, false)
				Expect(r).To(BeTrue())
			})
			It("should invalidate an alert with no name", func() {
				delete(testAlert.Labels, AMLabelAlertName)
				r := isValidAlert(testconst.TestAlert, false)
				Expect(r).To(BeFalse())
			})
			It("should invalidate an alert with no send_managed_notification label", func() {
				delete(testAlert.Labels, "send_managed_notification")
				r := isValidAlert(testconst.TestAlert, false)
				Expect(r).To(BeFalse())
			})
			It("should invalidate an alert with no managed_notification_template label", func() {
				delete(testAlert.Labels, "managed_notification_template")
				r := isValidAlert(testconst.TestAlert, false)
				Expect(r).To(BeFalse())
			})
		})
		Context("When running in fleet mode", func() {
			It("should indicate a valid alert is valid", func() {
				r := isValidAlert(testconst.TestFleetAlert, true)
				Expect(r).To(BeTrue())
			})
		})
	})
})
