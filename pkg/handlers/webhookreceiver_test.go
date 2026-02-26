package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/prometheus/alertmanager/template"
	"github.com/spf13/viper"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	k8serrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"

	"github.com/openshift/ocm-agent/pkg/config"
	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	"github.com/openshift/ocm-agent/pkg/ocm"
	webhookreceivermock "github.com/openshift/ocm-agent/pkg/ocm/mocks"
	clientmocks "github.com/openshift/ocm-agent/pkg/util/test/generated/mocks/client"
)

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func getConditions(isFiring, slSent, firingMinutesAgo, resolvedMinutesAgo, slSentMinutesAgo int) ocmagentv1alpha1.Conditions {
	conditions := ocmagentv1alpha1.Conditions{}
	nowTime := time.Now()

	if isFiring >= 0 {
		var firingStatus, resolvedStatus corev1.ConditionStatus
		if isFiring > 0 {
			firingStatus = corev1.ConditionTrue
			resolvedStatus = corev1.ConditionFalse
		} else {
			firingStatus = corev1.ConditionFalse
			resolvedStatus = corev1.ConditionTrue
		}
		conditions = append(conditions, ocmagentv1alpha1.NotificationCondition{
			Type:               ocmagentv1alpha1.ConditionAlertFiring,
			Status:             firingStatus,
			LastTransitionTime: &metav1.Time{Time: nowTime.Add(time.Duration(-firingMinutesAgo) * time.Minute)},
		})
		conditions = append(conditions, ocmagentv1alpha1.NotificationCondition{
			Type:               ocmagentv1alpha1.ConditionAlertResolved,
			Status:             resolvedStatus,
			LastTransitionTime: &metav1.Time{Time: nowTime.Add(time.Duration(-resolvedMinutesAgo) * time.Minute)},
		})
	}

	if slSent >= 0 {
		var slSentStatus corev1.ConditionStatus
		if slSent > 0 {
			slSentStatus = corev1.ConditionTrue
		} else {
			slSentStatus = corev1.ConditionFalse
		}
		conditions = append(conditions, ocmagentv1alpha1.NotificationCondition{
			Type:               ocmagentv1alpha1.ConditionServiceLogSent,
			Status:             slSentStatus,
			LastTransitionTime: &metav1.Time{Time: nowTime.Add(time.Duration(-slSentMinutesAgo) * time.Minute)},
		})
	}

	return conditions
}

func assertConditions(conditions ocmagentv1alpha1.Conditions, isFiring, slSent, firingMinutesAgo, resolvedMinutesAgo, slSentMinutesAgo int) {
	isResolved := -1
	if isFiring >= 0 {
		isResolved = 1 - isFiring
	}
	assertStatus := func(status corev1.ConditionStatus, expectedValue bool) {
		if expectedValue {
			Expect(status).To(Equal(corev1.ConditionTrue))
		} else {
			Expect(status).To(Equal(corev1.ConditionFalse))
		}
	}
	nowTime := time.Now()
	assertMinutesAgo := func(conditionTime *metav1.Time, expectedMinutesAgo int) {
		if expectedMinutesAgo < 0 {
			Expect(conditionTime).To(BeNil())
		} else {
			actualMinutesAgo := int(nowTime.Sub(conditionTime.Time).Minutes())
			Expect(actualMinutesAgo).To(Equal(expectedMinutesAgo))
		}
	}

	for _, condition := range conditions {
		switch condition.Type {
		case ocmagentv1alpha1.ConditionAlertFiring:
			Expect(isFiring >= 0).To(BeTrue())
			assertStatus(condition.Status, isFiring == 1)
			assertMinutesAgo(condition.LastTransitionTime, firingMinutesAgo)
			isFiring = -1 // to mark as found
		case ocmagentv1alpha1.ConditionAlertResolved:
			Expect(isResolved >= 0).To(BeTrue())
			assertStatus(condition.Status, isResolved == 1)
			assertMinutesAgo(condition.LastTransitionTime, resolvedMinutesAgo)
			isResolved = -1 // to mark as found
		case ocmagentv1alpha1.ConditionServiceLogSent:
			Expect(slSent >= 0).To(BeTrue())
			assertStatus(condition.Status, slSent == 1)
			assertMinutesAgo(condition.LastTransitionTime, slSentMinutesAgo)
			slSent = -1 // to mark as found
		default:
			Fail("Unknown condition type: " + string(condition.Type))
		}
	}
	Expect(isFiring).To(Equal(-1))
	Expect(isResolved).To(Equal(-1))
	Expect(slSent).To(Equal(-1))
}

