package vizzini_test

import (
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	. "code.cloudfoundry.org/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLS Proxy", func() {
	var lrp *models.DesiredLRP

	BeforeEach(func() {
		if !enableContainerProxyTests {
			Skip("container proxy tests are disabled")
		}

		lrp = DesiredLRPWithGuid(guid)
		Expect(bbsClient.DesireLRP(logger, lrp)).To(Succeed())
		Eventually(ActualGetter(logger, guid, 0)).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
	})

	It("proxies traffic to the application process inside the container", func() {
		directURL := "http://" + TLSDirectAddressFor(guid, 0, 8080)

		resp, err := http.Get(directURL)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
