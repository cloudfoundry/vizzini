package receptor_suite_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func LRPGetter(guid string) func() (receptor.DesiredLRPResponse, error) {
	return func() (receptor.DesiredLRPResponse, error) {
		return GetDesiredLRP(guid)
	}
}

func GetDesiredLRP(guid string) (receptor.DesiredLRPResponse, error) {
	lrps, err := client.GetAllDesiredLRPs()
	if err != nil {
		return receptor.DesiredLRPResponse{}, err
	}
	for _, lrp := range lrps {
		if lrp.ProcessGuid == guid {
			return lrp, nil
		}
	}
	return receptor.DesiredLRPResponse{}, errors.New("no lrp found")
}

func GetDesiredLRPsInDomain(domain string) ([]receptor.DesiredLRPResponse, error) {
	lrps, err := client.GetAllDesiredLRPs()
	if err != nil {
		return nil, err
	}
	filteredLRPs := []receptor.DesiredLRPResponse{}
	for _, lrp := range lrps {
		if lrp.Domain == domain {
			filteredLRPs = append(filteredLRPs, lrp)
		}
	}

	return filteredLRPs, nil
}

func ClearOutDesiredLRPsInDomain(domain string) {
	lrps, err := GetDesiredLRPsInDomain(domain)
	Ω(err).ShouldNot(HaveOccurred())
	for _, lrp := range lrps {
		Ω(temporaryBBS.RemoveDesiredLRPByProcessGuid(lrp.ProcessGuid)).Should(Succeed())
	}
}

func EndpointCurler(endpoint string) func() (int, error) {
	return func() (int, error) {
		resp, err := http.Get(endpoint)
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}
}

func IndexCounter(guid string) func() (int, error) {
	url := "http://" + RouteForGuid(guid) + "/index"
	return func() (int, error) {
		counts := map[string]bool{}
		for i := 0; i < 40; i++ {
			resp, err := http.Get(url)
			if err != nil {
				return 0, err
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return 0, err
			}
			content, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return 0, err
			}
			counts[string(content)] = true
		}
		return len(counts), nil
	}
}

func RouteForGuid(guid string) string {
	return fmt.Sprintf("%s.10.244.0.34.xip.io", guid)
}

func DesiredLRPWithGuid(guid string) receptor.DesiredLRPCreateRequest {
	return receptor.DesiredLRPCreateRequest{
		ProcessGuid: guid,
		Domain:      domain,
		Instances:   1,
		Actions: []models.ExecutorAction{
			{
				models.DownloadAction{
					From:     "http://onsi-public.s3.amazonaws.com/grace.tar.gz",
					To:       ".",
					CacheKey: "grace",
				},
			},
			{
				models.DownloadAction{
					From:     "http://file_server.service.dc1.consul:8080/v1/static/linux-circus/linux-circus.tgz",
					To:       "/tmp/circus",
					CacheKey: "linux-circus",
				},
			},
			models.Parallel(
				models.ExecutorAction{
					models.RunAction{
						Path: "./grace",
						Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					},
				},
				models.ExecutorAction{
					models.MonitorAction{
						Action: models.ExecutorAction{
							models.RunAction{
								Path: "/tmp/circus/spy",
								Args: []string{"-addr=:8080"},
							},
						},
						HealthyHook:        models.HealthRequest{Method: "PUT", URL: "http://127.0.0.1:20515/lrp_running/" + guid + "/PLACEHOLDER_INSTANCE_INDEX/PLACEHOLDER_INSTANCE_GUID"},
						HealthyThreshold:   1,
						UnhealthyThreshold: 1,
					},
				},
			),
		},
		Stack:     stack,
		MemoryMB:  128,
		DiskMB:    128,
		CPUWeight: 100,
		Ports: []receptor.PortMapping{
			{ContainerPort: 8080},
		},
		Routes: []string{
			RouteForGuid(guid),
		},
		Log: receptor.LogConfig{
			Guid:       guid,
			SourceName: "VIZ",
		},
		Annotation: "arbitrary-data",
	}
}

