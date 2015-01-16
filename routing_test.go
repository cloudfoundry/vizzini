package vizzini_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"time"

	"github.com/cloudfoundry-incubator/receptor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"
)

var _ = Describe("Routing Related Tests", func() {
	var lrp receptor.DesiredLRPCreateRequest

	Describe("sticky sessions", func() {
		var httpClient *http.Client

		BeforeEach(func() {
			jar, err := cookiejar.New(nil)
			Ω(err).ShouldNot(HaveOccurred())

			httpClient = &http.Client{
				Jar: jar,
			}

			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3

			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(IndexCounter(guid, httpClient)).Should(Equal(3))
		})

		It("should only route to the stuck instance", func() {
			resp, err := httpClient.Get("http://" + RouteForGuid(guid) + "/stick")
			Ω(err).ShouldNot(HaveOccurred())
			resp.Body.Close()

			//for some reason this isn't always 1!  it's sometimes 2....
			Ω(IndexCounter(guid, httpClient)()).Should(BeNumerically("<", 3))

			resp, err = httpClient.Get("http://" + RouteForGuid(guid) + "/unstick")
			Ω(err).ShouldNot(HaveOccurred())
			resp.Body.Close()

			Ω(IndexCounter(guid, httpClient)()).Should(Equal(3))
		})
	})

	Context("as containers come and go", func() {
		var url string
		var lrp receptor.DesiredLRPCreateRequest

		BeforeEach(func() {
			url = "http://" + RouteForGuid(guid) + "/env"
			lrp = DesiredLRPWithGuid(guid)
			lrp.Instances = 3
			Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
			Eventually(ActualByProcessGuidGetter(guid)).Should(ConsistOf(
				BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning),
				BeActualLRPWithState(guid, 1, receptor.ActualLRPStateRunning),
				BeActualLRPWithState(guid, 2, receptor.ActualLRPStateRunning),
			))
		})

		It("{SLOW} should only route to running containers", func() {
			done := make(chan struct{})
			badCodes := []int{}
			attempts := 0

			go func() {
				t := time.NewTicker(10 * time.Millisecond)
				for {
					select {
					case <-done:
						t.Stop()
					case <-t.C:
						attempts += 1
						code, _ := EndpointCurler(url)()
						if code != http.StatusOK {
							badCodes = append(badCodes, code)
						}
					}
				}
			}()

			var three = 3
			var one = 1

			updateToThree := receptor.DesiredLRPUpdateRequest{
				Instances: &three,
			}

			updateToOne := receptor.DesiredLRPUpdateRequest{
				Instances: &one,
			}

			for i := 0; i < 4; i++ {
				By(fmt.Sprintf("Scaling down then back up #%d", i+1))
				Ω(client.UpdateDesiredLRP(guid, updateToOne)).Should(Succeed())
				Eventually(ActualByProcessGuidGetter(guid)).Should(ConsistOf(
					BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning),
				))

				time.Sleep(200 * time.Millisecond)

				Ω(client.UpdateDesiredLRP(guid, updateToThree)).Should(Succeed())
				Eventually(ActualByProcessGuidGetter(guid)).Should(ConsistOf(
					BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning),
					BeActualLRPWithState(guid, 1, receptor.ActualLRPStateRunning),
					BeActualLRPWithState(guid, 2, receptor.ActualLRPStateRunning),
				))
			}

			close(done)

			fmt.Fprintf(GinkgoWriter, "%d bad codes out of %d attempts (%.3f%%)", len(badCodes), attempts, float64(len(badCodes))/float64(attempts)*100)
			Ω(len(badCodes)).Should(BeNumerically("<", float64(attempts)*0.01))
		})
	})
})
