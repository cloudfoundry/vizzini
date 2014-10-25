package receptor_suite_test

import (
	"fmt"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/pivotal-cf-experimental/vizzini/receptor-suite/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TaskGetter(guid string) func() (receptor.TaskResponse, error) {
	return func() (receptor.TaskResponse, error) {
		return client.GetTask(guid)
	}
}

func ClearOutDomain(domain string) {
	tasks, err := client.GetAllTasksByDomain(domain)
	Ω(err).ShouldNot(HaveOccurred())
	for _, task := range tasks {
		Eventually(TaskGetter(task.TaskGuid), 5).Should(HaveTaskState(receptor.TaskStateCompleted))
		Ω(client.DeleteTask(task.TaskGuid)).Should(Succeed())
	}
	Ω(client.GetAllTasksByDomain(domain)).Should(BeEmpty())
}

var _ = Describe("Tasks", func() {
	var task receptor.CreateTaskRequest
	var guid string

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
		ClearOutDomain(domain)
	})

	Describe("Creating Tasks", func() {
		Context("When the task is well formed (the happy path)", func() {
			BeforeEach(func() {
				Ω(client.CreateTask(task)).Should(Succeed())
			})

			It("runs the task", func() {
				Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))

				task, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.TaskGuid).Should(Equal(guid))

				Ω(task.Result).Should(ContainSubstring("some output"))
			})
		})

		Context("when the task guid is malformed", func() {
			It("should fail to create", func() {
				task.TaskGuid = "abc def"
				err := client.CreateTask(task)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.InvalidTask))

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
				err := client.CreateTask(task)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.TaskGuidAlreadyExists))

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

	Describe("Getting a task", func() {
		Context("when the task exists", func() {
			BeforeEach(func() {
				Ω(client.CreateTask(task)).Should(Succeed())
			})

			It("should succeed", func() {
				Eventually(TaskGetter(guid)).ShouldNot(BeZero())
				task, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.TaskGuid).Should(Equal(guid))
			})
		})

		Context("when the task does not exist", func() {
			It("should error", func() {
				task, err := client.GetTask("floobeedoo")
				Ω(task).Should(BeZero())
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.TaskNotFound))
			})
		})
	})

	Describe("Getting All Tasks and Getting Tasks by Domain", func() {
		var otherGuids []string
		var otherDomain string
		BeforeEach(func() {
			Ω(client.CreateTask(task)).Should(Succeed())
			Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))

			otherDomain = fmt.Sprintf("New-Domain-%d", GinkgoParallelNode())
			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherTask := task
				otherTask.TaskGuid = otherGuid
				otherTask.Domain = otherDomain
				Ω(client.CreateTask(otherTask)).Should(Succeed())
				Eventually(TaskGetter(otherGuid)).Should(HaveTaskState(receptor.TaskStateCompleted))
			}
		})

		AfterEach(func() {
			ClearOutDomain(otherDomain)
		})

		It("should fetch tasks in the given domain", func() {
			defaultDomain, err := client.GetAllTasksByDomain(domain)
			Ω(err).ShouldNot(HaveOccurred())

			otherDomain, err := client.GetAllTasksByDomain(otherDomain)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(defaultDomain).Should(HaveLen(1))
			Ω(otherDomain).Should(HaveLen(2))
			Ω([]string{otherDomain[0].TaskGuid, otherDomain[1].TaskGuid}).Should(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			domain, err := client.GetAllTasksByDomain("farfignoogan")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(domain).Should(BeEmpty())
		})

		It("should fetch all tasks", func() {
			allTasks, err := client.GetAllTasks()
			Ω(err).ShouldNot(HaveOccurred())

			//if we're running in parallel there may be more than 3 things here!
			Ω(len(allTasks)).Should(BeNumerically(">=", 3))
			taskGuids := []string{}
			for _, task := range allTasks {
				taskGuids = append(taskGuids, task.TaskGuid)
			}
			Ω(taskGuids).Should(ContainElement(guid))
			Ω(taskGuids).Should(ContainElement(otherGuids[0]))
			Ω(taskGuids).Should(ContainElement(otherGuids[1]))
		})
	})

	Describe("Deleting Tasks", func() {
		Context("when the task is in the completed state", func() {
			It("should be deleted", func() {
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))

				Ω(client.DeleteTask(guid)).Should(Succeed())
				_, err := client.GetTask(guid)
				Ω(err).Should(HaveOccurred())
			})
		})

		XContext("when the task is not in the completed state", func() {
			It("should not be deleted, and should error", func() {
				task.Actions = []models.ExecutorAction{
					{
						models.RunAction{
							Path: "bash",
							Args: []string{"-c", "sleep 2; echo 'some output' > /tmp/bar"},
						},
					},
				}
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateRunning))
				err := client.DeleteTask(guid)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.TaskNotDeletable))

				_, err = client.GetAllTasksByDomain(domain)
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		XContext("when the task does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := client.DeleteTask("floobeedoobee")
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.TaskNotFound))
			})
		})
	})
})