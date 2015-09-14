package vizzini_test

import (
	"time"

	. "github.com/cloudfoundry-incubator/vizzini/matchers"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/receptor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Actions", func() {
	var task receptor.TaskCreateRequest

	Describe("Timeout action", func() {
		BeforeEach(func() {
			task = TaskWithGuid(guid)
			task.Action = models.WrapAction(models.Timeout(
				&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 1000"},
					User: "vcap",
				},
				2*time.Second,
			))
			task.ResultFile = ""

			Ω(client.CreateTask(task)).Should(Succeed())
		})

		It("should fail the Task within the timeout window", func() {
			Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateRunning))
			Eventually(TaskGetter(guid), 10).Should(HaveTaskState(receptor.TaskStateCompleted))
			task, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Failed).Should(BeTrue())
			Ω(task.FailureReason).Should(ContainSubstring("timeout"))

			Ω(client.DeleteTask(guid)).Should(Succeed())
		})
	})

	Describe("Run action", func() {
		BeforeEach(func() {
			task = TaskWithGuid(guid)
			task.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Dir:  "/etc",
				Args: []string{"-c", "echo $PWD > /tmp/bar"},
				User: "vcap",
			})

			Ω(client.CreateTask(task)).Should(Succeed())
		})

		It("should be possible to specify a working directory", func() {
			Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))
			task, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Failed).Should(BeFalse())
			Ω(task.Result).Should(ContainSubstring("/etc"))

			Ω(client.DeleteTask(guid)).Should(Succeed())
		})
	})

	Describe("Cancelling Downloads", func() {
		var desiredLRP receptor.DesiredLRPCreateRequest
		BeforeEach(func() {
			desiredLRP = receptor.DesiredLRPCreateRequest{
				ProcessGuid: guid,
				RootFS:      defaultRootFS,
				Domain:      domain,
				Instances:   1,
				Action: models.WrapAction(&models.DownloadAction{
					From: "https://s3-us-west-1.amazonaws.com/onsi-public/foo.zip",
					To:   "/tmp",
					User: "vcap",
				}),
			}
		})

		It("should cancel the download", func() {
			Ω(client.CreateDesiredLRP(desiredLRP)).Should(Succeed())
			time.Sleep(3 * time.Second)
			Ω(client.DeleteDesiredLRP(desiredLRP.ProcessGuid)).Should(Succeed())
			Eventually(ActualGetter(desiredLRP.ProcessGuid, 0), 5).Should(BeZero())
		})
	})
})
