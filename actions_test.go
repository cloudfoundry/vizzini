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
				TaskGuid:   guid,
				Domain:     domain,
				RootFSPath: rootFS,
				Action: &models.TimeoutAction{
					Action: &models.RunAction{
						Path: "bash",
						Args: []string{"-c", "sleep 1000"},
					},
					Timeout: 2 * time.Second,
				},
				Stack:      stack,
				MemoryMB:   128,
				DiskMB:     128,
				CPUWeight:  100,
				LogGuid:    guid,
				LogSource:  "VIZ",
				ResultFile: "/tmp/bar",
				Annotation: "arbitrary-data",
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
})
