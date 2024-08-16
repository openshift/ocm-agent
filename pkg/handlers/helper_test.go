package handlers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/alertmanager/template"

	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
)

var _ = Describe("Webhook Handler Helpers", func() {

	var (
		testAlert      template.Alert
		testFleetAlert template.Alert
	)

	BeforeEach(func() {
		testAlert = testconst.NewTestAlert(false, false)
		testFleetAlert = testconst.NewTestAlert(false, true)
	})

	Context("When checking if an alert is valid", func() {
		Context("When running in non-fleet mode", func() {
			It("should indicate a valid alert is valid", func() {
				r := isValidAlert(testAlert, false)
				Expect(r).To(BeTrue())
			})
			It("should invalidate an alert with no name", func() {
				delete(testAlert.Labels, AMLabelAlertName)
				r := isValidAlert(testAlert, false)
				Expect(r).To(BeFalse())
			})
			It("should invalidate an alert with no send_managed_notification label", func() {
				delete(testAlert.Labels, "send_managed_notification")
				r := isValidAlert(testAlert, false)
				Expect(r).To(BeFalse())
			})
			It("should invalidate an alert with no managed_notification_template label", func() {
				delete(testAlert.Labels, "managed_notification_template")
				r := isValidAlert(testAlert, false)
				Expect(r).To(BeFalse())
			})
		})
		Context("When running in fleet mode", func() {
			It("should indicate a valid alert is valid", func() {
				r := isValidAlert(testFleetAlert, true)
				Expect(r).To(BeTrue())
			})
			It("should invalidate a fleet alert with no MC label", func() {
				delete(testFleetAlert.Labels, AMLabelAlertMCID)
				r := isValidAlert(testFleetAlert, true)
				Expect(r).To(BeFalse())
			})
			It("should invalidate a fleet alert with no HC label", func() {
				delete(testFleetAlert.Labels, AMLabelAlertHCID)
				r := isValidAlert(testFleetAlert, true)
				Expect(r).To(BeFalse())
			})
		})
	})
})
