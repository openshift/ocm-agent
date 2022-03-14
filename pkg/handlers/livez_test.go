package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Livez tests", func() {

	var (
		livezHandler *LivezHandler
		server       *ghttp.Server
	)

	BeforeEach(func() {
		livezHandler = &LivezHandler{}
		server = ghttp.NewServer()
	})

	AfterEach(func() {
		server.Close()
	})

	Context("Livez handler post", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// add handler to the server
			server.AppendHandlers(livezHandler.ServeHTTP)
			// Set data to post
			postData := ""
			// convert AMReceiverData to json for http request
			postDataJson, _ := json.Marshal(postData)
			// post to AMReceiver handler
			resp, err = http.Post(server.URL(), "application/json", bytes.NewBuffer(postDataJson))
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

	Context("Livez handler get", func() {
		var resp *http.Response
		var err error
		BeforeEach(func() {
			// add handler to the server
			server.AppendHandlers(livezHandler.ServeHTTP)
			resp, err = http.Get(server.URL())
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
			expected := LivezResponse{
				Status: "ok",
			}
			// Set response
			var response LivezResponse
			_ = json.NewDecoder(resp.Body).Decode(&response)
			Expect(response).Should(Equal(expected))
		})
	})
})
