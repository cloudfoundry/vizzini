package vizzini_test

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry-incubator/receptor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("FuseFS", func() {
	var lrp receptor.DesiredLRPCreateRequest
	var url string

	BeforeEach(func() {
		lrp = DesiredLRPWithGuid(guid)
		lrp.Privileged = true
		url = "http://" + RouteForGuid(guid) + "/env"

		Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
		Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
	})

	It("should support FuseFS", func() {
		resp, err := http.Post("http://"+RouteForGuid(guid)+"/fuse-fs/mount", "application/json", nil)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(resp.StatusCode).Should(Equal(http.StatusOK))

		resp, err = http.Get("http://" + RouteForGuid(guid) + "/fuse-fs/ls")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(resp.StatusCode).Should(Equal(http.StatusOK))
		contents, err := ioutil.ReadAll(resp.Body)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(contents).Should(ContainSubstring("fuse-fs-works.txt"))
	})
})
