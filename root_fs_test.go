package vizzini_test

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	. "github.com/cloudfoundry-incubator/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Targetting different RootFSes", func() {
	var task *models.TaskDefinition
	var rootFS string

	JustBeforeEach(func() {
		task = Task()
		task.RootFs = rootFS
		task.Action = models.WrapAction(&models.RunAction{
			Path: "bash",
			Args: []string{"-c", "bash --version > /tmp/bar"},
			User: "vcap",
		})
		Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
		Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
	})

	AfterEach(func() {
		Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
		Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
	})

	Describe("cflinuxfs2", func() {
		BeforeEach(func() {
			rootFS = models.PreloadedRootFS("cflinuxfs2")
		})

		It("should run the cflinuxfs2 rootfs", func() {
			completedTask, err := bbsClient.TaskByGuid(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(completedTask.Result).Should(ContainSubstring(`bash, version 4.3.11`))
		})
	})
})
