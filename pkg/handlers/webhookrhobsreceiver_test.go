package handlers

import (
	"context"

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
		testAlert = testconst.TestFleetAlert
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
					mockOCMClient.EXPECT().SendServiceLog(testconst.TestFleetNotification.Summary,
						testconst.TestFleetNotification.NotificationMessage,
						"",
						testconst.TestHostedClusterID, true),
					mockClient.EXPECT().Status().Return(mockStatusWriter),
					mockStatusWriter.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil),
				)

				err := testHandler.processAlert(testAlert, testconst.TestManagedFleetNotification)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})
})
