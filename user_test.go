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

			Expect(bbsClient.DesireTask(guid, domain, task)).To(Succeed())
		})

		It("runs an action as alice", func() {
			Eventually(TaskGetter(guid), 120).Should(HaveTaskState(models.Task_Running))
			Eventually(TaskGetter(guid), 120).Should(HaveTaskState(models.Task_Completed))

			task, err := bbsClient.TaskByGuid(guid)
			Expect(err).NotTo(HaveOccurred())

			Expect(task.Failed).To(BeFalse())
			Expect(task.Result).To(ContainSubstring("alice"))

			Expect(bbsClient.ResolvingTask(guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(guid)).To(Succeed())
		})
	})
})
