package vizzini_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Targetting different RootFSes", func() {
	var task receptor.TaskCreateRequest
	var rootFS string

	JustBeforeEach(func() {
		task = TaskWithGuid(guid)
		task.RootFS = rootFS
		task.Action = &models.RunAction{
			Path: "bash",
			Args: []string{"-c", "bash --version > /tmp/bar"},
		}
		Ω(client.CreateTask(task)).Should(Succeed())
		Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))
	})

	AfterEach(func() {
		Ω(client.DeleteTask(guid)).Should(Succeed())
	})

	Describe("lucid64", func() {
		BeforeEach(func() {
			rootFS = models.PreloadedRootFS("lucid64")
		})

		It("should run the lucid64 rootfs", func() {
			completedTask, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(completedTask.Result).Should(ContainSubstring(`bash, version 4.1.5`))
		})
	})

	Describe("cflinuxfs2", func() {
		BeforeEach(func() {
			rootFS = models.PreloadedRootFS("cflinuxfs2")
		})

		It("should run the cflinuxfs2 rootfs", func() {
			completedTask, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(completedTask.Result).Should(ContainSubstring(`bash, version 4.3.11`))
		})
	})
})
