package vizzini_test

import (
	"time"

	. "github.com/pivotal-cf-experimental/vizzini/matchers"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Actions", func() {
	var task receptor.TaskCreateRequest
	var guid string

	Describe("Timeout action", func() {
		BeforeEach(func() {
			guid = NewGuid()
			task = receptor.TaskCreateRequest{
				TaskGuid: guid,
				Domain:   domain,
				Action: &models.TimeoutAction{
					Action: &models.RunAction{
						Path: "bash",
						Args: []string{"-c", "sleep 1000"},
					},
					Timeout: 2 * time.Second,
				},
				Stack: stack,
			}

			Ω(client.CreateTask(task)).Should(Succeed())
		})

		AfterEach(func() {
			ClearOutTasksInDomain(domain)
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
			guid = NewGuid()
			task = receptor.TaskCreateRequest{
				TaskGuid: guid,
				Domain:   domain,
				Action: &models.RunAction{
					Path: "bash",
					Dir:  "/etc",
					Args: []string{"-c", "echo $PWD > /tmp/bar"},
				},
				Stack:      stack,
				ResultFile: "/tmp/bar",
			}

			Ω(client.CreateTask(task)).Should(Succeed())

		})

		It("should be possible to specify a working directory", func() {
			Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))
			task, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Failed).Should(BeFalse())
			Ω(task.Result).Should(ContainSubstring("/etc"))
		})
	})
})