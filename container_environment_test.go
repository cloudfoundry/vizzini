package vizzini_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/receptor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("The container environment", func() {
	var lrp receptor.DesiredLRPCreateRequest
	var guid, url string

	BeforeEach(func() {
		guid = NewGuid()
		url = "http://" + RouteForGuid(guid) + "/env?json=true"
		lrp = DesiredLRPWithGuid(guid)
		lrp.Ports = []uint32{8080, 5000}
	})

	AfterEach(func() {
		ClearOutDesiredLRPsInDomain(domain)
	})

	getEnvs := func(url string) [][]string {
		response, err := http.Get(url)
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusOK))
		envs := [][]string{}
		err = json.NewDecoder(response.Body).Decode(&envs)
		Ω(err).ShouldNot(HaveOccurred())
		response.Body.Close()
		return envs
	}

	Describe("InstanceGuid and InstanceIndex", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("matches the ActualLRP's index and instance guid", func() {
			actualLRPs, err := client.ActualLRPsByProcessGuidAndIndex(guid, 0)
			Ω(err).ShouldNot(HaveOccurred())
			actualLRP := actualLRPs[0]

			envs := getEnvs(url)

			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_INDEX", "0"}))
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_GUID", actualLRP.InstanceGuid}))

		})
	})

	Describe("InstanceGuid and InstanceIndex", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url), 40).Should(Equal(http.StatusOK))
		})

		It("matches the ActualLRP's index and instance guid", func() {
			actualLRPs, err := client.ActualLRPsByProcessGuidAndIndex(guid, 0)
			Ω(err).ShouldNot(HaveOccurred())
			actualLRP := actualLRPs[0]

			envs := getEnvs(url)
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_IP", actualLRP.Host}))
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_PORT", fmt.Sprintf("%d", actualLRP.Ports[0].HostPort)}))
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_ADDR", fmt.Sprintf("%s:%d", actualLRP.Host, actualLRP.Ports[0].HostPort)}))
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_PORTS", fmt.Sprintf("%d:%d,%d:%d", actualLRP.Ports[0].HostPort, actualLRP.Ports[0].ContainerPort, actualLRP.Ports[1].HostPort, actualLRP.Ports[1].ContainerPort)}))
		})
	})
})
