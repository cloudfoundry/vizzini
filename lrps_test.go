package vizzini_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"
)

func LRPGetter(guid string) func() (receptor.DesiredLRPResponse, error) {
	return func() (receptor.DesiredLRPResponse, error) {
		return client.GetDesiredLRP(guid)
	}
}

func ClearOutDesiredLRPsInDomain(domain string) {
	lrps, err := client.GetAllDesiredLRPsByDomain(domain)
	Ω(err).ShouldNot(HaveOccurred())
	for _, lrp := range lrps {
		Ω(client.DeleteDesiredLRP(lrp.ProcessGuid)).Should(Succeed())
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
						Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}, {"ACTION_LEVEL", "COYOTE"}, {"OVERRIDE", "DAQUIRI"}},
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
				Eventually(client.GetAllActualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))

				fetchedLRP, err := client.GetDesiredLRP(guid)
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

	Describe("Specifying environment variables", func() {
		BeforeEach(func() {
			lrp.EnvironmentVariables = []receptor.EnvironmentVariable{
				{"CONTAINER_LEVEL", "AARDVARK"},
				{"OVERRIDE", "BANANA"},
			}

			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("should be possible to specify environment variables on both the DesiredLRP and the RunAction", func() {
			resp, err := http.Get(url)
			Ω(err).ShouldNot(HaveOccurred())
			body, err := ioutil.ReadAll(resp.Body)
			Ω(err).ShouldNot(HaveOccurred())
			resp.Body.Close()

			Ω(body).Should(ContainSubstring("AARDVARK"))
			Ω(body).Should(ContainSubstring("COYOTE"))
			Ω(body).Should(ContainSubstring("DAQUIRI"))
			Ω(body).ShouldNot(ContainSubstring("BANANA"))
		})
	})

	Describe("Creating a Docker-based LRP", func() {
		BeforeEach(func() {
			lrp.RootFSPath = "docker:///onsi/grace-busybox"
			lrp.Actions = []models.ExecutorAction{
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
							Path: "/grace",
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
			}

			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
		})

		It("should succeed", func() {
			Eventually(EndpointCurler(url), 120).Should(Equal(http.StatusOK), "Docker can be quite slow to spin up...")
			Eventually(client.GetAllActualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
		})
	})

	Describe("Updating an existing DesiredLRP", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		Context("By redesiring it", func() {
			It("allows updating instances", func() {
				lrp.Instances = 2
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
				Eventually(client.GetAllActualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
				Eventually(client.GetAllActualLRPs).Should(ContainElement(BeActualLRP(guid, 1)))
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
				lrp, err := client.GetDesiredLRP(guid)
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
			Context("when the LRP exists", func() {
				It("allows updating instances", func() {
					two := 2
					Ω(client.UpdateDesiredLRP(guid, receptor.DesiredLRPUpdateRequest{
						Instances: &two,
					})).Should(Succeed())
					Eventually(IndexCounter(guid)).Should(Equal(2))
				})

				It("does not allow scaling down to 0", func() {
					zero := 0
					Ω(client.UpdateDesiredLRP(guid, receptor.DesiredLRPUpdateRequest{
						Instances: &zero,
					})).ShouldNot(Succeed())
					Eventually(IndexCounter(guid)).Should(Equal(1))
				})

				It("allows updating routes", func() {
					newRoute := RouteForGuid(NewGuid())
					routes := append(lrp.Routes, newRoute)
					Ω(client.UpdateDesiredLRP(guid, receptor.DesiredLRPUpdateRequest{
						Routes: routes,
					})).Should(Succeed())
					Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
					Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				})

				It("allows updating annotations", func() {
					annotation := "my new annotation"
					Ω(client.UpdateDesiredLRP(guid, receptor.DesiredLRPUpdateRequest{
						Annotation: &annotation,
					})).Should(Succeed())
					lrp, err := client.GetDesiredLRP(guid)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(lrp.Annotation).Should(Equal("my new annotation"))
				})

				It("allows multiple simultaneous updates", func() {
					two := 2
					annotation := "my new annotation"
					newRoute := RouteForGuid(NewGuid())
					routes := append(lrp.Routes, newRoute)
					Ω(client.UpdateDesiredLRP(guid, receptor.DesiredLRPUpdateRequest{
						Instances:  &two,
						Routes:     routes,
						Annotation: &annotation,
					})).Should(Succeed())

					Eventually(IndexCounter(guid)).Should(Equal(2))

					Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
					Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

					lrp, err := client.GetDesiredLRP(guid)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(lrp.Annotation).Should(Equal("my new annotation"))
				})
			})

			Context("when the LRP does not exit", func() {
				It("errors", func() {
					two := 2
					err := client.UpdateDesiredLRP("flooberdoobey", receptor.DesiredLRPUpdateRequest{
						Instances: &two,
					})
					Ω(err.(receptor.Error).Type).Should(Equal(receptor.LRPNotFound))
				})
			})
		})
	})

	Describe("Getting a DesiredLRP", func() {
		Context("when the DesiredLRP exists", func() {
			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			})

			It("should succeed", func() {
				lrp, err := client.GetDesiredLRP(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(lrp.ProcessGuid).Should(Equal(guid))
			})
		})

		Context("when the DesiredLRP does not exist", func() {
			It("should error", func() {
				lrp, err := client.GetDesiredLRP("floobeedoo")
				Ω(lrp).Should(BeZero())
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.LRPNotFound))
			})
		})
	})

	Describe("Getting All DesiredLRPs and Getting DesiredLRPs by Domain", func() {
		var otherGuids []string
		var otherDomain string

		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

			otherDomain = fmt.Sprintf("New-Domain-%d", GinkgoParallelNode())
			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherLRP := DesiredLRPWithGuid(otherGuid)
				otherLRP.Domain = otherDomain
				Ω(client.CreateDesiredLRP(otherLRP)).Should(Succeed())
				url := "http://" + RouteForGuid(otherGuid) + "/env"
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			}
		})

		AfterEach(func() {
			ClearOutDesiredLRPsInDomain(otherDomain)
		})

		It("should fetch desired lrps in the given domain", func() {
			defaultDomain, err := client.GetAllDesiredLRPsByDomain(domain)
			Ω(err).ShouldNot(HaveOccurred())

			otherDomain, err := client.GetAllDesiredLRPsByDomain(otherDomain)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(defaultDomain).Should(HaveLen(1))
			Ω(otherDomain).Should(HaveLen(2))
			Ω([]string{otherDomain[0].ProcessGuid, otherDomain[1].ProcessGuid}).Should(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			domain, err := client.GetAllDesiredLRPsByDomain("farfignoogan")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(domain).Should(BeEmpty())
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

	Describe("Deleting DesiredLRPs", func() {
		Context("when the DesiredLRP exists", func() {
			It("should be deleted", func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

				Ω(client.DeleteDesiredLRP(guid)).Should(Succeed())
				_, err := client.GetDesiredLRP(guid)
				Ω(err).Should(HaveOccurred())
				Eventually(EndpointCurler(url)).ShouldNot(Equal(http.StatusOK))
			})
		})

		Context("when the DesiredLRP does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := client.DeleteDesiredLRP("floobeedoobee")
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.LRPNotFound))
			})
		})
	})

	Describe("Getting all ActualLRPs", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("should fetch all Actual LRPs", func() {
			actualLRPs, err := client.GetAllActualLRPs()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(len(actualLRPs)).Should(BeNumerically(">=", 1))
			Ω(actualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
		})
	})
})
