package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/golang/mock/gomock"
	sdk "github.com/openshift-online/ocm-sdk-go"
	oav1alpha1 "github.com/openshift/ocm-agent-operator/pkg/apis/ocmagent/v1alpha1"
	"github.com/prometheus/alertmanager/template"

	testconst "github.com/openshift/ocm-agent/pkg/consts/test"
	clientmocks "github.com/openshift/ocm-agent/pkg/util/test/generated/mocks/client"
)

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (fn RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

const (
	dummyJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhbGciOiJIUzI1NiIsInR5cCI6IkJlYXJlciJ9.0rLPJ-zaY_wFsADkvKmW5nsZzeyFmCP0276XSrkctb4"
)

var _ = Describe("Webhook Handlers", func() {

	var (
		mockCtrl               *gomock.Controller
		mockClient             *clientmocks.MockClient
		testConn               *sdk.Connection
		webhookReceiverHandler *WebhookReceiverHandler
		server                 *ghttp.Server
		testAlert              template.Alert
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = clientmocks.NewMockClient(mockCtrl)
		server = ghttp.NewServer()
		testConn, _ = sdk.NewConnectionBuilder().Tokens(dummyJWT).TransportWrapper(
			func(tripper http.RoundTripper) http.RoundTripper {
				return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
					// Assert on request attributes
					// Return a response or error you want
					return &http.Response{}, nil
				})
			},
		).Build()
		webhookReceiverHandler = &WebhookReceiverHandler{
			c:   mockClient,
			ocm: testConn,
		}
		testAlert = testconst.TestAlert
	})
	AfterEach(func() {
		server.Close()
	})
	Context("AMReceiver handler post", func() {
		//var resp *http.Response
		//var err error
		//BeforeEach(func() {
		//	// add handler to the server
		//	server.AppendHandlers(webhookReceiverHandler.ServeHTTP)
		//	// Set data to post
		//	postData := AMReceiverData{
		//		Status: "foo",
		//	}
		//	// convert AMReceiverData to json for http request
		//	postDataJson, _ := json.Marshal(postData)
		//	// post to AMReceiver handler
		//	resp, err = http.Post(server.URL(), "application/json", bytes.NewBuffer(postDataJson))
		//})
		//It("Returns the correct http status code", func() {
		//	Expect(err).ShouldNot(HaveOccurred())
		//	Expect(resp.StatusCode).Should(Equal(http.StatusOK))
		//})
		//It("Returns the correct content type", func() {
		//	Expect(err).ShouldNot(HaveOccurred())
		//	Expect(resp.Header.Get("Content-Type")).Should(Equal("application/json"))
		//})
		//It("Returns the correct response", func() {
		//	Expect(err).ShouldNot(HaveOccurred())
		//	// Set expected
		//	expected := AMReceiverResponse{
		//		Status: "ok",
		//	}
		//	// Set response
		//	var response AMReceiverResponse
		//	_ = json.NewDecoder(resp.Body).Decode(&response)
		//	Expect(response).Should(Equal(expected))
		//})
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
	})

	Context("When looking for a matching notification for an alert", func() {
		It("will return one if one exists", func() {
			mnl := &oav1alpha1.ManagedNotificationList{
				Items: []oav1alpha1.ManagedNotification{
					testconst.TestManagedNotification,
				},
			}
			n, mn, err := getNotification(testconst.TestNotificationName, mnl)
			Expect(err).To(BeNil())
			Expect(reflect.DeepEqual(*n, testconst.TestNotification)).To(BeTrue())
			Expect(reflect.DeepEqual(*mn, testconst.TestManagedNotification)).To(BeTrue())
		})
	})
})
