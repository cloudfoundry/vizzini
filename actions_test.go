package vizzini_test

import (
	"time"

	. "github.com/cloudfoundry-incubator/vizzini/matchers"

	"github.com/cloudfoundry-incubator/bbs/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Actions", func() {
	var taskDef *models.TaskDefinition

	Describe("Timeout action", func() {
		BeforeEach(func() {
			taskDef = Task()
			taskDef.Action = models.WrapAction(models.Timeout(
				&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 1000"},
					User: "vcap",
				},
				2*time.Second,
			))
			taskDef.ResultFile = ""

			Expect(bbsClient.DesireTask(guid, domain, taskDef)).To(Succeed())
		})

		It("should fail the Task within the timeout window", func() {
			Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Running))
			Eventually(TaskGetter(guid), 10).Should(HaveTaskState(models.Task_Completed))
			task, err := bbsClient.TaskByGuid(guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.GetFailed()).To(BeTrue())
			Expect(task.GetFailureReason()).To(ContainSubstring("timeout"))

			Expect(bbsClient.ResolvingTask(guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(guid)).To(Succeed())
		})
	})

	Describe("Run action", func() {
		BeforeEach(func() {
			taskDef = Task()
			taskDef.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Dir:  "/etc",
				Args: []string{"-c", "echo $PWD > /tmp/bar"},
				User: "vcap",
			})

			Expect(bbsClient.DesireTask(guid, domain, taskDef)).To(Succeed())
		})

		It("should be possible to specify a working directory", func() {
			Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
			task, err := bbsClient.TaskByGuid(guid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.GetFailed()).To(BeFalse())
			Expect(task.GetResult()).To(ContainSubstring("/etc"))

			Expect(bbsClient.ResolvingTask(guid)).To(Succeed())
			Expect(bbsClient.DeleteTask(guid)).To(Succeed())
		})
	})

	Describe("Cancelling Downloads", func() {
		It("should cancel the download", func() {
			desiredLRP := &models.DesiredLRP{
				ProcessGuid: guid,
				RootFs:      defaultRootFS,
				Domain:      domain,
				Instances:   1,
				Action: models.WrapAction(&models.DownloadAction{
					From: "https://s3-us-west-1.amazonaws.com/onsi-public/foo.zip",
					To:   "/tmp",
					User: "vcap",
				}),
			}

			Expect(bbsClient.DesireLRP(desiredLRP)).To(Succeed())
			time.Sleep(3 * time.Second)
			Expect(bbsClient.RemoveDesiredLRP(desiredLRP.ProcessGuid)).To(Succeed())
			Eventually(ActualByProcessGuidGetter(desiredLRP.ProcessGuid), 5).Should(BeEmpty())
		})
	})
})
