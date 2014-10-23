package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	BBS "github.com/cloudfoundry-incubator/runtime-schema/bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/pivotal-golang/lager"
)

var _ = Describe("Tasks", func() {
	var bbs *BBS.BBS
	var store storeadapter.StoreAdapter

	BeforeEach(func() {
		store = etcdstoreadapter.NewETCDStoreAdapter([]string{
			"http://10.244.16.2.xip.io:4001",
		}, workpool.NewWorkPool(10))
		err := store.Connect()
		Ω(err).ShouldNot(HaveOccurred())
		bbs = BBS.NewBBS(store, timeprovider.NewTimeProvider(), lager.NewLogger("bbs"))
	})

	Describe("things that now work", func() {
		Describe("when container creation fails when creating a task (#77647588)", func() {
			It("should, eventually, be marked as failed", func() {
				tasks, stop, _ := bbs.WatchForCompletedTask()
				err := bbs.DesireTask(models.Task{
					TaskGuid: "some-task-guid",
					Domain:   "vizzini",
					Actions: []models.ExecutorAction{
						{models.RunAction{Path: "touch", Args: []string{"/tmp/foo"}}},
					},
					Stack:      "lucid64",
					CpuPercent: 1138.0,
				})
				Ω(err).ShouldNot(HaveOccurred())

				completedTask := models.Task{}
				Eventually(tasks, 10).Should(Receive(&completedTask))
				Ω(completedTask.Failed).Should(BeTrue())
				close(stop)

				err = bbs.ResolvingTask("some-task-guid")
				Ω(err).ShouldNot(HaveOccurred())

				err = bbs.ResolveTask("some-task-guid")
				Ω(err).ShouldNot(HaveOccurred())
			})
		})
	})
})
