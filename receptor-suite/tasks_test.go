package receptor_suite_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks", func() {
	var task receptor.CreateTaskRequest
	var guid string

	Describe("Creating Tasks", func() {
		BeforeEach(func() {
			guid = NewGuid()
			task = receptor.CreateTaskRequest{
				TaskGuid: guid,
				Domain:   domain,
				Actions: []models.ExecutorAction{
					{
						models.RunAction{
							Path: "bash",
							Args: []string{"-c", "echo 'some output' > /tmp/bar"},
						},
					},
				},
				Stack:      stack,
				MemoryMB:   128,
				DiskMB:     128,
				CpuPercent: 100,
				Log: models.LogConfig{
					Guid:       guid,
					SourceName: "VIZ",
				},
				ResultFile: "/tmp/bar",
				Annotation: "arbitrary-data",
			}
		})

		AfterEach(func() {
			fmt.Println("NEED TO CLEAN UP!  FOR NOW:\ncurl http://10.244.16.2.xip.io:4001/v2/keys/v1/task/?recursive=true -XDELETE")
		})

		Context("When the task is well formed (the happy path)", func() {
			BeforeEach(func() {
				Ω(client.CreateTask(task)).Should(Succeed())
			})

			It("runs the task", func() {
				Eventually(func() (receptor.TaskResponse, error) {
					return client.GetTask(guid)
				}).ShouldNot(BeZero())

				task, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.TaskGuid).Should(Equal(guid))
				//Ω(task.Result).Should(Equal("some output"))
			})
		})

		XContext("when the task guid is malformed", func() {
			It("should fail to create", func() {
				task.TaskGuid = "abc def"
				Ω(client.CreateTask(task)).ShouldNot(Succeed())

				task.TaskGuid = "abc/def"
				Ω(client.CreateTask(task)).ShouldNot(Succeed())

				task.TaskGuid = "abc,def"
				Ω(client.CreateTask(task)).ShouldNot(Succeed())

				task.TaskGuid = "abc.def"
				Ω(client.CreateTask(task)).ShouldNot(Succeed())

				task.TaskGuid = "abc∆def"
				Ω(client.CreateTask(task)).ShouldNot(Succeed())
			})
		})

		Context("when the task guid is not unique", func() {
			It("should fail to create", func() {
				Ω(client.CreateTask(task)).Should(Succeed())
				Ω(client.CreateTask(task)).ShouldNot(Succeed())

				By("even when the domain is different")
				task.Domain = "some-other-domain"
				Ω(client.CreateTask(task)).ShouldNot(Succeed())
			})
		})

		Context("when required fields are missing", func() {
			It("should fail", func() {
				By("not having TaskGuid")
				taskCopy := task
				taskCopy.TaskGuid = ""
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())

				By("not having a domain")
				taskCopy = task
				taskCopy.Domain = ""
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())

				By("not having any actions")
				taskCopy = task
				taskCopy.Actions = nil
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())
				taskCopy.Actions = []models.ExecutorAction{}
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())

				By("not having a stack")
				taskCopy = task
				taskCopy.Stack = ""
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())
			})
		})
	})
})
