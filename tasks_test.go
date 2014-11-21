package vizzini_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"

	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TaskGetter(guid string) func() (receptor.TaskResponse, error) {
	return func() (receptor.TaskResponse, error) {
		return client.GetTask(guid)
	}
}

func ClearOutTasksInDomain(domain string) {
	tasks, err := client.TasksByDomain(domain)
	Ω(err).ShouldNot(HaveOccurred())
	for _, task := range tasks {
		Eventually(TaskGetter(task.TaskGuid), 5).Should(HaveTaskState(receptor.TaskStateCompleted))
		Ω(client.DeleteTask(task.TaskGuid)).Should(Succeed())
	}
	Ω(client.TasksByDomain(domain)).Should(BeEmpty())
}

var _ = Describe("Tasks", func() {
	var task receptor.TaskCreateRequest
	var guid string

	BeforeEach(func() {
		guid = NewGuid()
		task = receptor.TaskCreateRequest{
			TaskGuid:   guid,
			Domain:     domain,
			RootFSPath: rootFS,
			Action: &models.RunAction{
				Path: "bash",
				Args: []string{"-c", "echo 'some output' > /tmp/bar"},
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
	})

	AfterEach(func() {
		ClearOutTasksInDomain(domain)
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
				taskCopy.Action = nil
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())

				By("not having a stack")
				taskCopy = task
				taskCopy.Stack = ""
				Ω(client.CreateTask(taskCopy)).ShouldNot(Succeed())
			})
		})

		Context("when the CPUWeight is out of bounds", func() {
			It("should fail", func() {
				task.CPUWeight = 101
				err := client.CreateTask(task)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.InvalidTask))
			})
		})

		Context("when the annotation is too large", func() {
			It("should fail", func() {
				task.Annotation = strings.Repeat("7", 1024*10+1)
				err := client.CreateTask(task)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.InvalidTask))
			})
		})
	})

	Describe("Specifying environment variables", func() {
		BeforeEach(func() {
			task.EnvironmentVariables = []receptor.EnvironmentVariable{
				{"CONTAINER_LEVEL", "A"},
				{"OVERRIDE", "B"},
			}
			task.Action = &models.RunAction{
				Path: "bash",
				Args: []string{"-c", "env > /tmp/bar"},
				Env: []models.EnvironmentVariable{
					{"ACTION_LEVEL", "C"},
					{"OVERRIDE", "D"},
				},
			}
		})

		It("should be possible to specify environment variables on both the Task and the RunAction", func() {
			Ω(client.CreateTask(task)).Should(Succeed())
			Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))

			task, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(task.Result).Should(ContainSubstring("CONTAINER_LEVEL=A"))
			Ω(task.Result).Should(ContainSubstring("ACTION_LEVEL=C"))
			Ω(task.Result).Should(ContainSubstring("OVERRIDE=D"))
			Ω(task.Result).ShouldNot(ContainSubstring("OVERRIDE=B"))
		})
	})

	Describe("{DOCKER} Creating a Docker-based Task", func() {
		BeforeEach(func() {
			task.RootFSPath = "docker:///onsi/grace-busybox"
			task.Action = &models.RunAction{
				Path: "sh",
				Args: []string{"-c", "ls / > /tmp/bar"},
			}
			Ω(client.CreateTask(task)).Should(Succeed())
		})

		It("should succeed", func() {
			Eventually(TaskGetter(guid), 120).Should(HaveTaskState(receptor.TaskStateCompleted), "Docker can be quite slow to spin up....")

			task, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Result).Should(ContainSubstring("grace"))
		})
	})

	Describe("Cancelling tasks", func() {
		Context("when the task exists", func() {
			BeforeEach(func() {
				task.Action = &models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 1000"},
				}
				Ω(client.CreateTask(task)).Should(Succeed())
			})

			It("should cancel the task marking it completed immediately", func() {
				Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateRunning))

				Ω(client.CancelTask(guid)).Should(Succeed())
				task, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.State).Should(Equal(receptor.TaskStateCompleted))
				Ω(task.Failed).Should(BeTrue())
				Ω(task.FailureReason).Should(Equal("task was cancelled"))

				Ω(client.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should fail", func() {
				Ω(client.CancelTask("floobeedoo")).ShouldNot(Succeed())
			})
		})

		Context("when the task is already completed", func() {
			BeforeEach(func() {
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))
			})

			It("should fail", func() {
				Ω(client.CancelTask(guid)).ShouldNot(Succeed())
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
			ClearOutTasksInDomain(otherDomain)
		})

		It("should fetch tasks in the given domain", func() {
			defaultDomain, err := client.TasksByDomain(domain)
			Ω(err).ShouldNot(HaveOccurred())

			otherDomain, err := client.TasksByDomain(otherDomain)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(defaultDomain).Should(HaveLen(1))
			Ω(otherDomain).Should(HaveLen(2))
			Ω([]string{otherDomain[0].TaskGuid, otherDomain[1].TaskGuid}).Should(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			domain, err := client.TasksByDomain("farfignoogan")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(domain).Should(BeEmpty())
		})

		It("should fetch all tasks", func() {
			allTasks, err := client.Tasks()
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

		Context("when the task is not in the completed state", func() {
			It("should not be deleted, and should error", func() {
				task.Action = &models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 2; echo 'some output' > /tmp/bar"},
				}
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateRunning))
				err := client.DeleteTask(guid)
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.TaskNotDeletable))

				_, err = client.TasksByDomain(domain)
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the task does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := client.DeleteTask("floobeedoobee")
				Ω(err.(receptor.Error).Type).Should(Equal(receptor.TaskNotFound))
			})
		})
	})

	Describe("Registering Completion Callbacks", func() {
		var testRunnerURL string
		var server *ghttp.Server
		var port string
		var status int
		var done chan struct{}

		BeforeEach(func() {
			testRunnerURL = "192.168.220.1"

			server = ghttp.NewUnstartedServer()
			l, err := net.Listen("tcp", "0.0.0.0:0")
			Ω(err).ShouldNot(HaveOccurred())
			server.HTTPTestServer.Listener = l
			server.HTTPTestServer.Start()

			re := regexp.MustCompile(`:(\d+)$`)
			port = re.FindStringSubmatch(server.URL())[1]
			Ω(port).ShouldNot(BeZero())

			done = make(chan struct{})
			server.AppendHandlers(ghttp.CombineHandlers(
				ghttp.VerifyRequest("POST", "/endpoint"),
				ghttp.RespondWithPtr(&status, nil),
				func(w http.ResponseWriter, req *http.Request) {
					var receivedTask receptor.TaskResponse
					json.NewDecoder(req.Body).Decode(&receivedTask)
					Ω(receivedTask.TaskGuid).Should(Equal(guid))
					close(done)
				},
			))

			task.CompletionCallbackURL = "http://" + testRunnerURL + ":" + port + "/endpoint"
		})

		AfterEach(func() {
			server.Close()
		})

		Context("when the server responds succesfully", func() {
			BeforeEach(func() {
				status = http.StatusOK
			})

			It("cleans up the task", func() {
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(func() bool {
					_, err := client.GetTask(guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("when the server responds in the 4XX range", func() {
			BeforeEach(func() {
				status = http.StatusNotFound
			})

			It("nonetheless, cleans up the task", func() {
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(func() bool {
					_, err := client.GetTask(guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("when the server responds with 503 repeatedly", func() {
			var secondDone chan struct{}

			BeforeEach(func() {
				status = http.StatusServiceUnavailable

				secondDone = make(chan struct{})
				server.AppendHandlers(ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/endpoint"),
					ghttp.RespondWith(http.StatusOK, nil),
					func(w http.ResponseWriter, req *http.Request) {
						var receivedTask receptor.TaskResponse
						json.NewDecoder(req.Body).Decode(&receivedTask)
						Ω(receivedTask.TaskGuid).Should(Equal(guid))
						close(secondDone)
					},
				))
			})

			It("should retry", func() {
				Ω(client.CreateTask(task)).Should(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(secondDone).Should(BeClosed())
				Eventually(func() bool {
					_, err := client.GetTask(guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})
	})
})
