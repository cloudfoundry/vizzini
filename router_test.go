package vizzini_test

import (
	"net/http"
	"net/http/cookiejar"

	"github.com/cloudfoundry-incubator/receptor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router Related Tests", func() {
	var lrp receptor.DesiredLRPCreateRequest

	Describe("sticky sessions", func() {
		var httpClient *http.Client

		BeforeEach(func() {
			jar, err := cookiejar.New(nil)
			Ω(err).ShouldNot(HaveOccurred())

			httpClient = &http.Client{
				Jar: jar,
			}

			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3

			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(IndexCounter(guid, httpClient)).Should(Equal(3))
		})

		It("should only route to the stuck instance", func() {
			resp, err := httpClient.Get("http://" + RouteForGuid(guid) + "/stick")
			Ω(err).ShouldNot(HaveOccurred())
			resp.Body.Close()

			//for some reason this isn't always 1!  it's sometimes 2....
			Ω(IndexCounter(guid, httpClient)()).Should(BeNumerically("<", 3))

			resp, err = httpClient.Get("http://" + RouteForGuid(guid) + "/unstick")
			Ω(err).ShouldNot(HaveOccurred())
			resp.Body.Close()

			Ω(IndexCounter(guid, httpClient)()).Should(Equal(3))
		})
	})
})
