package vizzini_test

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-incubator/bbs/models"
	. "github.com/cloudfoundry-incubator/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tasks", func() {
	var task *models.TaskDefinition

	BeforeEach(func() {
		task = Task()
	})

	Describe("Creating Tasks", func() {
		Context("When the task is well formed (the happy path)", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
			})

			It("runs the task", func() {
				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))

				task, err := bbsClient.TaskByGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.TaskGuid).Should(Equal(guid))

				Ω(task.Failed).Should(BeFalse())
				Ω(task.Result).Should(ContainSubstring("some output"))

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("when the task guid is malformed", func() {
			It("should fail to create", func() {
				var badGuid string

				badGuid = "abc def"
				err := bbsClient.DesireTask(badGuid, domain, task)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidRequest))

				badGuid = "abc/def"
				Ω(bbsClient.DesireTask(badGuid, domain, task)).ShouldNot(Succeed())

				badGuid = "abc,def"
				Ω(bbsClient.DesireTask(badGuid, domain, task)).ShouldNot(Succeed())

				badGuid = "abc.def"
				Ω(bbsClient.DesireTask(badGuid, domain, task)).ShouldNot(Succeed())

				badGuid = "abc∆def"
				Ω(bbsClient.DesireTask(badGuid, domain, task)).ShouldNot(Succeed())
			})
		})

		Context("when the task guid is not unique", func() {
			It("should fail to create", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				err := bbsClient.DesireTask(guid, domain, task)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceExists))

				By("even when the domain is different")
				Ω(bbsClient.DesireTask(guid, otherDomain, task)).ShouldNot(Succeed())

				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("when required fields are missing", func() {
			It("should fail", func() {
				By("not having TaskGuid")
				Ω(bbsClient.DesireTask("", domain, task)).ShouldNot(Succeed())

				By("not having a domain")
				Ω(bbsClient.DesireTask(guid, "", task)).ShouldNot(Succeed())

				By("not having any actions")
				invalidTask := Task()
				invalidTask.Action = nil
				Ω(bbsClient.DesireTask(guid, domain, invalidTask)).ShouldNot(Succeed())

				By("not having a rootfs")
				invalidTask = Task()
				invalidTask.RootFs = ""
				Ω(bbsClient.DesireTask(guid, domain, invalidTask)).ShouldNot(Succeed())

				By("having a malformed rootfs")
				invalidTask = Task()
				invalidTask.RootFs = "ploop"
				Ω(bbsClient.DesireTask(guid, domain, invalidTask)).ShouldNot(Succeed())
			})
		})

		Context("when the CPUWeight is out of bounds", func() {
			It("should fail", func() {
				task.CpuWeight = 101
				err := bbsClient.DesireTask(guid, domain, task)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidRequest))
			})
		})

		Context("when the annotation is too large", func() {
			It("should fail", func() {
				task.Annotation = strings.Repeat("7", 1024*10+1)
				err := bbsClient.DesireTask(guid, domain, task)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidRequest))
			})
		})

		Context("Upon failure", func() {
			BeforeEach(func() {
				task.Action = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "echo 'some output' > /tmp/bar && exit 1"},
					User: "vcap",
				})
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
			})

			It("should be marked as failed and should not return the result file", func() {
				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))

				task, err := bbsClient.TaskByGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.TaskGuid).Should(Equal(guid))
				Ω(task.Failed).Should(BeTrue())

				Ω(task.Result).Should(BeEmpty())

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})
	})

	Describe("Specifying environment variables", func() {
		BeforeEach(func() {
			task.EnvironmentVariables = []*models.EnvironmentVariable{
				{"CONTAINER_LEVEL", "A"},
				{"OVERRIDE", "B"},
			}
			task.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Args: []string{"-c", "env > /tmp/bar"},
				User: "vcap",
				Env: []*models.EnvironmentVariable{
					{"ACTION_LEVEL", "C"},
					{"OVERRIDE", "D"},
				},
			})
		})

		It("should be possible to specify environment variables on both the Task and the RunAction", func() {
			Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
			Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))

			task, err := bbsClient.TaskByGuid(guid)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(task.Result).Should(ContainSubstring("CONTAINER_LEVEL=A"))
			Ω(task.Result).Should(ContainSubstring("ACTION_LEVEL=C"))
			Ω(task.Result).Should(ContainSubstring("OVERRIDE=D"))
			Ω(task.Result).ShouldNot(ContainSubstring("OVERRIDE=B"))

			Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
			Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
		})
	})

	Describe("{DOCKER} Creating a Docker-based Task", func() {
		BeforeEach(func() {
			task.RootFs = "docker:///cloudfoundry/busybox-alice"
			task.Action = models.WrapAction(&models.RunAction{
				Path: "sh",
				Args: []string{"-c", "echo 'down-the-rabbit-hole' > payload && chmod 0400 payload"},
				User: "alice",
			})
			task.ResultFile = "/home/alice/payload"

			Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
		})

		It("should succeed", func() {
			Eventually(TaskGetter(guid), 120).Should(HaveTaskState(models.Task_Completed), "Docker can be quite slow to spin up....")

			task, err := bbsClient.TaskByGuid(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Failed).Should(BeFalse())
			Ω(task.Result).Should(ContainSubstring("down-the-rabbit-hole"))

			Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
			Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
		})
	})

	Describe("Cancelling tasks", func() {
		Context("when the task exists", func() {
			var lrpGuid string

			BeforeEach(func() {
				lrpGuid = NewGuid()

				lrp := DesiredLRPWithGuid(lrpGuid)
				Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
				Eventually(EndpointCurler("http://" + RouteForGuid(lrpGuid) + "/env")).Should(Equal(http.StatusOK))

				incrementCounterRoute := "http://" + RouteForGuid(lrpGuid) + "/counter"

				task.EgressRules = []*models.SecurityGroupRule{
					{
						Protocol:     models.AllProtocol,
						Destinations: []string{"0.0.0.0/0"},
					},
				}
				task.Action = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", fmt.Sprintf("while true; do curl %s -X POST; sleep 0.05; done", incrementCounterRoute)},
					User: "vcap",
				})

				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
			})

			It("should cancel the task immediately", func() {
				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Running))

				By("verifying the counter is being incremented")
				Eventually(GraceCounterGetter(lrpGuid)).Should(BeNumerically(">", 2))

				Ω(bbsClient.CancelTask(guid)).Should(Succeed())

				By("marking the task as completed")
				task, err := bbsClient.TaskByGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.State).Should(Equal(models.Task_Completed))
				Ω(task.Failed).Should(BeTrue())
				Ω(task.FailureReason).Should(Equal("task was cancelled"))

				By("actually shutting down the container immediately, it should stop incrementing the counter")
				counterAfterCancel, err := GraceCounterGetter(lrpGuid)()
				Ω(err).ShouldNot(HaveOccurred())

				time.Sleep(2 * time.Second)

				counterAfterSomeTime, err := GraceCounterGetter(lrpGuid)()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(counterAfterSomeTime).Should(BeNumerically("<", counterAfterCancel+20))

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should fail", func() {
				Ω(bbsClient.CancelTask("floobeedoo")).ShouldNot(Succeed())
			})
		})

		Context("when the task is already completed", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
			})

			It("should fail", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).ShouldNot(Succeed())

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})
	})

	Describe("Getting a task", func() {
		Context("when the task exists", func() {
			BeforeEach(func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
			})

			It("should succeed", func() {
				Eventually(TaskGetter(guid)).ShouldNot(BeZero())
				task, err := bbsClient.TaskByGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(task.TaskGuid).Should(Equal(guid))

				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should error", func() {
				task, err := bbsClient.TaskByGuid("floobeedoo")
				Ω(task).Should(BeZero())
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("Getting All Tasks and Getting Tasks by Domain", func() {
		var otherGuids []string

		BeforeEach(func() {
			Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
			Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))

			otherGuids = []string{NewGuid(), NewGuid()}
			for _, otherGuid := range otherGuids {
				otherTask := Task()
				Ω(bbsClient.DesireTask(otherGuid, otherDomain, otherTask)).Should(Succeed())
				Eventually(TaskGetter(otherGuid)).Should(HaveTaskState(models.Task_Completed))
			}
		})

		AfterEach(func() {
			Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
			Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			for _, otherGuid := range otherGuids {
				Ω(bbsClient.ResolvingTask(otherGuid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(otherGuid)).Should(Succeed())
			}
		})

		It("should fetch tasks in the given domain", func() {
			tasksInDomain, err := bbsClient.TasksByDomain(domain)
			Ω(err).ShouldNot(HaveOccurred())

			tasksInOtherDomain, err := bbsClient.TasksByDomain(otherDomain)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(tasksInDomain).Should(HaveLen(1))
			Ω(tasksInOtherDomain).Should(HaveLen(2))
			Ω([]string{tasksInOtherDomain[0].TaskGuid, tasksInOtherDomain[1].TaskGuid}).Should(ConsistOf(otherGuids))
		})

		It("should not error if a domain is empty", func() {
			tasks, err := bbsClient.TasksByDomain("farfignoogan")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(tasks).Should(BeEmpty())
		})

		It("should fetch all tasks", func() {
			allTasks, err := bbsClient.Tasks()
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
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
				_, err := bbsClient.TaskByGuid(guid)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when the task is not in the completed state", func() {
			It("should not be deleted, and should error", func() {
				task.Action = models.WrapAction(&models.RunAction{
					Path: "bash",
					Args: []string{"-c", "sleep 2; echo 'some output' > /tmp/bar"},
					User: "vcap",
				})
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Running))
				err := bbsClient.ResolvingTask(guid)
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_InvalidStateTransition))

				_, err = bbsClient.TasksByDomain(domain)
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(TaskGetter(guid)).Should(HaveTaskState(models.Task_Completed))
				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("when the task does not exist", func() {
			It("should not be deleted, and should error", func() {
				err := bbsClient.ResolvingTask("floobeedoobee")
				Ω(models.ConvertError(err).Type).Should(Equal(models.Error_ResourceNotFound))
			})
		})
	})

	Describe("{LOCAL} Registering Completion Callbacks", func() {
		var server *ghttp.Server
		var port string
		var status int
		var done chan struct{}

		BeforeEach(func() {
			status = http.StatusOK

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
					var receivedTask models.Task
					json.NewDecoder(req.Body).Decode(&receivedTask)
					Ω(receivedTask.TaskGuid).Should(Equal(guid))
					close(done)
				},
			))

			task.CompletionCallbackUrl = "http://" + hostAddress + ":" + port + "/endpoint"
		})

		AfterEach(func() {
			server.Close()
		})

		Context("when the server responds succesfully", func() {
			BeforeEach(func() {
				status = http.StatusOK
			})

			It("cleans up the task", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("when the server responds in the 4XX range", func() {
			BeforeEach(func() {
				status = http.StatusNotFound
			})

			It("nonetheless, cleans up the task", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(guid)
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
						var receivedTask models.Task
						json.NewDecoder(req.Body).Decode(&receivedTask)
						Ω(receivedTask.TaskGuid).Should(Equal(guid))
						close(secondDone)
					},
				))
			})

			It("should retry", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(done).Should(BeClosed())
				Eventually(secondDone).Should(BeClosed())
				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})

		Context("[Regression: #84595244] when there's no room for the Task", func() {
			BeforeEach(func() {
				task.MemoryMb = 1024 * 1024
			})

			It("should hit the callback", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(done).Should(BeClosed())

				Eventually(func() bool {
					_, err := bbsClient.TaskByGuid(guid)
					return err == nil
				}).Should(BeFalse(), "Eventually, the task should be resolved")
			})
		})
	})

	Describe("when the Task cannot be allocated", func() {
		Context("because it's too large", func() {
			BeforeEach(func() {
				task.MemoryMb = 1024 * 1024
			})

			It("should allow creation of the task but should (fairly quickly) mark the task as failed", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(TaskGetter(guid), 5).Should(HaveTaskState(models.Task_Completed))

				retreivedTask, err := bbsClient.TaskByGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(retreivedTask.Failed).Should(BeTrue())
				Ω(retreivedTask.FailureReason).Should(ContainSubstring("insufficient resources"))

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})

		Context("because of a stack mismatch", func() {
			BeforeEach(func() {
				task.RootFs = models.PreloadedRootFS("fruitfs")
			})

			It("should allow creation of the task but should (fairly quickly) mark the task as failed", func() {
				Ω(bbsClient.DesireTask(guid, domain, task)).Should(Succeed())
				Eventually(TaskGetter(guid), 5).Should(HaveTaskState(models.Task_Completed))

				retreivedTask, err := bbsClient.TaskByGuid(guid)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(retreivedTask.Failed).Should(BeTrue())
				Ω(retreivedTask.FailureReason).Should(ContainSubstring("found no compatible cell"))

				Ω(bbsClient.ResolvingTask(guid)).Should(Succeed())
				Ω(bbsClient.DeleteTask(guid)).Should(Succeed())
			})
		})
	})
})
