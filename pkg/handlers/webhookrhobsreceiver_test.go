package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"go.uber.org/mock/gomock"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	ocmagentv1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/prometheus/alertmanager/template"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"sigs.k8s.io/controller-runtime/pkg/client"

	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	"github.com/openshift/ocm-agent/pkg/ocm"
	webhookreceivermock "github.com/openshift/ocm-agent/pkg/ocm/mocks"
	clientmocks "github.com/openshift/ocm-agent/pkg/util/test/generated/mocks/client"
)

func initItemInRecord(managedFleetNotificationRecord *ocmagentv1alpha1.ManagedFleetNotificationRecord, firingSentCount, resolvedSentCount, lastUpdateMinutesAgo int) {
	notificationRecordItem := &managedFleetNotificationRecord.Status.NotificationRecordByName[0].NotificationRecordItems[0]

	notificationRecordItem.FiringNotificationSentCount = firingSentCount
	notificationRecordItem.ResolvedNotificationSentCount = resolvedSentCount
	notificationRecordItem.LastTransitionTime = &metav1.Time{Time: time.Now().Add(time.Duration(-lastUpdateMinutesAgo) * time.Minute)}
}

func assertRecordMetadata(managedFleetNotificationRecord *ocmagentv1alpha1.ManagedFleetNotificationRecord) {
	Expect(managedFleetNotificationRecord).ToNot(BeNil())
	Expect(managedFleetNotificationRecord.ObjectMeta.Name).To(Equal(testconst.TestManagedClusterID))
	Expect(managedFleetNotificationRecord.ObjectMeta.Namespace).To(Equal(OCMAgentNamespaceName))
}

func assertRecordStatus(managedFleetNotificationRecord *ocmagentv1alpha1.ManagedFleetNotificationRecord) {
	Expect(managedFleetNotificationRecord.Status.ManagementCluster).To(Equal(testconst.TestManagedClusterID))
	Expect(len(managedFleetNotificationRecord.Status.NotificationRecordByName)).To(Equal(1))
	Expect(managedFleetNotificationRecord.Status.NotificationRecordByName[0].NotificationName).To(Equal(testconst.TestNotificationName))
	Expect(len(managedFleetNotificationRecord.Status.NotificationRecordByName[0].NotificationRecordItems)).To(Equal(1))
}

func assertRecordItem(notificationRecordItem *ocmagentv1alpha1.NotificationRecordItem, firingSentCount, resolvedSentCount, lastUpdateMinutesAgo int) {
	Expect(notificationRecordItem.HostedClusterID).To(Equal(testconst.TestHostedClusterID))
	Expect(notificationRecordItem.FiringNotificationSentCount).To(Equal(firingSentCount))
	Expect(notificationRecordItem.ResolvedNotificationSentCount).To(Equal(resolvedSentCount))

	if lastUpdateMinutesAgo < 0 {
		Expect(notificationRecordItem.LastTransitionTime).To(BeNil())
	} else {
		actualMinutesAgo := int(time.Since(notificationRecordItem.LastTransitionTime.Time).Minutes())
		Expect(actualMinutesAgo).To(Equal(lastUpdateMinutesAgo))
	}
}

