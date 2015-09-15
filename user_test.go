package vizzini_test

import (
	. "github.com/cloudfoundry-incubator/vizzini/matchers"

	"github.com/cloudfoundry-incubator/bbs/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users", func() {
	var task *models.TaskDefinition

	Context("{DOCKER} with an existing 'alice' user in the rootfs", func() {
		BeforeEach(func() {
			task = Task()
			task.RootFs = "docker:///cloudfoundry/busybox-alice"
			task.Action = models.WrapAction(&models.RunAction{
				Path: "sh",
				Args: []string{"-c", "whoami > /tmp/output"},
				User: "alice",
			})
			task.ResultFile = "/tmp/output"

			Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
		})

		It("runs an action as alice", func() {
			Eventually(TaskGetter(guid), 120).Should(HaveTaskState(models.Task_Running))
			Eventually(TaskGetter(guid), 10).Should(HaveTaskState(models.Task_Completed))

			task, err := bbsClient.TaskByGuid(guid)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(task.Failed).Should(BeFalse())
			Ω(task.Result).Should(ContainSubstring("alice"))

			Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
			Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
		})
	})
})
