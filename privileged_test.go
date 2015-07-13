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
	var runUser string
	var containerPrivileged bool

	// TODO: remove once cflinuxfs2 rootfs no longer has its /etc/seed script
	var rootfs string
	var shell string
	var timeout int

	JustBeforeEach(func() {
		task = TaskWithGuid(guid)
		task.RootFS = rootfs
		task.Privileged = containerPrivileged
		task.Action = &models.RunAction{
			Path: shell,
			Args: []string{"-c", "id > /tmp/bar; echo h > /proc/sysrq-trigger ; echo have_real_root=$? >> /tmp/bar"},
			User: runUser,
		}

		Ω(client.CreateTask(task)).Should(Succeed())
		Eventually(TaskGetter(guid), timeout).Should(HaveTaskState(receptor.TaskStateCompleted))
	})

	AfterEach(func() {
		Ω(client.DeleteTask(guid)).Should(Succeed())
	})

	Context("with a privileged container", func() {
		BeforeEach(func() {
			containerPrivileged = true

			// TODO: remove once cflinuxfs2 rootfs no longer has its /etc/seed script
			rootfs = defaultRootFS
			shell = "bash"
			timeout = 10
		})

		//{LOCAL} because: privileged may not be allowed in the remote environment
		Context("{LOCAL} when running a privileged action", func() {
			BeforeEach(func() {
				runUser = "root"
			})

			It("should run as root", func() {
				completedTask, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(completedTask.Result).Should(ContainSubstring("uid=0(root)"), "If this fails, then your executor may not be configured to allow privileged actions")
				Ω(completedTask.Result).Should(MatchRegexp(`groups=.*0\(root\)`))
				Ω(completedTask.Result).Should(ContainSubstring("have_real_root=0"))
			})
		})

		Context("when running a non-privileged action", func() {
			BeforeEach(func() {
				runUser = "vcap"
			})

			It("should run as non-root", func() {
				completedTask, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(completedTask.Result).Should(MatchRegexp(`uid=\d{4,5}\(vcap\)`))
				Ω(completedTask.Result).Should(MatchRegexp(`groups=.*0\(root\)`))
				Ω(completedTask.Result).Should(ContainSubstring("have_real_root=1"))
			})
		})
	})

	Context("with an unprivileged container", func() {
		BeforeEach(func() {
			containerPrivileged = false

			// TODO: remove once cflinuxfs2 rootfs no longer has its /etc/seed script
			rootfs = "docker:///busybox"
			shell = "sh"
			timeout = 120
		})

		//{LOCAL} because: privileged may not be allowed in the remote environment
		Context("{LOCAL} when running a privileged action", func() {
			BeforeEach(func() {
				runUser = "root"
			})

			It("should run as root", func() {
				completedTask, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(completedTask.Result).Should(ContainSubstring("uid=0(root)"), "If this fails, then your executor may not be configured to allow privileged actions")
				Ω(completedTask.Result).Should(ContainSubstring("have_real_root=1"))
			})
		})

		Context("when running a non-privileged action", func() {
			BeforeEach(func() {
				runUser = "vcap"
			})

			It("should run as non-root", func() {
				completedTask, err := client.GetTask(guid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(completedTask.Result).Should(MatchRegexp(`uid=\d{4,5}\(vcap\)`))
				Ω(completedTask.Result).Should(ContainSubstring("have_real_root=1"))
			})
		})
	})
})
