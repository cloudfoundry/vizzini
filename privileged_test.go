package vizzini_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Privileged", func() {
	var task receptor.TaskCreateRequest
	var guid string
	var privileged bool

	JustBeforeEach(func() {
		guid = NewGuid()
		task = receptor.TaskCreateRequest{
			TaskGuid:   guid,
			Domain:     domain,
			RootFSPath: rootFS,
			Action: models.ExecutorAction{
				models.RunAction{
					Path:       "bash",
					Args:       []string{"-c", "id > /tmp/bar"},
					Privileged: privileged,
				},
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
		Eventually(TaskGetter(guid)).Should(HaveTaskState(receptor.TaskStateCompleted))
	})

	AfterEach(func() {
		ClearOutTasksInDomain(domain)
	})

	Context("{PRIVILEGED} when running a privileged action", func() {
		BeforeEach(func() {
			privileged = true
		})

		It("should run as root", func() {
			completedTask, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(completedTask.Result).Should(ContainSubstring("uid=0(root)"), "If this fails, then your executor may not be configured to allow privileged actions")
		})
	})

	Context("when running a non-privileged action", func() {
		BeforeEach(func() {
			privileged = false
		})

		It("should run as non-root", func() {
			completedTask, err := client.GetTask(guid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(completedTask.Result).Should(MatchRegexp(`uid=\d{5}\(vcap\)`))
		})
	})
})
