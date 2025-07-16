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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/prometheus/alertmanager/template"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

var _ = Describe("WebhookRHOBSReceiverHandler HTTP Tests", func() {

	var (
		mockCtrl              *gomock.Controller
		mockClient            *clientmocks.MockClient
		mockOCMClient         *webhookreceivermock.MockOCMClient
		testHandler           *WebhookRHOBSReceiverHandler
		server                *ghttp.Server
		testAlertFiring       template.Alert
		testAlertResolved     template.Alert
		testMFN               oav1alpha1.ManagedFleetNotification
		testLimitedSupportMFN oav1alpha1.ManagedFleetNotification
		testMFNR              oav1alpha1.ManagedFleetNotificationRecord
		testMFNRWithStatus    oav1alpha1.ManagedFleetNotificationRecord
		mockStatusWriter      *clientmocks.MockStatusWriter
		serviceLog            *ocm.ServiceLog
		limitedSupportReason  *cmv1.LimitedSupportReason
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
		testMFN = testconst.NewManagedFleetNotification(false)
		testLimitedSupportMFN = testconst.NewManagedFleetNotification(true)
		testMFNR = testconst.NewManagedFleetNotificationRecord()
		testMFNRWithStatus = testconst.NewManagedFleetNotificationRecordWithStatus()
		serviceLog = testconst.NewTestServiceLog(
			ocm.ServiceLogActivePrefix+": "+testconst.ServiceLogSummary,
			testconst.ServiceLogFleetDesc,
			testconst.TestHostedClusterID,
			testconst.TestNotification.Severity,
			"",
			testconst.TestNotification.References)
		limitedSupportReason, _ = cmv1.NewLimitedSupportReason().Summary(testLimitedSupportMFN.Spec.FleetNotification.Summary).Details(testLimitedSupportMFN.Spec.FleetNotification.NotificationMessage).DetectionType(cmv1.DetectionTypeManual).Build()
	})

	AfterEach(func() {
		server.Close()
	})

	Context("When calling updateManagedFleetNotificationRecord", func() {
		Context("When the fleet record and record and status don't exist yet", func() {
			It("Creates it and sets the status", func() {
				gomock.InOrder(
					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{
						Group: oav1alpha1.GroupVersion.Group, Resource: "ManagedFleetNotificationRecord"}, testconst.TestManagedClusterID),
					),
					// Create the MFNR
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.CreateOption) error {
							Expect(mfnr.Name).To(Equal(testconst.TestManagedClusterID))
							return nil
						}),
					// Update status for the handled alert
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.updateManagedFleetNotificationRecord(testAlertFiring, &testMFN)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
		Context("When the record and status already exists", func() {
			It("Updates the existing record and updates the status for the notification type", func() {
				gomock.InOrder(
					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus),

					// Update status for the handled alert
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.updateManagedFleetNotificationRecord(testAlertFiring, &testMFN)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Context("When processing a firing alert", func() {
		Context("When the MFN of type limited support for a firing alert", func() {
			It("Sends limited support", func() {
				gomock.InOrder(
					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus),

					// Send limited support
					mockOCMClient.EXPECT().SendLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason).Return(nil),

					// Fetch the MFNR and update it's status
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus),

					// Update status for the handled alert
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.processFiringAlert(testAlertFiring, &testLimitedSupportMFN)
				Expect(err).ShouldNot(HaveOccurred())
			})
			Context("When the MFN of type limited support for a firing alert and a previous firing notification hasn't resolved yet", func() {
				It("Doesn't re-send limited support", func() {
					// Increment firing status (firing = 1 and resolved = 0)
					_, err := testMFNRWithStatus.UpdateNotificationRecordItem(testconst.TestNotificationName, testconst.TestHostedClusterID, true)
					Expect(err).ShouldNot(HaveOccurred())

					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus)
					// Return right after as there was already a LS sent that didn't resolve yet

					err = testHandler.processFiringAlert(testAlertFiring, &testLimitedSupportMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
		})

		Context("When the MFN of type limited support for a firing alert", func() {
			It("Removes no limited support if none exist", func() {
				gomock.InOrder(
					// Get limited support reasons, returns empty so no limited supports will be removed
					mockOCMClient.EXPECT().GetLimitedSupportReasons(testconst.TestHostedClusterID).Return([]*cmv1.LimitedSupportReason{}, nil),

					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus),

					// Update status for the handled alert
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.processAlert(testAlertResolved, &testLimitedSupportMFN)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("Removes limited support if it was previously set", func() {
				// This reason has an ID which is used to test deleting it
				limitedSupportReason, _ = cmv1.NewLimitedSupportReason().Summary(testMFN.Spec.FleetNotification.Summary).Details(testMFN.Spec.FleetNotification.NotificationMessage).ID("1234").DetectionType(cmv1.DetectionTypeManual).Build()

				gomock.InOrder(
					// LS reasons are fetched
					mockOCMClient.EXPECT().GetLimitedSupportReasons(testconst.TestHostedClusterID).Return([]*cmv1.LimitedSupportReason{limitedSupportReason}, nil),
					// LS reason matching for the MFN is removed
					mockOCMClient.EXPECT().RemoveLimitedSupport(testconst.TestHostedClusterID, limitedSupportReason.ID()).Return(nil),

					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus),

					// Update status for the handled alert
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.processAlert(testAlertResolved, &testLimitedSupportMFN)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("When a managed fleet notification record does exist", func() {
			Context("And doesn't have a Management Cluster in the status", func() {
				BeforeEach(func() {
					testMFNR.Status.ManagementCluster = ""
					testMFNR.Status.NotificationRecordByName = []oav1alpha1.NotificationRecordByName{}
				})
				It("Updates the status", func() {
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil),

						// Update SL sent status
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNRWithStatus),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					)

					err := testHandler.processAlert(testAlertFiring, &testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
			Context("When a notification record doesn't exist", func() {
				It("Creates one", func() {
					// Let's add a notification record, but named differently to the one we want,
					// so we can verify the handler doesn't try and use it
					testMFNR.Status.NotificationRecordByName = []oav1alpha1.NotificationRecordByName{
						{
							NotificationName:        "dummy-name",
							ResendWait:              24,
							NotificationRecordItems: []oav1alpha1.NotificationRecordItem{},
						},
					}
					testMFNR.Status.ManagementCluster = testconst.TestManagedClusterID

					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),

						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil),

						// Update status (create the record item)
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),

						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.UpdateOptions) error {
								Expect(len(mfnr.Status.NotificationRecordByName)).To(Equal(2))
								Expect(mfnr.Status.NotificationRecordByName[1].NotificationRecordItems[0].FiringNotificationSentCount).To(Equal(1))
								Expect(mfnr.Status.NotificationRecordByName[1].NotificationRecordItems[0].HostedClusterID).To(Equal(testconst.TestHostedClusterID))
								return nil
							}),
					)
					err := testHandler.processAlert(testAlertFiring, &testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
			Context("When a notification record item for the hosted cluster already exists", func() {
				var testTime = &metav1.Time{Time: time.Now().Add(time.Duration(-48) * time.Hour)}
				It("Updates the existing one", func() {
					testMFNR.Status.NotificationRecordByName = []oav1alpha1.NotificationRecordByName{
						{
							NotificationName: testconst.TestNotificationName,
							ResendWait:       24,
							NotificationRecordItems: []oav1alpha1.NotificationRecordItem{
								{
									HostedClusterID:             testconst.TestHostedClusterID,
									FiringNotificationSentCount: 1,
									LastTransitionTime:          testTime,
								},
							},
						},
					}
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil),
						// Update existing MFNR item
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.UpdateOptions) error {
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].FiringNotificationSentCount).To(Equal(2))
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].HostedClusterID).To(Equal(testconst.TestHostedClusterID))
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].LastTransitionTime.After(testTime.Time)).To(BeTrue())
								return nil
							}),
					)
					err := testHandler.processAlert(testAlertFiring, &testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
			Context("When a notification record item for a hosted cluster does not exist", func() {
				It("Creates one", func() {
					testMFNR.Status.NotificationRecordByName = []oav1alpha1.NotificationRecordByName{
						{
							NotificationName:        testconst.TestNotificationName,
							ResendWait:              24,
							NotificationRecordItems: []oav1alpha1.NotificationRecordItem{},
						},
					}
					testMFNR.Status.ManagementCluster = testconst.TestManagedClusterID

					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(serviceLog).Return(nil),

						// Re-fetch the MFNR for the status update
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.UpdateOptions) error {
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].FiringNotificationSentCount).To(Equal(1))
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].HostedClusterID).To(Equal(testconst.TestHostedClusterID))
								return nil
							}),
					)
					err := testHandler.processAlert(testAlertFiring, &testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
			Context("When a SL has been sent for the alert recently", func() {
				It("Does not re-send", func() {
					// Set a notification record item with a last sent time within the 24 hour window of the notification record
					testMFNR.Status.NotificationRecordByName = []oav1alpha1.NotificationRecordByName{
						{
							NotificationName: testconst.TestNotificationName,
							ResendWait:       24,
							NotificationRecordItems: []oav1alpha1.NotificationRecordItem{
								{
									HostedClusterID:             testconst.TestHostedClusterID,
									FiringNotificationSentCount: 1,
									LastTransitionTime:          &metav1.Time{Time: time.Now().Add(time.Duration(-1) * time.Hour)},
								},
							},
						},
					}
					testMFN.Spec.FleetNotification.ResendWait = 24
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
					)

					err := testHandler.processAlert(testAlertFiring, &testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
		})
	})
})

var _ = Describe("WebhookRHOBSReceiverHandler Additional Tests", func() {
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

	Context("Constructor Tests", func() {
		It("should create a new WebhookRHOBSReceiverHandler with valid parameters", func() {
			handler := NewWebhookRHOBSReceiverHandler(mockClient, mockOCMClient)
			Expect(handler).ToNot(BeNil())
			Expect(handler.c).To(Equal(mockClient))
			Expect(handler.ocm).To(Equal(mockOCMClient))
		})

		It("should create a new WebhookRHOBSReceiverHandler with nil client", func() {
			handler := NewWebhookRHOBSReceiverHandler(nil, mockOCMClient)
			Expect(handler).ToNot(BeNil())
			Expect(handler.c).To(BeNil())
			Expect(handler.ocm).To(Equal(mockOCMClient))
		})

		It("should create a new WebhookRHOBSReceiverHandler with nil OCM client", func() {
			handler := NewWebhookRHOBSReceiverHandler(mockClient, nil)
			Expect(handler).ToNot(BeNil())
			Expect(handler.c).To(Equal(mockClient))
			Expect(handler.ocm).To(BeNil())
		})
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
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.NewManagedFleetNotificationRecordWithStatus())
			mockClient.EXPECT().Status().Return(mockStatusWriter)
			mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			testHandler.ServeHTTP(responseRecorder, req)

			Expect(responseRecorder.Code).To(Equal(http.StatusOK))
			Expect(responseRecorder.Header().Get("Content-Type")).To(Equal("application/json"))
		})
	})

	Context("processAMReceiver Tests", func() {
		It("should handle empty alerts slice", func() {
			alertData := AMReceiverData{Alerts: []template.Alert{}}
			response := testHandler.processAMReceiver(alertData, context.Background())

			Expect(response.Status).To(Equal("ok"))
			Expect(response.Code).To(Equal(http.StatusOK))
			Expect(response.Error).To(BeNil())
		})

		It("should return error when ManagedFleetNotification is not found", func() {
			alert := testconst.NewTestAlert(false, true)
			alertData := AMReceiverData{Alerts: []template.Alert{alert}}

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{}, "not-found"))

			response := testHandler.processAMReceiver(alertData, context.Background())

			Expect(response.Status).To(ContainSubstring("unable to find ManagedFleetNotification"))
			Expect(response.Code).To(Equal(http.StatusInternalServerError))
			Expect(response.Error).ToNot(BeNil())
		})

		It("should skip invalid alerts", func() {
			invalidAlert := template.Alert{
				Labels: map[string]string{
					"alertname": "TestAlert",
					// Missing send_managed_notification and managed_notification_template labels
					// which are required for valid alerts
				},
				Status: "firing",
			}
			alertData := AMReceiverData{Alerts: []template.Alert{invalidAlert}}

			// Even invalid alerts trigger client.Get() with empty templateName before validation
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{}, "not-found"))

			response := testHandler.processAMReceiver(alertData, context.Background())

			Expect(response.Status).To(ContainSubstring("unable to find ManagedFleetNotification"))
			Expect(response.Code).To(Equal(http.StatusInternalServerError))
			Expect(response.Error).ToNot(BeNil())
		})
	})

	Context("processAlert Tests", func() {
		var (
			validMFN    oav1alpha1.ManagedFleetNotification
			firingAlert template.Alert
		)

		BeforeEach(func() {
			validMFN = testconst.NewManagedFleetNotification(false)
			firingAlert = testconst.NewTestAlert(false, true)
		})

		It("should return error for alert with unknown status", func() {
			unknownAlert := firingAlert
			unknownAlert.Status = "unknown"

			err := testHandler.processAlert(unknownAlert, &validMFN)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected status unknown"))
		})

		It("should return error for alert with empty status", func() {
			emptyAlert := firingAlert
			emptyAlert.Status = ""

			err := testHandler.processAlert(emptyAlert, &validMFN)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected status"))
		})
	})

	Context("Error Handling Tests", func() {
		It("should handle OCM client errors in processFiringAlert", func() {
			alert := testconst.NewTestAlert(false, true)
			limitedSupportMFN := testconst.NewManagedFleetNotification(true)

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.NewManagedFleetNotificationRecordWithStatus())
			mockOCMClient.EXPECT().SendLimitedSupport(gomock.Any(), gomock.Any()).Return(errors.New("OCM API error"))

			err := testHandler.processFiringAlert(alert, &limitedSupportMFN)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("OCM API error"))
		})

		It("should handle Kubernetes client errors in updateManagedFleetNotificationRecord", func() {
			alert := testconst.NewTestAlert(false, true)
			mfn := testconst.NewManagedFleetNotification(false)

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("k8s client error"))

			err := testHandler.updateManagedFleetNotificationRecord(alert, &mfn)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("k8s client error"))
		})

		It("should handle status update errors", func() {
			alert := testconst.NewTestAlert(false, true)
			mfn := testconst.NewManagedFleetNotification(false)
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr)
			mockClient.EXPECT().Status().Return(mockStatusWriter)
			mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("status update error"))

			err := testHandler.updateManagedFleetNotificationRecord(alert, &mfn)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("status update error"))
		})
	})

	Context("Edge Cases Tests", func() {
		It("should handle alerts with missing required labels", func() {
			alert := template.Alert{
				Labels: map[string]string{
					// Missing alertname, managed_notification_template, send_managed_notification
					"some_other_label": "value",
				},
				Status: "firing",
			}

			// Even invalid alerts trigger client.Get() with empty templateName before validation
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{}, "not-found"))

			response := testHandler.processAMReceiver(AMReceiverData{Alerts: []template.Alert{alert}}, context.Background())

			Expect(response.Status).To(ContainSubstring("unable to find ManagedFleetNotification"))
			Expect(response.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should handle nil ManagedFleetNotification", func() {
			alert := testconst.NewTestAlert(false, true)

			// Nil ManagedFleetNotification will cause a panic when accessing its fields, which is expected
			defer func() {
				r := recover()
				Expect(r).ToNot(BeNil())
				Expect(fmt.Sprintf("%v", r)).To(ContainSubstring("runtime error: invalid memory address or nil pointer dereference"))
			}()

			_ = testHandler.processAlert(alert, nil)

			// This line should not be reached due to panic
			Fail("Expected panic for nil ManagedFleetNotification")
		})

		It("should handle context cancellation", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			alert := testconst.NewTestAlert(false, true)
			alertData := AMReceiverData{Alerts: []template.Alert{alert}}

			// Even with cancelled context, the alert is valid so it will try to process it
			validMFN := testconst.NewManagedFleetNotification(false)
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()

			gomock.InOrder(
				// First client call to get ManagedFleetNotification
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, validMFN),
				// Then check if firing can be sent
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr),
				// Send service log
				mockOCMClient.EXPECT().SendServiceLog(gomock.Any()).Return(nil),
				// Update status
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
			)

			response := testHandler.processAMReceiver(alertData, ctx)

			// Should still process but with cancelled context
			Expect(response).ToNot(BeNil())
			Expect(response.Status).To(Equal("ok"))
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

			// The JSON parses successfully but creates an invalid alert (missing required labels)
			// Even invalid alerts trigger client.Get() with empty templateName before validation
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{}, "not-found"))

			testHandler.ServeHTTP(responseRecorder, req)

			// The alert fails to find ManagedFleetNotification due to missing template name
			Expect(responseRecorder.Code).To(Equal(http.StatusInternalServerError))
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

	Context("Retry Logic Tests", func() {
		It("should retry on conflict errors", func() {
			alert := testconst.NewTestAlert(false, true)
			mfn := testconst.NewManagedFleetNotification(false)
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()

			gomock.InOrder(
				// First attempt fails with conflict
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewConflict(schema.GroupResource{}, "conflict", nil)),

				// Second attempt succeeds
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
			)

			err := testHandler.updateManagedFleetNotificationRecord(alert, &mfn)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should retry on AlreadyExists errors", func() {
			alert := testconst.NewTestAlert(false, true)
			mfn := testconst.NewManagedFleetNotification(false)

			gomock.InOrder(
				// First attempt fails with AlreadyExists
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{}, "not-found")),
				mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).Return(kerrors.NewAlreadyExists(schema.GroupResource{}, "already-exists")),

				// Second attempt succeeds
				mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testconst.NewManagedFleetNotificationRecordWithStatus()),
				mockClient.EXPECT().Status().Return(mockStatusWriter),
				mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
			)

			err := testHandler.updateManagedFleetNotificationRecord(alert, &mfn)

			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("firingCanBeSent Tests", func() {
		var (
			alert template.Alert
			mfn   oav1alpha1.ManagedFleetNotification
		)

		BeforeEach(func() {
			alert = testconst.NewTestAlert(false, true)
			mfn = testconst.NewManagedFleetNotification(false)
		})

		It("should return true when no ManagedFleetNotificationRecord exists", func() {
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(kerrors.NewNotFound(schema.GroupResource{}, "not-found"))

			result := testHandler.firingCanBeSent(alert, &mfn)

			Expect(result).To(BeTrue())
		})

		It("should return true when no NotificationRecordItem exists", func() {
			mfnr := testconst.NewManagedFleetNotificationRecord()
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr)

			result := testHandler.firingCanBeSent(alert, &mfn)

			Expect(result).To(BeTrue())
		})

		It("should return true when LastTransitionTime is nil", func() {
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()
			mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].LastTransitionTime = nil
			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr)

			result := testHandler.firingCanBeSent(alert, &mfn)

			Expect(result).To(BeTrue())
		})

		It("should return false for limited support when previous firing didn't resolve", func() {
			limitedSupportMFN := testconst.NewManagedFleetNotification(true)
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()
			mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].FiringNotificationSentCount = 2
			mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].ResolvedNotificationSentCount = 1
			mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].LastTransitionTime = &metav1.Time{Time: time.Now()}

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr)

			result := testHandler.firingCanBeSent(alert, &limitedSupportMFN)

			Expect(result).To(BeFalse())
		})

		It("should return false when within resend wait interval", func() {
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()
			mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].LastTransitionTime = &metav1.Time{Time: time.Now()}
			mfnr.Status.NotificationRecordByName[0].ResendWait = 24 // 24 hours

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr)

			result := testHandler.firingCanBeSent(alert, &mfn)

			Expect(result).To(BeFalse())
		})

		It("should return true when past resend wait interval", func() {
			mfnr := testconst.NewManagedFleetNotificationRecordWithStatus()
			mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].LastTransitionTime = &metav1.Time{Time: time.Now().Add(-25 * time.Hour)}
			mfnr.Status.NotificationRecordByName[0].ResendWait = 24 // 24 hours

			mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, mfnr)

			result := testHandler.firingCanBeSent(alert, &mfn)

			Expect(result).To(BeTrue())
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
