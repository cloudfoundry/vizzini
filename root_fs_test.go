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
		Expect(bbsClient.DesireTask(guid, domain, task)).To(Succeed())
		Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
	})

	AfterEach(func() {
		Expect(bbsClient.ResolvingTask(guid)).To(Succeed())
		Expect(bbsClient.DeleteTask(guid)).To(Succeed())
	})

	Describe("cflinuxfs2", func() {
		BeforeEach(func() {
			rootFS = models.PreloadedRootFS("cflinuxfs2")
		})

		It("should run the cflinuxfs2 rootfs", func() {
			completedTask, err := bbsClient.TaskByGuid(guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(completedTask.Result).To(ContainSubstring(`bash, version 4.3.11`))
		})
	})
})
