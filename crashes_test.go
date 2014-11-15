package vizzini_test

import (
	"bytes"
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
	url := fmt.Sprintf("%s/exit/%d", baseURL, status)
	resp, err := http.Post(url, "application/octet-stream", nil)
	Ω(err).ShouldNot(HaveOccurred())
	resp.Body.Close()
	Ω(resp.StatusCode).Should(Equal(http.StatusOK))
}

func TellGraceToMakeFile(baseURL string, filename string, content string) {
	url := fmt.Sprintf("%s/file/%s", baseURL, filename)
	resp, err := http.Post(url, "application/octet-stream", bytes.NewBufferString(content))
	Ω(err).ShouldNot(HaveOccurred())
	resp.Body.Close()
	Ω(resp.StatusCode).Should(Equal(http.StatusOK))
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
	return fmt.Sprintf("http://%s:%d", actualLRP.Host, actualLRP.Ports[0].HostPort)
}

const HealthyCheckInterval = 30 * time.Second

var _ = Describe("{CRASHES} Crashes", func() {
	var lrp receptor.DesiredLRPCreateRequest
	var guid, url string

	BeforeEach(func() {
		guid = NewGuid()
		url = fmt.Sprintf("http://%s", RouteForGuid(guid))
		lrp = DesiredLRPWithGuid(guid)
		lrp.Action = models.ExecutorAction{
			models.RunAction{
				Path: "./grace",
				Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
			},
		}
		lrp.Monitor = nil
	})

	AfterEach(func() {
		ClearOutDesiredLRPsInDomain(domain)
	})

	Context("with no monitor action", func() {
		BeforeEach(func() {
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
			Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
		})

		It("comes up as soon as the process starts", func() {
			Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
		})

		Context("when the process dies with exit code 0", func() {
			BeforeEach(func() {
				MakeGraceExit(url, 0)
			})

			It("gets removed from the BBS", func() {
				Eventually(ActualGetter(guid, 0)).Should(BeZero())
			})
		})

		Context("when the process dies with exit code 1", func() {
			BeforeEach(func() {
				MakeGraceExit(url, 1)
			})

			It("gets removed from the BBS", func() {
				Eventually(ActualGetter(guid, 0)).Should(BeZero())
			})
		})
	})

	Context("with a monitor action", func() {
		Context("when the monitor never succeeds", func() {
			JustBeforeEach(func() {
				lrp.Monitor = &models.ExecutorAction{
					models.RunAction{
						Path: "false",
					},
				}

				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateStarting))
			})

			It("never enters the running state", func() {
				Consistently(ActualGetter(guid, 0), 2).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateStarting))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					lrp.Action = models.ExecutorAction{
						models.RunAction{
							Path: "./grace",
							Args: []string{"-exitAfter=2s", "-exitAfterCode=0"},
							Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
						},
					}
				})

				It("does not get removed from the BBS, as it has presumably daemonized and we are waiting on the health check", func() {
					Consistently(ActualGetter(guid, 0), 5).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateStarting))
				})

				PIt("it gets removed after a timeout", func() {

				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					lrp.Action = models.ExecutorAction{
						models.RunAction{
							Path: "./grace",
							Args: []string{"-exitAfter=2s", "-exitAfterCode=1"},
							Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
						},
					}
				})

				It("gets removed from the BBS", func() {
					Eventually(ActualGetter(guid, 0)).Should(BeZero())
				})
			})
		})

		Context("when the monitor eventually succeeds", func() {
			BeforeEach(func() {
				lrp.Action = models.ExecutorAction{
					models.RunAction{
						Path: "./grace",
						Args: []string{"-upFile=up"},
						Env:  []models.EnvironmentVariable{{Name: "PORT", Value: "8080"}},
					},
				}

				lrp.Monitor = &models.ExecutorAction{
					models.RunAction{
						Path: "cat",
						Args: []string{"/tmp/up"},
					},
				}

				Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
				Eventually(EndpointCurler(url + "/env")).Should(Equal(http.StatusOK))
				url = DirectURL(guid, 0)
			})

			It("enters the running state", func() {
				Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
			})

			Context("when the process dies with exit code 0", func() {
				BeforeEach(func() {
					MakeGraceExit(url, 0)
				})

				It("does not get removed from the BBS", func() {
					Consistently(ActualGetter(guid, 0), 2).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
				})
			})

			Context("when the process dies with exit code 0 and the monitor subsequently fails", func() {
				BeforeEach(func() {
					//tell grace to delete the file then exit, it's highly unlikely that the health check will run
					//between these two lines so the test should actually be covering the edge case in question
					TellGraceToDeleteFile(url, "up")
					MakeGraceExit(url, 0)
				})

				It("{SLOW} gets removed from the BBS", func() {
					Consistently(ActualGetter(guid, 0), 2).ShouldNot(BeZero(), "Banking on the fact that the health check runs every thirty seconds and is unlikely to run immediately")
					Eventually(ActualGetter(guid, 0), HealthyCheckInterval+5*time.Second).Should(BeZero())
				})
			})

			Context("when the process dies with exit code 1", func() {
				BeforeEach(func() {
					MakeGraceExit(url, 1)
				})

				It("gets removed from the BBS", func() {
					Eventually(ActualGetter(guid, 0)).Should(BeZero())
				})
			})

			Context("when the monitor subsequently fails", func() {
				BeforeEach(func() {
					TellGraceToDeleteFile(url, "up")
				})

				It("{SLOW} gets removed from the BBS and cleans up the process", func() {
					Eventually(ActualGetter(guid, 0), HealthyCheckInterval+5*time.Second).Should(BeZero())
					httpClient := &http.Client{
						Timeout: time.Second,
					}
					time.Sleep(time.Second) //give it a second to actually die...
					_, err := httpClient.Get(url + "/env")
					Ω(err).Should(HaveOccurred())
				})
			})
		})
	})
})
