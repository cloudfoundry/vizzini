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
	var url string

	BeforeEach(func() {
		url = "http://" + RouteForGuid(guid) + "/env?json=true"
		lrp = DesiredLRPWithGuid(guid)
		lrp.Ports = []uint16{8080, 5000}
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
			actualLRP, err := client.ActualLRPByProcessGuidAndIndex(guid, 0)
			Ω(err).ShouldNot(HaveOccurred())

			envs := getEnvs(url)

			Ω(envs).Should(ContainElement([]string{"INSTANCE_INDEX", "0"}))
			Ω(envs).Should(ContainElement([]string{"INSTANCE_GUID", actualLRP.InstanceGuid}))

		})
	})

	//{LOCAL} because: Instance IP and PORT are not injected by default.  One needs to opt-into this feature.
	Describe("{LOCAL} Instance IP and PORT", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url), 40).Should(Equal(http.StatusOK))
		})

		It("matches the ActualLRP's index and instance guid", func() {
			actualLRP, err := client.ActualLRPByProcessGuidAndIndex(guid, 0)
			Ω(err).ShouldNot(HaveOccurred())

			type cfPortMapping struct {
				External uint16 `json:"external"`
				Internal uint16 `json:"internal"`
			}

			cfPortMappingPayload, err := json.Marshal([]cfPortMapping{
				{External: actualLRP.Ports[0].HostPort, Internal: actualLRP.Ports[0].ContainerPort},
				{External: actualLRP.Ports[1].HostPort, Internal: actualLRP.Ports[1].ContainerPort},
			})
			Ω(err).ShouldNot(HaveOccurred())

			envs := getEnvs(url)
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_IP", actualLRP.Address}), "If this fails, then your executor may not be configured to expose ip:port to the container")
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_PORT", fmt.Sprintf("%d", actualLRP.Ports[0].HostPort)}))
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_ADDR", fmt.Sprintf("%s:%d", actualLRP.Address, actualLRP.Ports[0].HostPort)}))
			Ω(envs).Should(ContainElement([]string{"CF_INSTANCE_PORTS", string(cfPortMappingPayload)}))
		})
	})
})
