package vizzini_test

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden/warden"

	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/pivotal-golang/lager"

	"github.com/cloudfoundry-incubator/executor/client"
	"github.com/cloudfoundry-incubator/inigo/inigo_server"
	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry-incubator/runtime-schema/models/factories"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var _ = Describe("Stress tests", func() {
	var bbs *Bbs.BBS

	BeforeEach(func() {
		bbs = Bbs.NewBBS(suiteContext.EtcdRunner.Adapter(), timeprovider.NewTimeProvider(), lager.NewLogger("vizzini"))
	})

	XDescribe("{serial} Handling many tasks", func() {
		BeforeEach(func() {
			suiteContext.ExecutorRunner.Start()
			suiteContext.RepRunner.Start()
		})

		AfterEach(func() {
			containers, err := suiteContext.WardenClient.Containers(nil)
			Ω(err).ShouldNot(HaveOccurred())
			wg := &sync.WaitGroup{}
			for _, container := range containers {
				wg.Add(1)
				go func(container warden.Container) {
					handle := container.Handle()
					vizzini.Println("Deleting ", handle)
					suiteContext.WardenClient.Destroy(handle)
					vizzini.Println("Done ", handle)
					wg.Done()
				}(container)
			}
			wg.Wait()
		})

		It("should be able to gently ramp up to handle many tasks without issue", func() {
			c := client.New(http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d", suiteContext.ExecutorPort))
			totalResources, _ := c.TotalResources()
			vizzini.Printf("Total resources: %s\n", format.Object(totalResources, 0))
			remainingResources, _ := c.RemainingResources()
			vizzini.Printf("Remaining resources: %s\n", format.Object(remainingResources, 0))
			nTasks := totalResources.Containers - 1

			t := time.Now()

			for j := 0; j < nTasks; j++ {
				guid := factories.GenerateGuid()
				task := models.Task{
					Guid:     guid,
					MemoryMB: 10,
					DiskMB:   10,
					Domain:   "vizzini",
					Actions: []models.ExecutorAction{
						{Action: models.RunAction{
							Path: "bash",
							Args: []string{"-c", fmt.Sprintf("curl %s; sleep 1000", strings.Join(inigo_server.CurlArgs(guid), " "))},
						}},
					},
					Stack: suiteContext.RepStack,
				}

				vizzini.Printf("Scheduling %d [%s]...\n", j+1, guid)
				err := bbs.DesireTask(task)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(inigo_server.ReportingGuids, DEFAULT_EVENTUALLY_TIMEOUT, 0.1).Should(ContainElement(guid))

				remainingResources, _ := c.RemainingResources()
				vizzini.Printf("...#%d is up [took %s].  Remaining resources: %s\n", j+1, time.Since(t), format.Object(remainingResources, 0))
			}
		})

		It("should be able to run many tasks simultaneously without issue", func() {
			c := client.New(http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d", suiteContext.ExecutorPort))
			nRounds := 10
			nTasks := 20
			for i := 0; i < nRounds; i++ {
				totalResources, _ := c.TotalResources()
				vizzini.Printf("Total resources: %s\n", format.Object(totalResources, 0))
				remainingResources, _ := c.RemainingResources()
				vizzini.Printf("Remaining resources: %s\n", format.Object(remainingResources, 0))

				for j := 0; j < nTasks; j++ {
					guid := factories.GenerateGuid()
					task := models.Task{
						Guid:     guid,
						MemoryMB: 10,
						DiskMB:   10,
						Domain:   "vizzini",
						Actions: []models.ExecutorAction{
							{Action: models.RunAction{
								Path: "bash",
								Args: []string{"-c", fmt.Sprintf("curl %s; sleep 2", strings.Join(inigo_server.CurlArgs(guid), " "))},
							}},
						},
						Stack: suiteContext.RepStack,
					}

					vizzini.Printf("Scheduling %d [%s]...\n", i*nTasks+j+1, task.Guid)
					err := bbs.DesireTask(task)
					Ω(err).ShouldNot(HaveOccurred())
				}

				Eventually(func() interface{} {
					resources, _ := c.RemainingResources()
					tasks, _ := bbs.GetAllCompletedTasks()
					vizzini.Printf("Have %d completed tasks.  Remaining Resources: %s\n", len(tasks), format.Object(resources, 0))
					return tasks
				}, 5*time.Minute, 1).Should(HaveLen((i + 1) * nTasks))
				Eventually(inigo_server.ReportingGuids).Should(HaveLen((i + 1) * nTasks))
			}
		})
	})
})
