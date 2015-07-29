package vizzini_test

import (
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/receptor"
	oldmodels "github.com/cloudfoundry-incubator/runtime-schema/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"
)

var _ = Describe("Security groups", func() {
	var listener receptor.DesiredLRPCreateRequest
	var listenerGuid string
	var protectedURL string

	BeforeEach(func() {
		listenerGuid = NewGuid()
		listener = DesiredLRPWithGuid(listenerGuid)

		Ω(client.CreateDesiredLRP(listener)).Should(Succeed())
		Eventually(ActualGetter(listenerGuid, 0)).Should(BeActualLRPWithState(listenerGuid, 0, receptor.ActualLRPStateRunning))
		Eventually(EndpointCurler("http://" + RouteForGuid(listenerGuid) + "/env")).Should(Equal(http.StatusOK))

		listenerActual, err := client.ActualLRPByProcessGuidAndIndex(listenerGuid, 0)
		Ω(err).ShouldNot(HaveOccurred())
		protectedURL = fmt.Sprintf("http://%s:%d/env", listenerActual.Address, listenerActual.Ports[0].HostPort)
	})

	Context("for LRPs", func() {
		var allowedCaller, disallowedCaller receptor.DesiredLRPCreateRequest
		var allowedCallerGuid, disallowedCallerGuid string

		BeforeEach(func() {
			allowedCallerGuid, disallowedCallerGuid = NewGuid(), NewGuid()
			allowedCaller, disallowedCaller = DesiredLRPWithGuid(allowedCallerGuid), DesiredLRPWithGuid(disallowedCallerGuid)

			Ω(client.CreateDesiredLRP(disallowedCaller)).Should(Succeed())
			Eventually(ActualGetter(disallowedCallerGuid, 0)).Should(BeActualLRPWithState(disallowedCallerGuid, 0, receptor.ActualLRPStateRunning))
			Eventually(EndpointCurler("http://" + RouteForGuid(disallowedCallerGuid) + "/env")).Should(Equal(http.StatusOK))

			allowedCaller.EgressRules = []oldmodels.SecurityGroupRule{
				{
					Protocol:     oldmodels.AllProtocol,
					Destinations: []string{"0.0.0.0/0"},
				},
			}

			Ω(client.CreateDesiredLRP(allowedCaller)).Should(Succeed())
			Eventually(ActualGetter(allowedCallerGuid, 0)).Should(BeActualLRPWithState(allowedCallerGuid, 0, receptor.ActualLRPStateRunning))
			Eventually(EndpointCurler("http://" + RouteForGuid(allowedCallerGuid) + "/env")).Should(Equal(http.StatusOK))
		})

		It("should allow access to the opened up location", func() {
			urlToProxyThroughDisallowedCaller := fmt.Sprintf("http://"+RouteForGuid(disallowedCallerGuid)+"/curl?url=%s", protectedURL)
			urlToProxyThroughAllowedCaller := fmt.Sprintf("http://"+RouteForGuid(allowedCallerGuid)+"/curl?url=%s", protectedURL)

			By("verifiying that calling into the VPC is disallowed")
			resp, err := http.Get(urlToProxyThroughDisallowedCaller)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(resp.StatusCode).Should(Equal(http.StatusInternalServerError))

			By("asserting that opening up the security group rules allow us to call into the VPC")
			resp, err = http.Get(urlToProxyThroughAllowedCaller)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(resp.StatusCode).Should(Equal(http.StatusOK))
		})
	})

	Context("for Tasks", func() {
		var allowedTask, disallowedTask receptor.TaskCreateRequest
		var allowedTaskGuid, disallowedTaskGuid string

		BeforeEach(func() {
			allowedTaskGuid, disallowedTaskGuid = NewGuid(), NewGuid()
			allowedTask, disallowedTask = TaskWithGuid(allowedTaskGuid), TaskWithGuid(disallowedTaskGuid)
			allowedTask.ResultFile, disallowedTask.ResultFile = "", ""

			disallowedTask.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Args: []string{"-c", fmt.Sprintf("curl %s", protectedURL)},
				User: "vcap",
			})

			allowedTask.Action = models.WrapAction(&models.RunAction{
				Path: "bash",
				Args: []string{"-c", fmt.Sprintf("curl %s", protectedURL)},
				User: "vcap",
			})

			allowedTask.EgressRules = []*models.SecurityGroupRule{
				{
					Protocol:     models.AllProtocol,
					Destinations: []string{"0.0.0.0/0"},
				},
			}
		})

		It("should allow access to the opened up location", func() {
			Ω(client.CreateTask(allowedTask)).Should(Succeed())
			Ω(client.CreateTask(disallowedTask)).Should(Succeed())

			Eventually(TaskGetter(allowedTaskGuid)).Should(HaveTaskState(receptor.TaskStateCompleted))
			Eventually(TaskGetter(disallowedTaskGuid)).Should(HaveTaskState(receptor.TaskStateCompleted))

			task, err := client.GetTask(disallowedTaskGuid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Failed).Should(Equal(true))

			task, err = client.GetTask(allowedTaskGuid)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(task.Failed).Should(Equal(false))

			Ω(client.DeleteTask(allowedTaskGuid)).Should(Succeed())
			Ω(client.DeleteTask(disallowedTaskGuid)).Should(Succeed())
		})
	})
})
