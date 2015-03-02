package vizzini_test

import (
	"fmt"
	"net/http"
	"time"

	. "github.com/pivotal-cf-experimental/vizzini/matchers"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func MakeGraceExit(baseURL string, status int) {
	//make sure Grace is up first
	Eventually(EndpointCurler(baseURL + "/env")).Should(Equal(http.StatusOK))

	//make Grace exit
	for i := 0; i < 3; i++ {
		url := fmt.Sprintf("%s/exit/%d", baseURL, status)
		resp, err := http.Post(url, "application/octet-stream", nil)
		Ω(err).ShouldNot(HaveOccurred())
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	Fail("failed to make grace exit")
}

func TellGraceToDeleteFile(baseURL string, filename string) {
	url := fmt.Sprintf("%s/file/%s", baseURL, filename)
	req, err := http.NewRequest("DELETE", url, nil)
	Ω(err).ShouldNot(HaveOccurred())
	resp, err := http.DefaultClient.Do(req)
	Ω(err).ShouldNot(HaveOccurred())
	resp.Body.Close()
	Ω(resp.StatusCode).Should(Equal(http.StatusOK))
}

func DirectURL(guid string, index int) string {
	actualLRP, err := ActualGetter(guid, 0)()
	Ω(err).ShouldNot(HaveOccurred())
	Ω(actualLRP).ShouldNot(BeZero())
	return fmt.Sprintf("http://%s:%d", actualLRP.Address, actualLRP.Ports[0].HostPort)
}

var _ = Describe("Crashes", func() {
	var lrp receptor.DesiredLRPCreateRequest
	var url string

	BeforeEach(func() {
		url = fmt.Sprintf("http://%s", RouteForGuid(guid))
		lrp = DesiredLRPWithGuid(guid)
		lrp.Action = &models.RunAction{
			Path: "./grace",
			Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
		}
		lrp.Monitor = nil
	})

	Describe("backoff behavior", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("{SLOW} restarts the application immediately twice, and then starts backing it off, and updates the modification tag as it goes", func() {
			actualLRP, err := client.ActualLRPByProcessGuidAndIndex(guid, 0)
			Ω(err).ShouldNot(HaveOccurred())
			tag := actualLRP.ModificationTag

			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 1))

			restartedActualLRP, err := client.ActualLRPByProcessGuidAndIndex(guid, 0)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(restartedActualLRP.InstanceGuid).ShouldNot(Equal(actualLRP.InstanceGuid))
			Ω(restartedActualLRP.ModificationTag.Epoch).Should(Equal(tag.Epoch))
			Ω(restartedActualLRP.ModificationTag.Index).Should(BeNumerically(">", tag.Index))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 2))

			By("eventually restarting #3 (slow)")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateCrashed, 3))
			Consistently(ActualGetter(guid, 0), CrashRestartTimeout-5*time.Second).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateCrashed, 3))
			Eventually(ActualGetter(guid, 0), ConvergerInterval*2).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 3))
			Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
		})

		It("deletes the crashed ActualLRP when scaling down", func() {
			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 1))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 2))

			By("eventually restarting #3")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateCrashed, 3))

			By("deleting the DesiredLRP")
			Ω(client.DeleteDesiredLRP(guid)).Should(Succeed())
			Eventually(ActualByProcessGuidGetter(guid)).Should(BeEmpty())
		})
	})

	Describe("killing crashed applications", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("should delete the Crashed ActualLRP succesfully", func() {
			By("immediately restarting #1")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 1))

			By("immediately restarting #2")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 2))

			By("eventually restarting #3")
			MakeGraceExit(url, 1)
			Eventually(ActualGetter(guid, 0), ConvergerInterval).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateCrashed, 3))

			Ω(client.KillActualLRPByProcessGuidAndIndex(guid, 0)).Should(Succeed())
			Eventually(ActualByProcessGuidGetter(guid)).Should(BeEmpty())
		})
	})

	Context("with no monitor action", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("comes up as soon as the process starts", func() {
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
		})

		Context("when the process dies with exit code 0", func() {
			BeforeEach(func() {
				MakeGraceExit(url, 0)
			})

			It("gets restarted immediately", func() {
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 1))
				Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
			})
		})

		Context("when the process dies with exit code 1", func() {
			BeforeEach(func() {
				MakeGraceExit(url, 1)
			})

			It("gets restarted immediately", func() {
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 1))
				Eventually(EndpointCurler(url+"/env")).Should(Equal(http.StatusOK), "This can be removed when #89463754 lands")
			})
		})
	})

	Context("with a monitor action", func() {
		Context("when the monitor eventually succeeds", func() {
			var directURL string
			var indirectURL string
			BeforeEach(func() {
				lrp.Action = &models.RunAction{
					Path: "./grace",
					Args: []string{"-upFile=up"},
					Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
				}

				lrp.Monitor = &models.RunAction{
					Path: "cat",
					Args: []string{"/tmp/up"},
				}

				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
				Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				directURL = DirectURL(guid, 0)
				indirectURL = "http://" + RouteForGuid(guid)
			})

			It("enters the running state", func() {
				Ω(ActualGetter(guid, 0)()).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					MakeGraceExit(indirectURL, 0)
				})

				It("does not get marked as crashed (may have daemonized)", func() {
					Consistently(ActualGetter(guid, 0), 3).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateRunning, 0))
				})
			})

			Context("when the process dies with exit code 0 and the monitor subsequently fails", func() {
				BeforeEach(func() {
					//tell grace to delete the file then exit, it's highly unlikely that the health check will run
					//between these two lines so the test should actually be covering the edge case in question
					TellGraceToDeleteFile(url, "up")
					MakeGraceExit(indirectURL, 0)
				})

				It("{SLOW} is marked as crashed", func() {
					Consistently(ActualGetter(guid, 0), 2).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning), "Banking on the fact that the health check runs every thirty seconds and is unlikely to run immediately")
					Eventually(ActualGetter(guid, 0), HealthyCheckInterval+5*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					MakeGraceExit(indirectURL, 1)
				})

				It("is marked as crashed (immediately)", func() {
					Eventually(ActualGetter(guid, 0), HealthyCheckInterval/3).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			//{LOCAL} because: this test attempts to communicate with the container *directly* to ensure the process has been torn down
			//this is not possible against a remote installation as it entails connecting directly into the VPC
			Context("{LOCAL} when the monitor subsequently fails", func() {
				BeforeEach(func() {
					TellGraceToDeleteFile(indirectURL, "up")
				})

				It("{SLOW} is marked as crashed (and reaped)", func() {
					httpClient := &http.Client{
						Timeout: time.Second,
					}

					By("first validate that we can connect to the container directly")
					_, err := httpClient.Get(directURL + "/env")
					Ω(err).ShouldNot(HaveOccurred())

					By("being marked as crashed")
					Eventually(ActualGetter(guid, 0), HealthyCheckInterval+5*time.Second).Should(BeActualLRPWithCrashCount(guid, 0, 1))

					By("tearing down the process -- this reaches out to the container's direct address and ensures we can't reach it")
					_, err = httpClient.Get(directURL + "/env")
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Context("when the monitor never succeeds", func() {
			JustBeforeEach(func() {
				lrp.Monitor = &models.RunAction{
					Path: "false",
				}

				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateClaimed))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					lrp.Action = &models.RunAction{
						Path: "./grace",
						Args: []string{"-exitAfter=2s", "-exitAfterCode=0"},
						Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					}
				})

				It("does not get marked as crash, as it has presumably daemonized and we are waiting on the health check", func() {
					Consistently(ActualGetter(guid, 0), 3).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateClaimed, 0))
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					lrp.Action = &models.RunAction{
						Path: "./grace",
						Args: []string{"-exitAfter=2s", "-exitAfterCode=1"},
						Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					}
				})

				It("gets marked as crashed (immediately)", func() {
					Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("and there is a StartTimeout", func() {
				BeforeEach(func() {
					lrp.StartTimeout = 5
				})

				It("never enters the running state and is marked as crashed after the StartTimeout", func() {
					Consistently(ActualGetter(guid, 0), 3).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateClaimed))
					Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithCrashCount(guid, 0, 1))
				})
			})

			Context("and there is no start timeout", func() {
				BeforeEach(func() {
					lrp.StartTimeout = 0
				})

				It("never enters the running state, and never crashes", func() {
					Consistently(ActualGetter(guid, 0), 5).Should(BeActualLRPWithStateAndCrashCount(guid, 0, receptor.ActualLRPStateClaimed, 0))
				})
			})
		})
	})
})