var _ = Describe("Webhook Handlers", func() {

	var (
		mockCtrl               *gomock.Controller
		mockClient             *clientmocks.MockClient
		mockStatusWriter       *clientmocks.MockStatusWriter
		mockOCMClient          *webhookreceivermock.MockOCMClient
		webhookReceiverHandler *WebhookReceiverHandler
		server                 *ghttp.Server
		testAlert              template.Alert
		testAlertResolved      template.Alert
		activeServiceLog       *ocm.ServiceLog
		resolvedServiceLog     *ocm.ServiceLog
	)

	BeforeEach(func() {
		// Mock valid OCM URL in viper configuration
		viper.Set(config.OcmURL, "http://api.stage.openshift.com") // Mock a valid URL for the test
		slRefs := ""
		for k, ref := range testconst.TestNotification.References {
			if k > 0 {
				slRefs += "\",\""
			}
			slRefs += string(ref)
		}

		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		server = ghttp.NewServer()
		// //mockHTTPChecker = httpcheckermock.NewMockHTTPChecker(mockCtrl)
		mockOCMClient = webhookreceivermock.NewMockOCMClient(mockCtrl)
		webhookReceiverHandler = &WebhookReceiverHandler{
			c:   mockClient,
			ocm: mockOCMClient,
		}
		testAlert = testconst.NewTestAlert(false, false)
		testAlertResolved = testconst.NewTestAlert(true, false)
		activeServiceLog = testconst.NewTestServiceLog(
			ocm.ServiceLogActivePrefix+": "+testconst.ServiceLogSummary,
			testconst.ServiceLogActiveDesc,
			"",
			testconst.TestNotification.Severity,
			testconst.TestNotification.LogType,
			testconst.TestNotification.References)
		resolvedServiceLog = testconst.NewTestServiceLog(
			ocm.ServiceLogResolvePrefix+": "+testconst.ServiceLogSummary,
			testconst.ServiceLogResolvedDesc,
			"",
			testconst.TestNotification.Severity,
			testconst.TestNotification.LogType,
			testconst.TestNotification.References)
	})
	AfterEach(func() {
		server.Close()
	})

	Context("NewWebhookReceiverHandler", func() {
		It("should create a new Webhook Receiver Handler", func() {
			handler := NewWebhookReceiverHandler(mockClient, mockOCMClient)
			Expect(handler).ToNot(BeNil())
			Expect(handler).To(BeAssignableToTypeOf(&WebhookReceiverHandler{}))
		})
	})

	Context("AMReceiver processAMReceiver", func() {
		var r http.Request
		BeforeEach(func() {
			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		})
		It("Returns the correct AMReceiverResponse", func() {
			alert := AMReceiverData{
				Status: "foo",
			}
			response := webhookReceiverHandler.processAMReceiver(alert, r.Context())
			expect := AMReceiverResponse{
				Error:  nil,
				Code:   200,
				Status: "ok",
			}
			Expect(response.Status).Should(Equal(expect.Status))
		})
	})
	Context("AMReceiver handler post", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// add handler to the server
			server.AppendHandlers(webhookReceiverHandler.ServeHTTP)
			// Expect call *client.List(arg1, arg2, arg3) on mocked WebhookReceiverHandler
			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			// Set data to post
			postData := AMReceiverData{
				Status: "foo",
			}
			// convert AMReceiverData to json for http request
			postDataJson, _ := json.Marshal(postData)
			// post to AMReceiver handler
			resp, err = http.Post(server.URL(), "application/json", bytes.NewBuffer(postDataJson))
		})
		It("Returns the correct http status code", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusOK))
		})
		It("Returns the correct content type", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.Header.Get("Content-Type")).Should(Equal("application/json"))
		})
		It("Returns the correct response", func() {
			Expect(err).ShouldNot(HaveOccurred())
			// Set expected
			expected := AMReceiverResponse{
				Status: "ok",
				Code:   200,
				Error:  nil,
			}
			// Set response
			var response AMReceiverResponse
			_ = json.NewDecoder(resp.Body).Decode(&response)
			Expect(response).Should(Equal(expected))
		})
	})
	Context("AMReceiver handler post bad data", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// add handler to the server
			server.AppendHandlers(webhookReceiverHandler.ServeHTTP)
			// Set data to post
			postData := ""
			// convert AMReceiverData to json for http request
			postDataJson, _ := json.Marshal(postData)
			// post to AMReceiver handler
			resp, err = http.Post(server.URL(), "application/json", bytes.NewBuffer(postDataJson))
		})
		It("Returns the correct http status code", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusBadRequest))
		})
		It("Returns the correct content type", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.Header.Get("Content-Type")).Should(Equal("text/plain; charset=utf-8"))
		})
		It("Returns the correct response", func() {
			Expect(err).ShouldNot(HaveOccurred())
			body, _ := io.ReadAll(resp.Body)
			Expect(string(body)).Should(Equal("Bad request body\n"))
		})
	})
	Context("AMReceiver handler get", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// add handler to the server
			server.AppendHandlers(webhookReceiverHandler.ServeHTTP)
			resp, err = http.Get(server.URL())
		})
		It("Returns the correct http status code", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusMethodNotAllowed))
		})
		It("Returns the correct content type", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.Header.Get("Content-Type")).Should(Equal("text/plain; charset=utf-8"))
		})
		It("Returns the correct response", func() {
			Expect(err).ShouldNot(HaveOccurred())
			body, _ := io.ReadAll(resp.Body)
			Expect(string(body)).Should(Equal("Method Not Allowed\n"))
		})
	})

	Context("notificationRetriever", func() {
		var notificationRetriever *notificationRetriever
		var err error
		BeforeEach(func() {
			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).SetArg(1, *testconst.TestManagedNotificationList).Return(nil)
			notificationRetriever, err = newNotificationRetriever(mockClient, context.TODO())
		})
		It("Can be created", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(notificationRetriever).ToNot(BeNil())
		})
		It("Should return a notificationContext if one exists", func() {
			mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
				Namespace: OCMAgentNamespaceName,
				Name:      testconst.TestManagedNotification.ObjectMeta.Name,
			}, gomock.Any()).Return(nil).SetArg(2, testconst.TestManagedNotificationList.Items[0])

			notificationContext, err := notificationRetriever.retrieveNotificationContext(testconst.TestNotificationName)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(notificationContext).ToNot(BeNil())
			Expect(notificationContext.retriever).ToNot(BeNil())
			Expect(notificationContext.notification).To(Equal(&testconst.TestNotification))
			Expect(notificationContext.managedNotification).To(Equal(&testconst.TestManagedNotification))
			Expect(notificationContext.notificationRecord).To(Equal(&testconst.TestNotificationRecord))
			Expect(notificationContext.wasFiring).To(BeTrue())
		})
		It("Should return nil if there is no notification for the given name", func() {
			notificationContext, err := notificationRetriever.retrieveNotificationContext("dummy-nonexistent-test")
			Expect(err).Should(HaveOccurred())
			Expect(notificationContext).To(BeNil())
		})
	})

	Context("WebhookReceiverHandler.processAlert", func() {
		var testNotifRetriever *notificationRetriever
		BeforeEach(func() {
			testNotifRetriever = &notificationRetriever{context.TODO(), mockClient, map[string]string{testconst.TestNotificationName: testconst.TestManagedNotification.ObjectMeta.Name}}
		})
		Context("Alert is invalid", func() {
			It("Reports error if alert does not have alertname label", func() {
				delete(testAlert.Labels, "alertname")
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
			})
			It("Reports error if alert does not have managed_notification_template label", func() {
				delete(testAlert.Labels, "managed_notification_template")
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
			})
			It("Reports error if alert does not have send_managed_notification label", func() {
				delete(testAlert.Labels, "send_managed_notification")
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
			})
			It("Reports error if alert send_managed_notification label does not name a valid notification", func() {
				testAlert.Labels["managed_notification_template"] = "dummy-nonexistent-test"
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
			})
		})
		Context("Alert is valid", func() {
			var notification ocmagentv1alpha1.Notification
			var conditions ocmagentv1alpha1.Conditions
			var updatedConditions []ocmagentv1alpha1.Conditions
			var updatedConditionsError error
			BeforeEach(func() {
				notification = testconst.TestNotification
				updatedConditions = nil
				mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Namespace: OCMAgentNamespaceName,
					Name:      testconst.TestManagedNotification.ObjectMeta.Name,
				}, gomock.Any()).DoAndReturn(
					func(ctx context.Context, key client.ObjectKey, res *ocmagentv1alpha1.ManagedNotification, opts ...client.GetOption) error {
						*res = ocmagentv1alpha1.ManagedNotification{
							Spec: ocmagentv1alpha1.ManagedNotificationSpec{
								Notifications: []ocmagentv1alpha1.Notification{
									notification,
								},
							},
							Status: ocmagentv1alpha1.ManagedNotificationStatus{
								NotificationRecords: ocmagentv1alpha1.NotificationRecords{
									ocmagentv1alpha1.NotificationRecord{
										Name:                testconst.TestNotificationName,
										ServiceLogSentCount: 0,
										Conditions:          conditions,
									},
								},
							},
						}
						return nil
					}).MinTimes(1)
				mockClient.EXPECT().Status().Return(mockStatusWriter).MinTimes(1)
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
						updatedManagedNotification, _ := obj.(*ocmagentv1alpha1.ManagedNotification)
						Expect(updatedManagedNotification).ToNot(BeNil())
						Expect(updatedManagedNotification.Spec.Notifications).To(Equal([]ocmagentv1alpha1.Notification{notification}))
						updatedNotificationRecords := updatedManagedNotification.Status.NotificationRecords
						Expect(len(updatedNotificationRecords)).To(Equal(1))
						Expect(updatedNotificationRecords[0].Name).To(Equal(testconst.TestNotificationName))
						updatedConditions = append(updatedConditions, updatedNotificationRecords[0].Conditions.DeepCopy())

						return updatedConditionsError
					},
				).MinTimes(1)
			})
			It("Should send a service log when receiving a firing alert and the alert never fired before", func() {
				conditions = getConditions(-1, -1, 0, 0, 0)
				mockOCMClient.EXPECT().SendServiceLog(activeServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 1, -1, 0, 0, 0)
				assertConditions(updatedConditions[1], 1, 1, 0, 0, 0)
			})
			It("Should send a service log when receiving a firing alert and the alert was marked as resolved", func() {
				conditions = getConditions(0, 1, 90, 90, 90)
				mockOCMClient.EXPECT().SendServiceLog(activeServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 1, 1, 0, 0, 90)
				assertConditions(updatedConditions[1], 1, 1, 0, 0, 0)
			})
			It("Should send a service log only once even if 2 firing alerts are received", func() { // SREP-2079
				conditions = getConditions(0, 1, 90, 90, 90)
				mockOCMClient.EXPECT().SendServiceLog(activeServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				err = webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(3))
				assertConditions(updatedConditions[0], 1, 1, 0, 0, 90)
				assertConditions(updatedConditions[1], 1, 1, 0, 0, 0)
				assertConditions(updatedConditions[2], 1, 1, 0, 0, 0)
			})
			It("Should not resend a service log when receiving a firing alert within the resend time window", func() {
				conditions = getConditions(1, 1, 30, 30, 30)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 1, 1, 30, 0, 30)
			})
			It("Should not send a service log when receiving a firing alert within the resend time window even if the alert was marked as resolved", func() {
				conditions = getConditions(0, 1, 30, 30, 30)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 1, 1, 0, 0, 30)
			})
			It("Should not resend a service log when receiving a firing alert if the AlertResolved condition updated recently", func() { // SREP-2079
				conditions = getConditions(1, 1, 90, 0, 90)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 1, 1, 90, 0, 90)
			})
			It("Should resend a service log when receiving a firing alert if out of the resend time window and the AlertResolved condition did not update recently", func() {
				conditions = getConditions(1, 1, 90, 5, 90)
				mockOCMClient.EXPECT().SendServiceLog(activeServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 1, 1, 90, 0, 90)
				assertConditions(updatedConditions[1], 1, 1, 90, 0, 0)
			})
			It("Should resend a service log only once even if 2 firing alerts are received", func() { // SREP-2079
				conditions = getConditions(1, 1, 90, 5, 90)
				mockOCMClient.EXPECT().SendServiceLog(activeServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				err = webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(3))
				assertConditions(updatedConditions[0], 1, 1, 90, 0, 90)
				assertConditions(updatedConditions[1], 1, 1, 90, 0, 0)
				assertConditions(updatedConditions[2], 1, 1, 90, 0, 0)
			})

			It("Should send a service log when receiving an alert resolution and the alert was marked as firing", func() {
				conditions = getConditions(1, 1, 90, 30, 90)
				mockOCMClient.EXPECT().SendServiceLog(resolvedServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 0, 1, 0, 0, 90)
				assertConditions(updatedConditions[1], 0, 1, 0, 0, 0)
			})
			It("Should send a service log when receiving an alert resolution and even if the alert was marked as firing very recently", func() {
				conditions = getConditions(1, 1, 1, 1, 1)
				mockOCMClient.EXPECT().SendServiceLog(resolvedServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 0, 1, 0, 0, 1)
				assertConditions(updatedConditions[1], 0, 1, 0, 0, 0)
			})
			It("Should send a service log only once even if 2 alert resolutions are received", func() { // SREP-2079
				conditions = getConditions(1, 1, 90, 30, 90)
				mockOCMClient.EXPECT().SendServiceLog(resolvedServiceLog).Return(nil)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				err = webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(3))
				assertConditions(updatedConditions[0], 0, 1, 0, 0, 90)
				assertConditions(updatedConditions[1], 0, 1, 0, 0, 0)
				assertConditions(updatedConditions[2], 0, 1, 0, 0, 0)
			})
			It("Should not send a service log when receiving an alert resolution and the alert was not marked as firing", func() {
				conditions = getConditions(0, 1, 90, 90, 30)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 0, 1, 90, 90, 30)
			})
			It("Should not send a service log when receiving an alert resolution but the service log failed to be sent when the alert was firing", func() {
				conditions = getConditions(1, 0, 30, 30, 30)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 0, 0, 0, 0, 30)
			})
			It("Should not send a service log when receiving an alert resolution but the last service log was sent before the alert was firing", func() {
				conditions = getConditions(1, 1, 30, 30, 50)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 0, 1, 0, 0, 50)
			})

			It("Should not send a service when receiving a firing alert and some place holder cannot be resolved with an alert label or annotation", func() {
				conditions = getConditions(-1, -1, 0, 0, 0)
				testAlert.Annotations = nil
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 1, -1, 0, 0, 0)
				assertConditions(updatedConditions[1], 1, 0, 0, 0, 0)
			})
			It("Should not send a service log when receiving an alert resolution and the resolved body is empty", func() {
				notification = testconst.NotificationWithoutResolvedBody
				conditions = getConditions(1, 1, 5, 5, 5)
				err := webhookReceiverHandler.processAlert(testAlertResolved, testNotifRetriever, false)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 0, 1, 0, 0, 5)
			})
			It("Should report an error if not able to send service log", func() {
				conditions = getConditions(0, 1, 90, 90, 90)
				mockOCMClient.EXPECT().SendServiceLog(activeServiceLog).Return(k8serrs.NewInternalError(fmt.Errorf("a fake error")))
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(2))
				assertConditions(updatedConditions[0], 1, 1, 0, 0, 90)
				assertConditions(updatedConditions[1], 1, 0, 0, 0, 0)
			})
			It("Should report an error if not able to update NotificationStatus", func() {
				updatedConditionsError = k8serrs.NewInternalError(fmt.Errorf("a fake error"))
				conditions = getConditions(0, 1, 90, 90, 90)
				err := webhookReceiverHandler.processAlert(testAlert, testNotifRetriever, true)
				Expect(err).Should(HaveOccurred())
				Expect(len(updatedConditions)).To(Equal(1))
				assertConditions(updatedConditions[0], 1, 1, 0, 0, 90)
			})
		})
	})

	Context("Checking the response from OCM", func() {
		var testOperationId = "test"
		var testResponseBody = "{\"reason\": \"test\"}"

		It("will treat 201 as a successful response", func() {
			err := responseChecker(testOperationId, http.StatusCreated, []byte(testResponseBody))
			Expect(err).To(BeNil())
		})
		It("should return an error for non-JSON input", func() {
			testResponseBody := []byte(`This is not JSON data.`)
			err := responseChecker(testOperationId, http.StatusMovedPermanently, []byte(testResponseBody))
			Expect(err).Should(HaveOccurred())
		})
		It("will treat all other responses as failures", func() {
			var testFailedResponseCodes = []int{
				http.StatusForbidden,
				http.StatusBadRequest,
				http.StatusUnauthorized,
				http.StatusInternalServerError,
				http.StatusOK,
			}
			for _, code := range testFailedResponseCodes {
				err := responseChecker(testOperationId, code, []byte(testResponseBody))
				Expect(err).NotTo(BeNil())
			}
		})
	})
})
