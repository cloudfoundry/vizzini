package vizzini_test

import (
	. "github.com/cloudfoundry-incubator/vizzini/matchers"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/receptor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users", func() {
	var task receptor.TaskCreateRequest

	Context("{DOCKER} with an existing 'alice' user in the rootfs", func() {
		BeforeEach(func() {
			task = TaskWithGuid(guid)
			task.RootFS = "docker:///cloudfoundry/busybox-alice"
			task.Action = models.WrapAction(&models.RunAction{
				Path: "sh",
				Args: []string{"-c", "whoami > /tmp/output"},
				User: "alice",
			})
			task.ResultFile = "/tmp/output"

			Ω(client.CreateTask(task)).Should(Succeed())
		})

		It("runs an action as alice", func() {
			Eventually(TaskGetter(guid), 120).Should(HaveTaskState(receptor.TaskStateRunning))
			Eventually(TaskGetter(guid), 10).Should(HaveTaskState(receptor.TaskStateCompleted))

			task, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(task.Failed).Should(BeFalse())
			Ω(task.Result).Should(ContainSubstring("alice"))

			Ω(client.DeleteTask(guid)).Should(Succeed())
		})
	})
})
