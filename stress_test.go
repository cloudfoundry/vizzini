package vizzini_test

import (
	"fmt"
	"net/http"
	"strings"

	steno "github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/gunk/timeprovider"

	"github.com/cloudfoundry-incubator/executor/client"
	"github.com/cloudfoundry-incubator/inigo/inigo_server"
	Bbs "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models/factories"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var _ = Describe("Stress tests", func() {
	var bbs *Bbs.BBS

	BeforeEach(func() {
		logSink := steno.NewTestingSink()

		steno.Init(&steno.Config{
			Sinks: []steno.Sink{logSink},
		})

		logger := steno.NewLogger("the-logger")
		steno.EnterTestMode()

		bbs = Bbs.NewBBS(suiteContext.EtcdRunner.Adapter(), timeprovider.NewTimeProvider(), logger)
	})

	Describe("{serial} Handling many tasks", func() {
		BeforeEach(func() {
			suiteContext.ExecutorRunner.Start()
			suiteContext.RepRunner.Start()
		})

		It("should be able to run many tasks without issue", func() {
			c := client.New(http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d", suiteContext.ExecutorPort))
			nRounds := 10
			nTasks := 100
			for i := 0; i < nRounds; i++ {
				totalResources, _ := c.TotalResources()
				vizzini.Printf("Total resources: %s\n", format.Object(totalResources, 0))
				remainingResources, _ := c.RemainingResources()
				vizzini.Printf("Remaining resources: %s\n", format.Object(remainingResources, 0))

				for j := 0; j < nTasks; j++ {
					task := factories.BuildTaskWithRunAction(
						suiteContext.RepStack,
						10,
						10,
						"bash",
						[]string{"-c", fmt.Sprintf("curl %s; sleep 2", strings.Join(inigo_server.CurlArgs(factories.GenerateGuid()), " "))},
					)
					vizzini.Printf("Scheduling %d [%s]...\n", i*nTasks+j+1, task.Guid)
					err := bbs.DesireTask(task)
					Ω(err).ShouldNot(HaveOccurred())
				}

				Eventually(func() interface{} {
					resources, _ := c.RemainingResources()
					tasks, _ := bbs.GetAllCompletedTasks()
					vizzini.Printf("\nHave %d completed tasks\n", len(tasks))
					vizzini.Printf("Remaining resources: %s\n", format.Object(resources, 0))
					return tasks
				}, 120, 1).Should(HaveLen(nTasks))
				Eventually(inigo_server.ReportingGuids, 120).Should(HaveLen((i + 1) * nTasks))
			}
		})
	})
})
