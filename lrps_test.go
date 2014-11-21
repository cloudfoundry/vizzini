package vizzini_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

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

func ActualGetter(guid string, index int) func() (receptor.ActualLRPResponse, error) {
	return func() (receptor.ActualLRPResponse, error) {
		actuals, err := client.ActualLRPsByProcessGuidAndIndex(guid, index)
		if err != nil {
			return receptor.ActualLRPResponse{}, err
		}

		if len(actuals) == 0 {
			return receptor.ActualLRPResponse{}, nil
		}

		if len(actuals) != 1 {
			return receptor.ActualLRPResponse{}, errors.New("found too many ActualLRPs")
		}

		return actuals[0], nil
	}
}

func ClearOutDesiredLRPsInDomain(domain string) {
	lrps, err := client.DesiredLRPsByDomain(domain)
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

func StartedAtGetter(guid string) func() (int64, error) {
	url := "http://" + RouteForGuid(guid) + "/started-at"
	return func() (int64, error) {
		resp, err := http.Get(url)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return 0, errors.New(fmt.Sprintf("invalid status code: %d", resp.StatusCode))
		}
		content, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}
		return strconv.ParseInt(string(content), 10, 64)
	}
}

func RouteForGuid(guid string) string {
	return fmt.Sprintf("%s.10.244.0.34.xip.io", guid)
}