var _ = Describe("LRPs", func() {
	var lrp receptor.DesiredLRPCreateRequest
	var guid, url string

	BeforeEach(func() {
		guid = NewGuid()
		url = "http://" + RouteForGuid(guid) + "/env"
		lrp = DesiredLRPWithGuid(guid)
	})

	AfterEach(func() {
		ClearOutDesiredLRPsInDomain(domain)
	})

	Describe("Desiring LRPs", func() {
		Context("when the LRP is well-formed", func() {
			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			})

			It("desires the LRP", func() {
				Eventually(LRPGetter(guid)).ShouldNot(BeZero())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				fetchedLRP, err := GetDesiredLRP(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(fetchedLRP.Annotation).Should(Equal("arbitrary-data"))
			})
		})

		Context("when the LRP's process guid contains invalid characters", func() {
			It("should fail to create", func() {
				lrp.ProcessGuid = "abc def"
				err := client.CreateDesiredLRP(lrp)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.InvalidLRP))

				lrp.ProcessGuid = "abc/def"
				Ω(client.CreateDesiredLRP(lrp)).ShouldNot(Succeed())

				lrp.ProcessGuid = "abc,def"
				Ω(client.CreateDesiredLRP(lrp)).ShouldNot(Succeed())

				lrp.ProcessGuid = "abc.def"
				Ω(client.CreateDesiredLRP(lrp)).ShouldNot(Succeed())

				lrp.ProcessGuid = "abc∆def"
				Ω(client.CreateDesiredLRP(lrp)).ShouldNot(Succeed())
			})
		})

		Context("when the LRP's # of instances is <= 0", func() {
			It("should fail to create", func() {
				lrp.Instances = 0
				err := client.CreateDesiredLRP(lrp)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.InvalidLRP))
			})
		})

		Context("when required fields are missing", func() {
			It("should fail to create", func() {
				By("not having ProcessGuid")
				lrpCopy := lrp
				lrpCopy.ProcessGuid = ""
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())

				By("not having a domain")
				lrpCopy = lrp
				lrpCopy.Domain = ""
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())

				By("not having any actions")
				lrpCopy = lrp
				lrpCopy.Actions = nil
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())
				lrpCopy.Actions = []models.ExecutorAction{}
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())

				By("not having a stack")
				lrpCopy = lrp
				lrpCopy.Stack = ""
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())
			})
		})

		Context("when the CPUWeight is out of bounds", func() {
			It("should fail", func() {
				lrp.CPUWeight = 101
				err := client.CreateDesiredLRP(lrp)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.InvalidLRP))
			})
		})
	})

	Describe("Updating an existing DesiredLRP", func() {
		Context("By redesiring it", func() {
			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("allows updating instances", func() {
				lrp.Instances = 2
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("allows updating routes", func() {
				newRoute := RouteForGuid(NewGuid())
				lrp.Routes = append(lrp.Routes, newRoute)
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("allows updating annotations", func() {
				lrp.Annotation = "my new annotation"
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				lrp, err := GetDesiredLRP(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(lrp.Annotation).Should(Equal("my new annotation"))
			})

			It("disallows updating anything else", func() {
				lrpCopy := lrp
				lrpCopy.Domain = NewGuid()
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())

				lrpCopy.Stack = ".net"
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())

				lrpCopy.MemoryMB = 256
				Ω(client.CreateDesiredLRP(lrpCopy)).ShouldNot(Succeed())
			})
		})

		Context("By explicitly updating it", func() {

		})
	})

	Describe("Getting All LRPs and Getting LRPs by Domain", func() {
		var otherGuids []string
		var otherDomain string

		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

			otherDomain = fmt.Sprintf("New-Domain-%d", GinkgoParallelNode())
			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherLRP := DesiredLRPWithGuid(otherGuid)
				Ω(client.CreateDesiredLRP(otherLRP)).Should(Succeed())
				url := "http://" + RouteForGuid(otherGuid) + "/env"
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			}
		})

		AfterEach(func() {
			ClearOutDesiredLRPsInDomain(otherDomain)
		})

		PIt("should fetch desired lrps in the given domain", func() {
		})

		PIt("should not error if a domain is empty", func() {
		})

		It("should fetch all desired lrps", func() {
			allDesiredLRPs, err := client.GetAllDesiredLRPs()
			Ω(err).ShouldNot(HaveOccurred())

			//if we're running in parallel there may be more than 3 things here!
			Ω(len(allDesiredLRPs)).Should(BeNumerically(">=", 3))
			lrpGuids := []string{}
			for _, lrp := range allDesiredLRPs {
				lrpGuids = append(lrpGuids, lrp.ProcessGuid)
			}
			Ω(lrpGuids).Should(ContainElement(guid))
			Ω(lrpGuids).Should(ContainElement(otherGuids[0]))
			Ω(lrpGuids).Should(ContainElement(otherGuids[1]))
		})
	})
})
