package handlers

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/mock/gomock"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/api/v1alpha1"
	"github.com/prometheus/alertmanager/template"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"sigs.k8s.io/controller-runtime/pkg/client"

	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	webhookreceivermock "github.com/openshift/ocm-agent/pkg/handlers/mocks"
	clientmocks "github.com/openshift/ocm-agent/pkg/util/test/generated/mocks/client"
)

var _ = Describe("RHOBS Webhook Handlers", func() {

	var (
		mockCtrl         *gomock.Controller
		mockClient       *clientmocks.MockClient
		mockOCMClient    *webhookreceivermock.MockOCMClient
		testHandler      *WebhookRHOBSReceiverHandler
		server           *ghttp.Server
		testAlert        template.Alert
		testFN           oav1alpha1.FleetNotification
		testMFN          oav1alpha1.ManagedFleetNotification
		testMFNR         oav1alpha1.ManagedFleetNotificationRecord
		mockStatusWriter *clientmocks.MockStatusWriter
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
		testAlert = testconst.NewTestAlert(false, true)
		testFN = testconst.NewFleetNotification()
		testMFN = testconst.NewManagedFleetNotification()
		testMFNR = testconst.NewManagedFleetNotificationRecord()
	})
	AfterEach(func() {
		server.Close()
	})

	Context("When processing an alert", func() {
		Context("When a fleet notification record doesn't exist", func() {
			It("Creates one", func() {
				gomock.InOrder(
					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.NewNotFound(schema.GroupResource{
						Group: oav1alpha1.GroupVersion.Group, Resource: "ManagedFleetNotificationRecord"}, testconst.TestManagedClusterID),
					),
					// Create the MFNR
					mockClient.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
						func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.CreateOption) error {
							Expect(mfnr.Name).To(Equal(testconst.TestManagedClusterID))
							return nil
						}),
					// Send the SL
					mockOCMClient.EXPECT().SendServiceLog(testFN.Summary, testFN.NotificationMessage, "", testconst.TestHostedClusterID, gomock.Any(), true),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.processAlert(testAlert, testMFN)
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
						// Update the status
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(testFN.Summary, testFN.NotificationMessage, "", testconst.TestHostedClusterID, gomock.Any(), true),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
					)

					err := testHandler.processAlert(testAlert, testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
			It("Uses the existing one", func() {
				gomock.InOrder(
					// Fetch the MFNR
					mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
					// Send the SL
					mockOCMClient.EXPECT().SendServiceLog(testFN.Summary, testFN.NotificationMessage, "", testconst.TestHostedClusterID, gomock.Any(), true),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.processAlert(testAlert, testMFN)
				Expect(err).ShouldNot(HaveOccurred())
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
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(testFN.Summary, testFN.NotificationMessage, "", testconst.TestHostedClusterID, gomock.Any(), true),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.UpdateOptions) error {
								Expect(len(mfnr.Status.NotificationRecordByName)).To(Equal(2))
								Expect(mfnr.Status.NotificationRecordByName[1].NotificationRecordItems[0].ServiceLogSentCount).To(Equal(1))
								Expect(mfnr.Status.NotificationRecordByName[1].NotificationRecordItems[0].HostedClusterID).To(Equal(testconst.TestHostedClusterID))
								return nil
							}),
					)
					err := testHandler.processAlert(testAlert, testMFN)
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
									HostedClusterID:     testconst.TestHostedClusterID,
									ServiceLogSentCount: 1,
									LastTransitionTime:  testTime,
								},
							},
						},
					}
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(testFN.Summary, testFN.NotificationMessage, "", testconst.TestHostedClusterID, gomock.Any(), true),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.UpdateOptions) error {
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].ServiceLogSentCount).To(Equal(2))
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].HostedClusterID).To(Equal(testconst.TestHostedClusterID))
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].LastTransitionTime.After(testTime.Time)).To(BeTrue())
								return nil
							}),
					)
					err := testHandler.processAlert(testAlert, testMFN)
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
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
						// Send the SL
						mockOCMClient.EXPECT().SendServiceLog(testFN.Summary, testFN.NotificationMessage, "", testconst.TestHostedClusterID, gomock.Any(), true),
						mockClient.EXPECT().Status().Return(mockStatusWriter),
						mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
							func(ctx context.Context, mfnr *oav1alpha1.ManagedFleetNotificationRecord, co ...client.UpdateOptions) error {
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].ServiceLogSentCount).To(Equal(1))
								Expect(mfnr.Status.NotificationRecordByName[0].NotificationRecordItems[0].HostedClusterID).To(Equal(testconst.TestHostedClusterID))
								return nil
							}),
					)
					err := testHandler.processAlert(testAlert, testMFN)
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
									HostedClusterID:     testconst.TestHostedClusterID,
									ServiceLogSentCount: 1,
									LastTransitionTime:  &metav1.Time{Time: time.Now().Add(time.Duration(-1) * time.Hour)},
								},
							},
						},
					}
					gomock.InOrder(
						// Fetch the MFNR
						mockClient.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).SetArg(2, testMFNR),
					)

					err := testHandler.processAlert(testAlert, testMFN)
					Expect(err).ShouldNot(HaveOccurred())
				})
			})
		})
	})
})
