package webhookreceiver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gorilla/mux"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

func TestWebhookHandlers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Handler Suite")
}

var _ = Describe("Webhook Handlers", func() {
	var server *ghttp.Server
	BeforeEach(func() {
		server = ghttp.NewServer()
	})
	AfterEach(func() {
		server.Close()
	})
	Context("AMReceiver handler post", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// setup handler
			h := AMReceiver()
			h.AddRoute(mux.NewRouter())
			// add handler to the server
			server.AppendHandlers(h.Func)
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
			}
			// Set response
			var response AMReceiverResponse
			json.NewDecoder(resp.Body).Decode(&response)
			Expect(response).Should(Equal(expected))
		})
	})
	Context("AMReceiver handler get", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// setup handler
			h := AMReceiver()
			h.AddRoute(mux.NewRouter())
			// add handler to the server
			server.AppendHandlers(h.Func)
			resp, err = http.Get(server.URL())
		})
		It("Returns the correct http status code", func() {
			Expect(err).ShouldNot(HaveOccurred())
			Expect(resp.StatusCode).Should(Equal(http.StatusMethodNotAllowed))
		})
	})
})
