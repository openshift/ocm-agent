package handlers

import (
	"bytes"
	"context"
	"encoding/json"

	"fmt"
	"io"
	"net/http"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/mock/gomock"
	"github.com/prometheus/alertmanager/template"

	corev1 "k8s.io/api/core/v1"
	k8serrs "k8s.io/apimachinery/pkg/api/errors"

	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/pkg/apis/ocmagent/v1alpha1"
	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	webhookreceivermock "github.com/openshift/ocm-agent/pkg/handlers/mocks"
	clientmocks "github.com/openshift/ocm-agent/pkg/util/test/generated/mocks/client"
)

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
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
		// TestNotificationRecord *ocmagentv1alpha1.NotificationRecord
		// TestNotificationConditions *ocmagentv1alpha1.NotificationCondition
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		server = ghttp.NewServer()
		mockOCMClient = webhookreceivermock.NewMockOCMClient(mockCtrl)
		webhookReceiverHandler = &WebhookReceiverHandler{
			c:   mockClient,
			ocm: mockOCMClient,
		}
		testAlert = testconst.TestAlert

	})
	AfterEach(func() {
		server.Close()
	})
	Context("AMReceiver processAMReceiver", func() {
		var r http.Request
		BeforeEach(func() {
			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
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
			mockClient.EXPECT().List(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
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

	Context("When checking if an alert is valid", func() {
		It("should indicate a valid alert is valid", func() {
			r := isValidAlert(testconst.TestAlert)
			Expect(r).To(BeTrue())
		})
		It("should invalidate an alert with no name", func() {
			delete(testAlert.Labels, AMLabelAlertName)
			r := isValidAlert(testconst.TestAlert)
			Expect(r).To(BeFalse())
		})
		It("should invalidate an alert with no send_managed_notification label", func() {
			delete(testAlert.Labels, "send_managed_notification")
			r := isValidAlert(testconst.TestAlert)
			Expect(r).To(BeFalse())
		})
		It("should invalidate an alert with no managed_notification_template label", func() {
			delete(testAlert.Labels, "managed_notification_template")
			r := isValidAlert(testconst.TestAlert)
			Expect(r).To(BeFalse())
		})
	})

	Context("When looking for a matching notification for an alert", func() {
		It("will return one if one exists", func() {
			n, mn, err := getNotification(testconst.TestNotificationName, testconst.TestManagedNotificationList)
			Expect(err).To(BeNil())
			Expect(reflect.DeepEqual(*n, testconst.TestNotification)).To(BeTrue())
			Expect(reflect.DeepEqual(*mn, testconst.TestManagedNotification)).To(BeTrue())
		})
		It("will return nil if one does not exist", func() {
			_, _, err := getNotification("dummy-nonexistent-test", testconst.TestManagedNotificationList)
			Expect(err).ToNot(BeNil())
		})
	})

	// Context("When processing an alert", func() {
	// 	When("when checking if an alert is valid or not", func() {
	// 		BeforeEach(func() {
	// 			delete(testconst.TestAlert.Labels, "alertname")
	// 		})
	// 		It("would throw error if alert does not have alertname label", func() {
	// 			err := webhookReceiverHandler.processAlert(testconst.TestAlert, testconst.TestManagedNotificationList, true)
	// 			Expect(err).Should(HaveOccurred())
	// 		})
	// 	})
	// 	Context("when checking if an alert can be mapped to existing notification template definition or not", func() {
	// 		When("when the managed_notification_template label does not exist", func() {
	// 			BeforeEach(func() {
	// 				delete(testconst.TestAlert.Labels, "managed_notification_template")
	// 			})
	// 			It("should fail with an error", func() {
	// 				err := webhookReceiverHandler.processAlert(testconst.TestAlert, testconst.TestManagedNotificationList, true)
	// 				Expect(err).Should(HaveOccurred())
	// 			})
	// 		})
	// 		When("when managednotificationlist is nil", func() {
	// 			BeforeEach(func() {
	// 				testconst.TestManagedNotificationList = &ocmagentv1alpha1.ManagedNotificationList{}
	// 			})
	// 			It("should fail with an error", func() {
	// 				err := webhookReceiverHandler.processAlert(testconst.TestAlert, testconst.TestManagedNotificationList, true)
	// 				Expect(err).Should(HaveOccurred())
	// 			})
	// 		})
	// 	})
	// 	When("when checking if the notification can be sent or not", func() {
	// 		BeforeEach(func() {
	// 			testconst.TestNotification = ocmagentv1alpha1.Notification{}
	// 		})
	// 		It("should throw error if notification is nil", func() {
	// 			err := webhookReceiverHandler.processAlert(testconst.TestAlert, testconst.TestManagedNotificationList, true)
	// 			Expect(err).Should(HaveOccurred())
	// 		})
	// 	})

	// It("will check if the service log has been sent without errors or not", func() {
	// 	gomock.InOrder(
	// 		mockOCMClient.EXPECT().SendServiceLog(testconst.TestManagedNotification, gomock.Any()).Return(nil),
	// 	)
	// 	err := webhookReceiverHandler.processAlert(testAlert, testconst.TestManagedNotificationList)
	// 	Expect(err).ToNot(BeNil())
	// })

	// })

	Context("When updating Notification status", func() {
		It("Report error if not able to get ManagedNotification", func() {
			fakeError := k8serrs.NewInternalError(fmt.Errorf("a fake error"))
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(fakeError),
			)
			err := webhookReceiverHandler.updateNotificationStatus(&testconst.TestNotification, &testconst.TestManagedNotification, true)
			Expect(err).ShouldNot(BeNil())
		})
		When("Getting NotificationRecord for which status does not exist", func() {
			It("should create status if NotificationRecord not found", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.TestManagedNotificationWithoutStatus),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, mn *ocmagentv1alpha1.ManagedNotification, client ...client.UpdateOptions) error {
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionAlertFiring).Status).To(Equal(corev1.ConditionTrue))
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionAlertResolved).Status).To(Equal(corev1.ConditionFalse))
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionServiceLogSent).Status).To(Equal(corev1.ConditionTrue))
							return nil
						}),
				)
				err := webhookReceiverHandler.updateNotificationStatus(&ocmagentv1alpha1.Notification{Name: "randomnotification"}, &testconst.TestManagedNotificationWithoutStatus, true)
				Expect(err).Should(BeNil())
				Expect(&testconst.TestManagedNotificationWithoutStatus).ToNot(BeNil())
			})
		})
		When("Getting NotificationRecord for which status already exists", func() {
			It("should send service log again after resend window passed when alert is firing", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.TestManagedNotification),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, mn *ocmagentv1alpha1.ManagedNotification, client ...client.UpdateOptions) error {
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionAlertFiring).Status).To(Equal(corev1.ConditionTrue))
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionAlertResolved).Status).To(Equal(corev1.ConditionFalse))
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionServiceLogSent).Status).To(Equal(corev1.ConditionTrue))
							return nil
						}),
				)
				err := webhookReceiverHandler.updateNotificationStatus(&testconst.TestNotification, &testconst.TestManagedNotification, true)
				Expect(err).Should(BeNil())
			})
			It("should send service log for alert resolved when no longer firing", func() {
				gomock.InOrder(
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.TestManagedNotification),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, mn *ocmagentv1alpha1.ManagedNotification, client ...client.UpdateOptions) error {
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionAlertFiring).Status).To(Equal(corev1.ConditionFalse))
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionAlertResolved).Status).To(Equal(corev1.ConditionTrue))
							Expect(mn.Status.NotificationRecords[0].Conditions.GetCondition(ocmagentv1alpha1.ConditionServiceLogSent).Status).To(Equal(corev1.ConditionTrue))
							return nil
						}),
				)
				err := webhookReceiverHandler.updateNotificationStatus(&testconst.TestNotification, &testconst.TestManagedNotification, false)
				Expect(err).Should(BeNil())
			})
		})
		It("Update ManagedNotificationStatus without any error", func() {
			gomock.InOrder(
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.TestManagedNotification),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
			)
			err := webhookReceiverHandler.updateNotificationStatus(&testconst.TestNotification, &testconst.TestManagedNotification, true)
			Expect(err).Should(BeNil())
		})
	})
})
