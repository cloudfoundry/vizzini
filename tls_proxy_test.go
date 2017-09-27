package vizzini_test

import (
	"crypto/tls"
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
		directURL := "https://" + TLSDirectAddressFor(guid, 0, 8080)

		resp, err := http.Get(directURL)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	It("verifies the connection cert", func() {
		conn, err := tls.Dial("tcp", TLSDirectAddressFor(guid, 0, 8080), nil)
		Expect(err).NotTo(HaveOccurred())

		err = conn.Handshake()
		Expect(err).NotTo(HaveOccurred())

		connState := conn.ConnectionState()
		Expect(connState.HandshakeComplete).To(BeTrue())
		certs := connState.PeerCertificates
		Expect(certs).To(HaveLen(1))
		lrp, err := ActualGetter(logger, guid, 0)()
		Expect(err).NotTo(HaveOccurred())
		Expect(certs[0].Subject.CommonName).To(Equal(lrp.InstanceGuid))
	})
})