func DesiredLRPWithGuid(guid string) receptor.DesiredLRPCreateRequest {
	return receptor.DesiredLRPCreateRequest{
		ProcessGuid: guid,
		RootFSPath:  rootFS,
		Domain:      domain,
		Instances:   1,
		Setup: &models.SerialAction{
			Actions: []models.Action{
				&models.DownloadAction{
					From:     "http://onsi-public.s3.amazonaws.com/grace.tar.gz",
					To:       ".",
					CacheKey: "grace",
				},
				&models.DownloadAction{
					From:     "http://file_server.service.dc1.consul:8080/v1/static/linux-circus/linux-circus.tgz",
					To:       "/tmp/circus",
					CacheKey: "linux-circus",
				},
			},
		},
		Action: &models.RunAction{
			Path: "./grace",
			Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}, {"ACTION_LEVEL", "COYOTE"}, {"OVERRIDE", "DAQUIRI"}},
		},
		Monitor: &models.RunAction{
			Path: "/tmp/circus/spy",
			Args: []string{"-addr=:8080"},
		},
		Stack:     stack,
		MemoryMB:  128,
		DiskMB:    128,
		CPUWeight: 100,
		Ports:     []uint32{8080},
		Routes: []string{
			RouteForGuid(guid),
		},
		LogGuid:    guid,
		LogSource:  "VIZ",
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
				Eventually(client.ActualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))

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

				By("not having an action")
				lrpCopy = lrp
				lrpCopy.Action = nil
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

		Context("when the annotation is too large", func() {
			It("should fail", func() {
				lrp.Annotation = strings.Repeat("7", 1024*10+1)
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

	Describe("{DOCKER} Creating a Docker-based LRP", func() {
		BeforeEach(func() {
			lrp.RootFSPath = "docker:///onsi/grace-busybox"
			lrp.Setup = &models.DownloadAction{
				From:     "http://file_server.service.dc1.consul:8080/v1/static/linux-circus/linux-circus.tgz",
				To:       "/tmp/circus",
				CacheKey: "linux-circus",
			}
			lrp.Action = &models.RunAction{
				Path: "/grace",
				Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			}
			lrp.Monitor = &models.RunAction{
				Path: "/tmp/circus/spy",
				Args: []string{"-addr=:8080"},
			}

			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
		})

		It("should succeed", func() {
			Eventually(EndpointCurler(url), 120).Should(Equal(http.StatusOK), "Docker can be quite slow to spin up...")
			Eventually(client.ActualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
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
				Eventually(client.ActualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
				Eventually(client.ActualLRPs).Should(ContainElement(BeActualLRP(guid, 1)))
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
			defaultDomain, err := client.DesiredLRPsByDomain(domain)
			Ω(err).ShouldNot(HaveOccurred())

			otherDomain, err := client.DesiredLRPsByDomain(otherDomain)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(defaultDomain).Should(HaveLen(1))
			Ω(otherDomain).Should(HaveLen(2))
			Ω([]string{otherDomain[0].ProcessGuid, otherDomain[1].ProcessGuid}).Should(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			domain, err := client.DesiredLRPsByDomain("farfignoogan")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(domain).Should(BeEmpty())
		})

		It("should fetch all desired lrps", func() {
			allDesiredLRPs, err := client.DesiredLRPs()
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
			actualLRPs, err := client.ActualLRPs()
			Ω(err).ShouldNot(HaveOccurred())
			Ω(len(actualLRPs)).Should(BeNumerically(">=", 1))
			Ω(actualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
		})
	})

	Describe("Getting ActualLRPs by Domain", func() {
		Context("when the domain is empty", func() {
			It("returns an empty list", func() {
				Ω(client.ActualLRPsByDomain("floobidoo")).Should(BeEmpty())
			})
		})

		Context("when the domain contains instances", func() {
			var secondDomain string
			var secondDomainLRP1 receptor.DesiredLRPCreateRequest
			var secondDomainLRP2 receptor.DesiredLRPCreateRequest

			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				secondDomain = NewGuid()

				secondDomainLRP1 = DesiredLRPWithGuid(NewGuid())
				secondDomainLRP1.Instances = 2
				secondDomainLRP1.Domain = secondDomain
				Ω(client.CreateDesiredLRP(secondDomainLRP1)).Should(Succeed())

				secondDomainLRP2 = DesiredLRPWithGuid(NewGuid())
				secondDomainLRP2.Domain = secondDomain
				Ω(client.CreateDesiredLRP(secondDomainLRP2)).Should(Succeed())

				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				Eventually(IndexCounter(secondDomainLRP1.ProcessGuid)).Should(Equal(2))
				Eventually(IndexCounter(secondDomainLRP2.ProcessGuid)).Should(Equal(1))
			})

			AfterEach(func() {
				ClearOutDesiredLRPsInDomain(secondDomain)
			})

			It("returns said instances", func() {
				actualLRPs, err := client.ActualLRPsByDomain(domain)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(actualLRPs).Should(HaveLen(1))
				Ω(actualLRPs).Should(ConsistOf(BeActualLRP(guid, 0)))

				actualLRPs, err = client.ActualLRPsByDomain(secondDomain)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(actualLRPs).Should(HaveLen(3))

				Ω(actualLRPs).Should(ConsistOf(
					BeActualLRP(secondDomainLRP1.ProcessGuid, 0),
					BeActualLRP(secondDomainLRP1.ProcessGuid, 1),
					BeActualLRP(secondDomainLRP2.ProcessGuid, 0),
				))
			})
		})
	})

	Describe("Getting ActualLRPs by ProcessGuid", func() {
		Context("when there are none", func() {
			It("returns an empty list", func() {
				Ω(client.ActualLRPsByProcessGuid("floobeedoo")).Should(BeEmpty())
			})
		})

		Context("when there are ActualLRPs for a given ProcessGuid", func() {
			BeforeEach(func() {
				lrp.Instances = 2
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("returns the ActualLRPs", func() {
				Ω(client.ActualLRPsByProcessGuid(guid)).Should(ConsistOf(
					BeActualLRP(guid, 0),
					BeActualLRP(guid, 1),
				))
			})
		})
	})

	Describe("Getting ActualLRPs at a given index for a ProcessGuid", func() {
		Context("when there is no matching ProcessGuid", func() {
			It("should return an empty list", func() {
				Ω(client.ActualLRPsByProcessGuidAndIndex("floobeedoo", 0)).Should(BeEmpty())
			})
		})

		Context("when there are no ActualLRPs at the given index", func() {
			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(1))
			})

			It("should return an empty list", func() {
				Ω(client.ActualLRPsByProcessGuidAndIndex(guid, 1)).Should(BeEmpty())
			})
		})

		Context("when there are ActualLRPs at the given index", func() {
			BeforeEach(func() {
				lrp.Instances = 2
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("returns them", func() {
				Ω(client.ActualLRPsByProcessGuidAndIndex(guid, 0)).Should(ConsistOf(BeActualLRP(guid, 0)))
				Ω(client.ActualLRPsByProcessGuidAndIndex(guid, 1)).Should(ConsistOf(BeActualLRP(guid, 1)))
			})
		})
	})

	Describe("Restarting an ActualLRP", func() {
		Context("when there is no matching ProcessGuid", func() {
			It("returns an error", func() {
				Ω(client.KillActualLRPsByProcessGuidAndIndex(guid, 0)).ShouldNot(Succeed())
			})
		})

		Context("when there is no ActualLRP at the given index", func() {
			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("returns an error", func() {
				Ω(client.KillActualLRPsByProcessGuidAndIndex(guid, 1)).ShouldNot(Succeed())
			})
		})

		Context("{SLOW} when an ActualLRP exists at the given ProcessGuid and index", func() {
			BeforeEach(func() {
				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("restarts the actual lrp", func() {
				initialTime, err := StartedAtGetter(guid)()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(client.KillActualLRPsByProcessGuidAndIndex(guid, 0)).Should(Succeed())
				//This needs a large timeout as the converger needs to run for it to return
				Eventually(StartedAtGetter(guid), 35).Should(BeNumerically(">", initialTime))
			})
		})
	})
})
