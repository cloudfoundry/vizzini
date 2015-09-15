package vizzini_test

import (
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/route-emitter/cfroutes"
	. "github.com/cloudfoundry-incubator/vizzini/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRPs", func() {
	var lrp *models.DesiredLRP
	var url string

	BeforeEach(func() {
		url = "http://" + RouteForGuid(guid) + "/env"
		lrp = DesiredLRPWithGuid(guid)
	})

	Describe("Desiring LRPs", func() {
		Context("when the LRP is well-formed", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
			})

			It("desires the LRP", func() {
				Eventually(LRPGetter(guid)).ShouldNot(BeZero())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				Eventually(ActualByDomainGetter(domain)).Should(ContainElement(BeActualLRP(guid, 0)))

				fetchedLRP, err := bbsClient.DesiredLRPByProcessGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(fetchedLRP.Annotation).Should(Equal("arbitrary-data"))
			})
		})

		Context("when the LRP's process guid contains invalid characters", func() {
			It("should fail to create", func() {
				lrp.ProcessGuid = "abc def"
				err := bbsClient.DesireLRP(lrp)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidRequest))

				lrp.ProcessGuid = "abc/def"
				Ω(bbsClient.DesireLRP(lrp)).ShouldNot(Succeed())

				lrp.ProcessGuid = "abc,def"
				Ω(bbsClient.DesireLRP(lrp)).ShouldNot(Succeed())

				lrp.ProcessGuid = "abc.def"
				Ω(bbsClient.DesireLRP(lrp)).ShouldNot(Succeed())

				lrp.ProcessGuid = "abc∆def"
				Ω(bbsClient.DesireLRP(lrp)).ShouldNot(Succeed())
			})
		})

		Context("when the LRP's # of instances is == 0", func() {
			It("should create the LRP and allow the user to subsequently scale up", func() {
				lrp.Instances = 0
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())

				two := int32(2)
				Ω(bbsClient.UpdateDesiredLRP(lrp.ProcessGuid, &models.DesiredLRPUpdate{
					Instances: &two,
				})).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})
		})

		Context("when required fields are missing", func() {
			It("should fail to create", func() {
				lrpCopy := new(models.DesiredLRP)

				By("not having ProcessGuid")
				*lrpCopy = *lrp
				lrpCopy.ProcessGuid = ""
				Ω(bbsClient.DesireLRP(lrpCopy)).ShouldNot(Succeed())

				By("not having a domain")
				*lrpCopy = *lrp
				lrpCopy.Domain = ""
				Ω(bbsClient.DesireLRP(lrpCopy)).ShouldNot(Succeed())

				By("not having an action")
				*lrpCopy = *lrp
				lrpCopy.Action = nil
				Ω(bbsClient.DesireLRP(lrpCopy)).ShouldNot(Succeed())

				By("not having a rootfs")
				*lrpCopy = *lrp
				lrpCopy.RootFs = ""
				Ω(bbsClient.DesireLRP(lrpCopy)).ShouldNot(Succeed())

				By("having a malformed rootfs")
				*lrpCopy = *lrp
				lrpCopy.RootFs = "ploop"
				Ω(bbsClient.DesireLRP(lrpCopy)).ShouldNot(Succeed())
			})
		})

		Context("when the CPUWeight is out of bounds", func() {
			It("should fail", func() {
				lrp.CpuWeight = 101
				err := bbsClient.DesireLRP(lrp)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidRequest))
			})
		})

		Context("when the annotation is too large", func() {
			It("should fail", func() {
				lrp.Annotation = strings.Repeat("7", 1024*10+1)
				err := bbsClient.DesireLRP(lrp)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidRequest))
			})
		})

		Context("when the DesiredLRP already exists", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("should fail", func() {
				err := bbsClient.DesireLRP(lrp)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceExists))
			})
		})
	})

	Describe("Specifying environment variables", func() {
		BeforeEach(func() {
			lrp.EnvironmentVariables = []*models.EnvironmentVariable{
				{"CONTAINER_LEVEL", "AARDVARK"},
				{"OVERRIDE", "BANANA"},
			}

			Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
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

	Describe("Specifying HTTP-based health check (move to inigo or DATs once CC can specify an HTTP-based health-check)", func() {
		BeforeEach(func() {
			lrp.Setup = models.WrapAction(models.Serial(
				&models.DownloadAction{
					From:     "http://onsi-public.s3.amazonaws.com/grace.tar.gz",
					To:       ".",
					CacheKey: "grace",
					User:     "vcap",
				},
				&models.DownloadAction{
					From:     "http://file-server.service.cf.internal:8080/v1/static/buildpack_app_lifecycle/buildpack_app_lifecycle.tgz",
					To:       "/tmp/lifecycle",
					CacheKey: "buildpack-app-lifecycle",
					User:     "vcap",
				},
			))
			lrp.Monitor = models.WrapAction(&models.RunAction{
				Path: "/tmp/lifecycle/healthcheck",
				Args: []string{"-port=8080", "-uri=/ping"},
				User: "vcap",
			})

			Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
		})

		It("should run", func() {
			Eventually(ActualByDomainGetter(domain)).Should(ContainElement(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning)))
		})
	})

	Describe("{DOCKER} Creating a Docker-based LRP", func() {
		BeforeEach(func() {
			lrp.RootFs = "docker:///onsi/grace-busybox"
			lrp.Setup = models.WrapAction(&models.DownloadAction{
				From:     "http://file-server.service.cf.internal:8080/v1/static/docker_app_lifecycle/docker_app_lifecycle.tgz",
				To:       "/tmp/lifecycle",
				CacheKey: "docker-app-lifecycle",
				User:     "root",
			})
			lrp.Action = models.WrapAction(&models.RunAction{
				Path: "/grace",
				User: "root",
				Env:  []*models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			})
			lrp.Monitor = models.WrapAction(&models.RunAction{
				Path: "/tmp/lifecycle/healthcheck",
				Args: []string{"-port=8080"},
				User: "root",
			})

			Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
		})

		It("should succeed", func() {
			Eventually(EndpointCurler(url), 120).Should(Equal(http.StatusOK), "Docker can be quite slow to spin up...")
			Eventually(ActualByDomainGetter(domain)).Should(ContainElement(BeActualLRP(guid, 0)))
		})
	})

	Describe("Updating an existing DesiredLRP", func() {
		var tag *models.ModificationTag

		BeforeEach(func() {
			Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			fetchedLRP, err := bbsClient.DesiredLRPByProcessGuid(lrp.ProcessGuid)
			Ω(err).ShouldNot(HaveOccurred())
			tag = fetchedLRP.ModificationTag
		})

		Context("By explicitly updating it", func() {
			Context("when the LRP exists", func() {
				It("allows updating instances", func() {
					two := int32(2)
					Ω(bbsClient.UpdateDesiredLRP(guid, &models.DesiredLRPUpdate{
						Instances: &two,
					})).Should(Succeed())
					Eventually(IndexCounter(guid)).Should(Equal(2))
				})

				It("allows scaling down to 0", func() {
					zero := int32(0)
					Ω(bbsClient.UpdateDesiredLRP(guid, &models.DesiredLRPUpdate{
						Instances: &zero,
					})).Should(Succeed())
					Eventually(IndexCounter(guid)).Should(Equal(0))
				})

				It("allows updating routes", func() {
					newRoute := RouteForGuid(NewGuid())
					routes, err := cfroutes.CFRoutesFromRoutingInfo(lrp.Routes)
					Ω(err).ShouldNot(HaveOccurred())

					routes[0].Hostnames = append(routes[0].Hostnames, newRoute)

					Ω(bbsClient.UpdateDesiredLRP(guid, &models.DesiredLRPUpdate{
						Routes: routes.RoutingInfo(),
					})).Should(Succeed())

					Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
					Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				})

				It("allows updating annotations", func() {
					annotation := "my new annotation"
					Ω(bbsClient.UpdateDesiredLRP(guid, &models.DesiredLRPUpdate{
						Annotation: &annotation,
					})).Should(Succeed())
					lrp, err := bbsClient.DesiredLRPByProcessGuid(guid)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(lrp.Annotation).Should(Equal("my new annotation"))
				})

				It("allows multiple simultaneous updates", func() {
					two := int32(2)
					annotation := "my new annotation"
					newRoute := RouteForGuid(NewGuid())
					routes, err := cfroutes.CFRoutesFromRoutingInfo(lrp.Routes)
					Ω(err).ShouldNot(HaveOccurred())
					routes[0].Hostnames = append(routes[0].Hostnames, newRoute)
					Ω(bbsClient.UpdateDesiredLRP(guid, &models.DesiredLRPUpdate{
						Instances:  &two,
						Routes:     routes.RoutingInfo(),
						Annotation: &annotation,
					})).Should(Succeed())

					Eventually(IndexCounter(guid)).Should(Equal(2))

					Eventually(EndpointCurler("http://" + newRoute + "/env")).Should(Equal(http.StatusOK))
					Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

					lrp, err := bbsClient.DesiredLRPByProcessGuid(guid)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(lrp.Annotation).Should(Equal("my new annotation"))
				})

				It("updates the modification index when a change occurs", func() {
					By("not modifying if no change has been made")
					fetchedLRP, err := bbsClient.DesiredLRPByProcessGuid(lrp.ProcessGuid)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(fetchedLRP.ModificationTag).Should(Equal(tag))

					By("modifying when a change is made")
					two := int32(2)
					Ω(bbsClient.UpdateDesiredLRP(guid, &models.DesiredLRPUpdate{
						Instances: &two,
					})).Should(Succeed())
					Eventually(IndexCounter(guid)).Should(Equal(2))

					fetchedLRP, err = bbsClient.DesiredLRPByProcessGuid(lrp.ProcessGuid)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(fetchedLRP.ModificationTag.Epoch).Should(Equal(tag.Epoch))
					Ω(fetchedLRP.ModificationTag.Index).Should(BeNumerically(">", tag.Index))
				})
			})

			Context("when the LRP does not exist", func() {
				It("errors", func() {
					two := int32(2)
					err := bbsClient.UpdateDesiredLRP("flooberdoobey", &models.DesiredLRPUpdate{
						Instances: &two,
					})
					Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceNotFound))
				})
			})
		})
	})

	Describe("Getting a DesiredLRP", func() {
		Context("when the DesiredLRP exists", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("should succeed", func() {
				lrp, err := bbsClient.DesiredLRPByProcessGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(lrp.ProcessGuid).Should(Equal(guid))
			})
		})

		Context("when the DesiredLRP does not exist", func() {
			It("should error", func() {
				lrp, err := bbsClient.DesiredLRPByProcessGuid("floobeedoo")
				Ω(lrp).Should(BeZero())
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Getting All DesiredLRPs and Getting DesiredLRPs by Domain", func() {
		var otherGuids []string

		BeforeEach(func() {
			Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherLRP := DesiredLRPWithGuid(otherGuid)
				otherLRP.Domain = otherDomain
				Ω(bbsClient.DesireLRP(otherLRP)).Should(Succeed())
				url := "http://" + RouteForGuid(otherGuid) + "/env"
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			}
		})

		It("should fetch desired lrps in the given domain", func() {
			lrpsInDomain, err := bbsClient.DesiredLRPs(models.DesiredLRPFilter{Domain: domain})
			Ω(err).ShouldNot(HaveOccurred())

			lrpsInOtherDomain, err := bbsClient.DesiredLRPs(models.DesiredLRPFilter{Domain: otherDomain})
			Ω(err).ShouldNot(HaveOccurred())

			Ω(lrpsInDomain).Should(HaveLen(1))
			Ω(lrpsInOtherDomain).Should(HaveLen(2))
			Ω([]string{lrpsInOtherDomain[0].ProcessGuid, lrpsInOtherDomain[1].ProcessGuid}).Should(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			lrpsInDomain, err := bbsClient.DesiredLRPs(models.DesiredLRPFilter{Domain: "farfignoogan"})
			Ω(err).ShouldNot(HaveOccurred())
			Ω(lrpsInDomain).Should(BeEmpty())
		})

		It("should fetch all desired lrps", func() {
			allDesiredLRPs, err := bbsClient.DesiredLRPs(models.DesiredLRPFilter{})
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
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))

				Ω(bbsClient.RemoveDesiredLRP(guid)).Should(Succeed())
				_, err := bbsClient.DesiredLRPByProcessGuid(guid)
				Ω(err).Should(HaveOccurred())
				Eventually(EndpointCurler(url)).ShouldNot(Equal(http.StatusOK))
			})
		})

		Context("when the DesiredLRP is deleted after it is claimed but before it is running #86668966", func() {
			It("should succesfully remove any ActualLRP", func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(ActualByProcessGuidGetter(lrp.ProcessGuid)).Should(ContainElement(BeActualLRPWithState(lrp.ProcessGuid, 0, models.ActualLRPStateClaimed)))
				//note: we don't wait for the ActualLRP to start running
				Ω(bbsClient.RemoveDesiredLRP(lrp.ProcessGuid)).Should(Succeed())
				Eventually(ActualByProcessGuidGetter(lrp.ProcessGuid)).Should(BeEmpty())
			})
		})

		Context("when the DesiredLRP does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := bbsClient.RemoveDesiredLRP("floobeedoobee")
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Getting all ActualLRPs", func() {
		BeforeEach(func() {
			Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
		})

		It("should fetch all Actual LRPs", func() {
			actualLRPs, err := ActualsByDomain("")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(len(actualLRPs)).Should(BeNumerically(">=", 1))
			Ω(actualLRPs).Should(ContainElement(BeActualLRP(guid, 0)))
		})
	})

	Describe("Getting ActualLRPs by Domain", func() {
		Context("when the domain is empty", func() {
			It("returns an empty list", func() {
				Ω(ActualsByDomain("floobidoo")).Should(BeEmpty())
			})
		})

		Context("when the domain contains instances", func() {
			var otherDomainLRP1 *models.DesiredLRP
			var otherDomainLRP2 *models.DesiredLRP

			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())

				otherDomainLRP1 = DesiredLRPWithGuid(NewGuid())
				otherDomainLRP1.Instances = 2
				otherDomainLRP1.Domain = otherDomain
				Ω(bbsClient.DesireLRP(otherDomainLRP1)).Should(Succeed())

				otherDomainLRP2 = DesiredLRPWithGuid(NewGuid())
				otherDomainLRP2.Domain = otherDomain
				Ω(bbsClient.DesireLRP(otherDomainLRP2)).Should(Succeed())

				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
				Eventually(IndexCounter(otherDomainLRP1.ProcessGuid)).Should(Equal(2))
				Eventually(IndexCounter(otherDomainLRP2.ProcessGuid)).Should(Equal(1))
			})

			It("returns said instances", func() {
				actualLRPs, err := ActualsByDomain(domain)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(actualLRPs).Should(HaveLen(1))
				Ω(actualLRPs).Should(ConsistOf(BeActualLRP(guid, 0)))

				actualLRPs, err = ActualsByDomain(otherDomain)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(actualLRPs).Should(HaveLen(3))

				Ω(actualLRPs).Should(ConsistOf(
					BeActualLRP(otherDomainLRP1.ProcessGuid, 0),
					BeActualLRP(otherDomainLRP1.ProcessGuid, 1),
					BeActualLRP(otherDomainLRP2.ProcessGuid, 0),
				))
			})
		})
	})

	Describe("Getting ActualLRPs by ProcessGuid", func() {
		Context("when there are none", func() {
			It("returns an empty list", func() {
				Ω(ActualsByProcessGuid("floobeedoo")).Should(BeEmpty())
			})
		})

		Context("when there are ActualLRPs for a given ProcessGuid", func() {
			BeforeEach(func() {
				lrp.Instances = 2
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("returns the ActualLRPs", func() {
				Ω(ActualsByProcessGuid(guid)).Should(ConsistOf(
					BeActualLRP(guid, 0),
					BeActualLRP(guid, 1),
				))
			})
		})
	})

	Describe("Getting the ActualLRP at a given index for a ProcessGuid", func() {
		Context("when there is no matching ProcessGuid", func() {
			It("should return a missing ActualLRP error", func() {
				actualLRP, err := ActualLRPByProcessGuidAndIndex("floobeedoo", 0)
				Ω(actualLRP).Should(BeZero())
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when there are no ActualLRPs at the given index", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(1))
			})

			It("should return a missing ActualLRP error", func() {
				actualLRP, err := ActualLRPByProcessGuidAndIndex(guid, 1)
				Ω(actualLRP).Should(BeZero())
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when there is an ActualLRP at the given index", func() {
			BeforeEach(func() {
				lrp.Instances = 2
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(IndexCounter(guid)).Should(Equal(2))
			})

			It("returns them", func() {
				Ω(ActualLRPByProcessGuidAndIndex(guid, 0)).Should(BeActualLRP(guid, 0))
				Ω(ActualLRPByProcessGuidAndIndex(guid, 1)).Should(BeActualLRP(guid, 1))
			})
		})
	})

	Describe("Restarting an ActualLRP", func() {
		Context("when there is no matching ProcessGuid", func() {
			It("returns an error", func() {
				lrpKey := models.NewActualLRPKey(guid, 0, domain)
				Ω(bbsClient.RetireActualLRP(&lrpKey)).ShouldNot(Succeed())
			})
		})

		Context("when there is no ActualLRP at the given index", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("returns an error", func() {
				lrpKey := models.NewActualLRPKey(guid, 1, domain)
				Ω(bbsClient.RetireActualLRP(&lrpKey)).ShouldNot(Succeed())
			})
		})

		Context("{SLOW} when an ActualLRP exists at the given ProcessGuid and index", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler(url)).Should(Equal(http.StatusOK))
			})

			It("restarts the actual lrp", func() {
				initialTime, err := StartedAtGetter(guid)()
				Ω(err).ShouldNot(HaveOccurred())
				lrpKey := models.NewActualLRPKey(guid, 0, domain)
				Ω(bbsClient.RetireActualLRP(&lrpKey)).Should(Succeed())
				//This needs a large timeout as the converger needs to run for it to return
				Eventually(StartedAtGetter(guid), ConvergerInterval*2).Should(BeNumerically(">", initialTime))
			})
		})
	})

	Describe("when an ActualLRP cannot be allocated", func() {
		Context("because it's too large", func() {
			BeforeEach(func() {
				lrp.MemoryMb = 1024 * 1024
			})

			It("should report this fact on the UNCLAIMED ActualLRP", func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(ActualGetter(guid, 0)).Should(BeUnclaimedActualLRPWithPlacementError(guid, 0))

				actualLRP, err := ActualLRPByProcessGuidAndIndex(guid, 0)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(actualLRP.State).Should(Equal(models.ActualLRPStateUnclaimed))
				Ω(actualLRP.PlacementError).Should(ContainSubstring("insufficient resources"))
			})
		})

		Context("because of a rootfs mismatch", func() {
			BeforeEach(func() {
				lrp.RootFs = models.PreloadedRootFS("fruitfs")
			})

			It("should allow creation of the task but should (fairly quickly) mark the task as failed", func() {
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(ActualGetter(guid, 0)).Should(BeUnclaimedActualLRPWithPlacementError(guid, 0))

				actualLRP, err := ActualLRPByProcessGuidAndIndex(guid, 0)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(actualLRP.State).Should(Equal(models.ActualLRPStateUnclaimed))
				Ω(actualLRP.PlacementError).Should(ContainSubstring("found no compatible cell"))
			})
		})
	})
})