var _ = Describe("Webhook Handlers", func() {

	var (
		mockCtrl             *gomock.Controller
		mockClient           *clientmocks.MockClient
		mockOCMClient        *webhookreceivermock.MockOCMClient
		testHandler          *WebhookRHOBSReceiverHandler
		server               *ghttp.Server
		testAlertFiring      template.Alert
		testAlertResolved    template.Alert
		mockStatusWriter     *clientmocks.MockStatusWriter
		serviceLog           *ocm.ServiceLog
		limitedSupportReason *cmv1.LimitedSupportReason
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		server = ghttp.NewServer()
		mockOCMClient = webhookreceivermock.NewMockOCMClient(mockCtrl)
		testHandler = &WebhookRHOBSReceiverHandler{
			c:   mockClient,
			ocm: mockOCMClient,
		}
		testAlertFiring = testconst.NewTestAlert(false, true)
		testAlertResolved = testconst.NewTestAlert(true, true)
		serviceLog = testconst.NewTestServiceLog(
			ocm.ServiceLogActivePrefix+": "+testconst.ServiceLogSummary,
			testconst.ServiceLogFleetDesc,
			testconst.TestHostedClusterID,
			testconst.TestNotification.Severity,
			"",
			testconst.TestNotification.References)
		limitedSupportManagedFleetNotification := testconst.NewManagedFleetNotification(true)
		limitedSupportReason, _ = cmv1.NewLimitedSupportReason().Summary(limitedSupportManagedFleetNotification.Spec.FleetNotification.Summary).Details(limitedSupportManagedFleetNotification.Spec.FleetNotification.NotificationMessage).DetectionType(cmv1.DetectionTypeManual).Build()
	})

	AfterEach(func() {
		server.Close()
	})

	Context("NewWebhookRHOBSReceiverHandler.processAlert", func() {
		Context("Alert is invalid", func() {
			It("Reports error if alert does not have alertname label", func() {
				delete(testAlertFiring.Labels, "alertname")
				err := testHandler.processAlert(testAlertFiring, true)
				Expect(err).Should(HaveOccurred())
			})
			It("Reports error if alert does not have managed_notification_template label", func() {
				delete(testAlertFiring.Labels, "managed_notification_template")
				err := testHandler.processAlert(testAlertFiring, true)
				Expect(err).Should(HaveOccurred())
			})
			It("Reports error if alert does not have send_managed_notification label", func() {
				delete(testAlertResolved.Labels, "send_managed_notification")
				err := testHandler.processAlert(testAlertResolved, false)
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("Alert is valid", func() {
			var managedFleetNotification *ocmagentv1alpha1.ManagedFleetNotification

			BeforeEach(func() {
				defaultManagedFleetNotification := testconst.NewManagedFleetNotification(false)
				defaultManagedFleetNotification.Spec.FleetNotification.ResendWait = 1
				managedFleetNotification = &defaultManagedFleetNotification

				mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
					Namespace: OCMAgentNamespaceName,
					Name:      managedFleetNotification.ObjectMeta.Name,
				}, gomock.Any()).DoAndReturn(
					func(ctx context.Context, key client.ObjectKey, res *ocmagentv1alpha1.ManagedFleetNotification, opts ...client.GetOption) error {
						if managedFleetNotification == nil {
							return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
						} else {
							*res = *managedFleetNotification
							return nil
						}
					}).MinTimes(1)
			})

			It("Should report an error if there is no ManagedFleetNotification", func() {
				managedFleetNotification = nil

				err := testHandler.processAlert(testAlertFiring, true)
				Expect(err).Should(HaveOccurred())
			})

			Context("There is a ManagedFleetNotification", func() {
				var managedFleetNotificationRecord *ocmagentv1alpha1.ManagedFleetNotificationRecord
				var updatedNotificationRecordItems []ocmagentv1alpha1.NotificationRecordItem
				var updateNotificationRecordError error

				BeforeEach(func() {
					defaultManagedFleetNotificationRecord := testconst.NewManagedFleetNotificationRecordWithStatus()
					managedFleetNotificationRecord = &defaultManagedFleetNotificationRecord
					updatedNotificationRecordItems = nil
					updateNotificationRecordError = nil

					// Fetch the ManagedFleetNotificationRecord
					mockClient.EXPECT().Get(gomock.Any(), client.ObjectKey{
						Namespace: OCMAgentNamespaceName,
						Name:      managedFleetNotificationRecord.ObjectMeta.Name,
					}, gomock.Any()).DoAndReturn(
						func(ctx context.Context, key client.ObjectKey, res *ocmagentv1alpha1.ManagedFleetNotificationRecord, opts ...client.GetOption) error {
							if managedFleetNotificationRecord == nil {
								return kerrors.NewNotFound(schema.GroupResource{}, key.Name)
							} else {
								*res = *managedFleetNotificationRecord
								return nil
							}
						}).AnyTimes()

					// Update status for the handled alert
					mockClient.EXPECT().Status().Return(mockStatusWriter).AnyTimes()
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
							updatedManagedFleetNotificationRecord, _ := obj.(*ocmagentv1alpha1.ManagedFleetNotificationRecord)
							assertRecordMetadata(updatedManagedFleetNotificationRecord)
							assertRecordStatus(updatedManagedFleetNotificationRecord)
							updatedNotificationRecordItems = append(updatedNotificationRecordItems, updatedManagedFleetNotificationRecord.Status.NotificationRecordByName[0].NotificationRecordItems[0])

							return updateNotificationRecordError
						},
					).AnyTimes()
				})

				Context("There is no ManagedFleetNotificationRecord", func() {
					BeforeEach(func() {
						managedFleetNotificationRecord = nil

						mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
								createdManagedFleetNotificationRecord, _ := obj.(*ocmagentv1alpha1.ManagedFleetNotificationRecord)
								assertRecordMetadata(createdManagedFleetNotificationRecord)
								Expect(len(createdManagedFleetNotificationRecord.Status.NotificationRecordByName)).To(Equal(0))

								return nil
							},
						)
					})
					Context("When notifications are of 'limited support' type", func() {
						BeforeEach(func() {
							managedFleetNotification.Spec.FleetNotification.LimitedSupport = true
						})
						It("Sends limited support when processing a firing alert", func() {
							// Send limited support
							mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).Return(nil)

							err := testHandler.processAlert(testAlertFiring, true)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(len(updatedNotificationRecordItems)).To(Equal(1))
							assertRecordItem(&updatedNotificationRecordItems[0], 1, 0, 0)
						})
						It("Does nothing when processing a resolving alert", func() {
							err := testHandler.processAlert(testAlertResolved, false)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(len(updatedNotificationRecordItems)).To(Equal(1))
							assertRecordItem(&updatedNotificationRecordItems[0], 0, 0, -1)
						})
					})
					Context("When notifications are of 'service log' type", func() {
						It("Sends service log when processing a firing alert", func() {
							// Send service log
							mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil)

							err := testHandler.processAlert(testAlertFiring, true)
							Expect(err).ShouldNot(HaveOccurred())
							Expect(len(updatedNotificationRecordItems)).To(Equal(1))
							assertRecordItem(&updatedNotificationRecordItems[0], 1, 0, 0)
						})
					})
				})

				Context("There is a ManagedFleetNotificationRecord", func() {
					Context("When notifications are of 'limited support' type", func() {
						BeforeEach(func() {
							managedFleetNotification.Spec.FleetNotification.LimitedSupport = true
						})
						Context("When processing a firing alert", func() {
							It("Sends limited support when there is a ManagedFleetNotificationRecord without status", func() {
								managedFleetNotificationRecord.Status.NotificationRecordByName = nil

								// Send limited support
								mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).Return(nil)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 1, 0, 0)
							})
							It("Sends limited support when there is an empty ManagedFleetNotificationRecord status", func() {
								// Send limited support
								mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).Return(nil)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 1, 0, 0)
							})
							It("Sends limited support when status ManagedFleetNotificationRecord counters are equal and out of the no-resend time window", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 42, 90)

								// Send limited support
								mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).Return(nil)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 42, 0)
							})
							It("Does nothing if the limited support cannot be sent", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 42, 90)

								// Send limited support
								mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).Return(errors.New("cannot be put in LS"))

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).Should(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(2))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 42, 0)
								assertRecordItem(&updatedNotificationRecordItems[1], 42, 42, 90)
							})
							It("Does nothing if updating ManagedFleetNotificationRecord status is in error", func() {
								updateNotificationRecordError = kerrors.NewInternalError(fmt.Errorf("a fake error"))

								initItemInRecord(managedFleetNotificationRecord, 42, 42, 90)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).Should(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 42, 0)
							})
							It("Does nothing if updating ManagedFleetNotificationRecord status is in conflict", func() {
								updateNotificationRecordError = kerrors.NewConflict(schema.GroupResource{}, managedFleetNotificationRecord.Name, fmt.Errorf("a fake error"))

								initItemInRecord(managedFleetNotificationRecord, 42, 42, 90)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).Should(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(5))
								for k := 0; k < 5; k++ {
									assertRecordItem(&updatedNotificationRecordItems[k], 43, 42, 0)
								}
							})
							It("Does nothing when status ManagedFleetNotificationRecord counters are equal and inside the no-resend time window", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 42, 30)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 42, 42, 30)
							})
							It("Does nothing when the status ManagedFleetNotificationRecord firing counter is already bigger than the resolved counter", func() {
								initItemInRecord(managedFleetNotificationRecord, 43, 42, 90)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 42, 90)
							})
							Context("2 alerts are received", func() {
								It("Sends limited support only once even if the time window is null", func() {
									managedFleetNotification.Spec.FleetNotification.ResendWait = 0
									initItemInRecord(managedFleetNotificationRecord, 42, 42, 90)

									// Send limited support
									mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).DoAndReturn(
										func(hcid string, lsr *cmv1.LimitedSupportReason) error {
											initItemInRecord(managedFleetNotificationRecord, 43, 42, 0)
											return nil
										},
									)

									err := testHandler.processAlert(testAlertFiring, true)
									Expect(err).ShouldNot(HaveOccurred())
									err = testHandler.processAlert(testAlertFiring, true)
									Expect(err).ShouldNot(HaveOccurred())
									Expect(len(updatedNotificationRecordItems)).To(Equal(2))
									assertRecordItem(&updatedNotificationRecordItems[0], 43, 42, 0)
									assertRecordItem(&updatedNotificationRecordItems[1], 43, 42, 0)
								})
							})
						})
						Context("When processing a resolving alert", func() {
							It("Does nothing when status ManagedFleetNotificationRecord counters are already equal", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 42, 90)

								err := testHandler.processAlert(testAlertResolved, false)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 42, 42, 90)
							})
							It("Removes limited support when the status ManagedFleetNotificationRecord firing counter is bigger than the resolved counter", func() {
								initItemInRecord(managedFleetNotificationRecord, 43, 42, 0)

								gomock.InOrder(
									// LS reasons are fetched
									mockOCMClient.EXPECT().GetLimitedSupportReasons(testconst.TestHostedClusterID).Return([]*cmv1.LimitedSupportReason{limitedSupportReason}, nil),
									// LS reason matching for the MFN is removed
									mockOCMClient.EXPECT().RemoveLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason.ID()).Return(nil),
								)

								err := testHandler.processAlert(testAlertResolved, false)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 43, 0)
							})
							It("Does nothing if the limited support cannot be removed", func() {
								initItemInRecord(managedFleetNotificationRecord, 43, 42, 90)

								gomock.InOrder(
									// LS reasons are fetched
									mockOCMClient.EXPECT().GetLimitedSupportReasons(testconst.TestHostedClusterID).Return([]*cmv1.LimitedSupportReason{limitedSupportReason}, nil),
									// LS reason matching for the MFN cannot be removed
									mockOCMClient.EXPECT().RemoveLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason.ID()).Return(errors.New("cannot be removed from LS")),
								)

								err := testHandler.processAlert(testAlertResolved, false)
								Expect(err).Should(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(2))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 43, 90)
								assertRecordItem(&updatedNotificationRecordItems[1], 43, 42, 90)
							})
							It("Removes limited support when the status ManagedFleetNotificationRecord firing counter is way bigger than the resolved counter", func() {
								initItemInRecord(managedFleetNotificationRecord, 43, 0, 10)

								gomock.InOrder(
									// LS reasons are fetched
									mockOCMClient.EXPECT().GetLimitedSupportReasons(testconst.TestHostedClusterID).Return([]*cmv1.LimitedSupportReason{limitedSupportReason}, nil),
									// LS reason matching for the MFN is removed
									mockOCMClient.EXPECT().RemoveLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason.ID()).Return(nil),
								)

								err := testHandler.processAlert(testAlertResolved, false)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 43, 10)
							})
						})
					})

					Context("When notifications are of 'service log' type", func() {
						Context("When processing a firing alert", func() {
							It("Sends service log when there is a ManagedFleetNotificationRecord without status", func() {
								managedFleetNotificationRecord.Status.NotificationRecordByName = nil

								// Send service log
								mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 1, 0, 0)
							})
							It("Sends service log when status ManagedFleetNotificationRecord counters are equal and out of the no-resend time window", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 0, 90)

								// Send service log
								mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 0, 0)
							})
							It("Does nothing if the service log cannot be sent", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 0, 90)

								// Send service log
								mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(errors.New("cannot send SL"))

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).Should(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(2))
								assertRecordItem(&updatedNotificationRecordItems[0], 43, 0, 0)
								assertRecordItem(&updatedNotificationRecordItems[1], 42, 0, 90)
							})
							It("Does nothing when inside the no-resend time window", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 0, 30)

								err := testHandler.processAlert(testAlertFiring, true)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(1))
								assertRecordItem(&updatedNotificationRecordItems[0], 42, 0, 30)
							})
							Context("2 alerts are received", func() {
								It("Sends service log once even if the time window is null", func() {
									managedFleetNotification.Spec.FleetNotification.ResendWait = 0
									initItemInRecord(managedFleetNotificationRecord, 42, 0, 90)

									// Send service log once
									mockOCMClient.EXPECT().SendServiceLog(serviceLog).DoAndReturn(
										func(sl *ocm.ServiceLog) error {
											initItemInRecord(managedFleetNotificationRecord, 43, 0, 0)
											return nil
										},
									)

									err := testHandler.processAlert(testAlertFiring, true)
									Expect(err).ShouldNot(HaveOccurred())
									err = testHandler.processAlert(testAlertFiring, true)
									Expect(err).ShouldNot(HaveOccurred())
									Expect(len(updatedNotificationRecordItems)).To(Equal(2))
									assertRecordItem(&updatedNotificationRecordItems[0], 43, 0, 0)
									assertRecordItem(&updatedNotificationRecordItems[1], 43, 0, 0)
								})
							})
						})
						Context("When processing a resolving alert", func() {
							It("Does nothing", func() {
								initItemInRecord(managedFleetNotificationRecord, 42, 0, 90)

								err := testHandler.processAlert(testAlertResolved, false)
								Expect(err).ShouldNot(HaveOccurred())
								Expect(len(updatedNotificationRecordItems)).To(Equal(0))
							})
						})
					})
				})
			})
		})
	})
})

var _ = Describe("Webhook Handlers Additional Tests", func() {
	var (
		mockCtrl         *gomock.Controller
		mockClient       *clientmocks.MockClient
		mockOCMClient    *webhookreceivermock.MockOCMClient
		testHandler      *WebhookRHOBSReceiverHandler
		mockStatusWriter *clientmocks.MockStatusWriter
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		mockStatusWriter = clientmocks.NewMockStatusWriter(mockCtrl)
		mockOCMClient = webhookreceivermock.NewMockOCMClient(mockCtrl)
		testHandler = NewWebhookRHOBSReceiverHandler(mockClient, mockOCMClient)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("HTTP Handler Tests", func() {
		var (
			responseRecorder *httptest.ResponseRecorder
			validAlert       template.Alert
			validMFN         oav1alpha1.ManagedFleetNotification
		)

		BeforeEach(func() {
			responseRecorder = httptest.NewRecorder()
			validAlert = testconst.NewTestAlert(false, true)
			validMFN = testconst.NewManagedFleetNotification(false)
		})

		It("should return 405 for GET method", func() {
			req := httptest.NewRequest("GET", "/webhook", nil)
			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusMethodNotAllowed))
			Expect(responseRecorder.Body.String()).To(Equal("Method Not Allowed\n"))
		})

		It("should return 405 for PUT method", func() {
			req := httptest.NewRequest("PUT", "/webhook", nil)
			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusMethodNotAllowed))
			Expect(responseRecorder.Body.String()).To(Equal("Method Not Allowed\n"))
		})

		It("should return 405 for DELETE method", func() {
			req := httptest.NewRequest("DELETE", "/webhook", nil)
			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusMethodNotAllowed))
			Expect(responseRecorder.Body.String()).To(Equal("Method Not Allowed\n"))
		})

		It("should return 400 for invalid JSON in request body", func() {
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader("invalid json"))
			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			Expect(responseRecorder.Body.String()).To(Equal("Bad request body\n"))
		})

		It("should return 400 for empty request body", func() {
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(""))
			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusBadRequest))
			Expect(responseRecorder.Body.String()).To(Equal("Bad request body\n"))
		})

		It("should handle nil request", func() {
			// Nil request will cause a panic when accessing r.Method, which is expected behavior
			defer func() {
				r := recover()
				Expect(r).ToNot(BeNil())
				Expect(fmt.Sprintf("%v", r)).To(ContainSubstring("runtime error: invalid memory address or nil pointer dereference"))
			}()

			testHandler.ServeHTTP(responseRecorder, nil)

			// This line should not be reached due to panic
			Fail("Expected panic for nil request")
		})

		It("should process valid POST request successfully", func() {
			alertData := AMReceiverData{
				Alerts: []template.Alert{validAlert},
			}
			jsonData, _ := json.Marshal(alertData)
			req := httptest.NewRequest("POST", "/webhook", bytes.NewBuffer(jsonData))

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, validMFN)
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.NewManagedFleetNotificationRecordWithStatus())
			mockOCMClient.EXPECT().SendServiceLog(gomock.Any()).Return(nil)
			//mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.NewManagedFleetNotificationRecordWithStatus())
			mockClient.EXPECT().Status().Return(mockStatusWriter)
			mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			Expect(responseRecorder.Header().Get("Content-Type")).To(Equal("application/json"))
		})
	})

	Context("JSON Error Handling Tests", func() {
		It("should handle JSON encoding errors in response", func() {
			// This test simulates a scenario where JSON encoding fails
			// by using a ResponseRecorder that will fail on write
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(`{"alerts":[]}`))

			failingWriter := &FailingResponseWriter{
				header:     make(http.Header),
				shouldFail: true,
			}

			testHandler.ServeHTTP(failingWriter, req)

			Expect(failingWriter.statusCode).To(Equal(http.StatusInternalServerError))
		})

		It("should handle malformed JSON with special characters", func() {
			malformedJSON := `{"alerts":[{"labels":{"key":"value with\nnewline\tand\ttab"}}]}`
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(malformedJSON))
			responseRecorder := httptest.NewRecorder()

			testHandler.ServeHTTP(responseRecorder, req)

			// Error code 200 is returned as errors returned by NewWebhookRHOBSReceiverHandler.processAlert are not propagated to HTTP response
			// (same behavior than the WebhookReceiverHandler handler)
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})

		It("should handle JSON with unexpected structure", func() {
			unexpectedJSON := `{"unexpected_field": "value"}`
			req := httptest.NewRequest("POST", "/webhook", strings.NewReader(unexpectedJSON))
			responseRecorder := httptest.NewRecorder()

			testHandler.ServeHTTP(responseRecorder, req)

			// Should process the empty alerts array
			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
		})
	})
})

// Mock Response Writer for testing JSON encoding errors
type FailingResponseWriter struct {
	header     http.Header
	statusCode int
	shouldFail bool
}

func (f *FailingResponseWriter) Header() http.Header {
	return f.header
}

func (f *FailingResponseWriter) Write([]byte) (int, error) {
	if f.shouldFail {
		return 0, errors.New("simulated write failure")
	}
	return 0, nil
}

func (f *FailingResponseWriter) WriteHeader(statusCode int) {
	f.statusCode = statusCode
}
