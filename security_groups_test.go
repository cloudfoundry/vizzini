package vizzini_test

import (
	"fmt"
	"net/http"

	"github.com/cloudfoundry-incubator/bbs/models"
	. "github.com/cloudfoundry-incubator/vizzini/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Security groups", func() {
	var listener *models.DesiredLRP
	var listenerGuid string
	var protectedURL string

	BeforeEach(func() {
		listenerGuid = NewGuid()
		listener = DesiredLRPWithGuid(listenerGuid)

		Expect(bbsClient.DesireLRP(listener)).To(Succeed())
		Eventually(ActualGetter(listenerGuid, 0)).Should(BeActualLRPWithState(listenerGuid, 0, models.ActualLRPStateRunning))
		Eventually(EndpointCurler("http://" + RouteForGuid(listenerGuid) + "/env")).Should(Equal(http.StatusOK))

		listenerActual, err := ActualLRPByProcessGuidAndIndex(listenerGuid, 0)
		Expect(err).NotTo(HaveOccurred())
		protectedURL = fmt.Sprintf("http://%s:%d/env", listenerActual.Address, listenerActual.Ports[0].HostPort)
	})

	Context("for LRPs", func() {
		var allowedCaller, disallowedCaller *models.DesiredLRP
		var allowedCallerGuid, disallowedCallerGuid string

		BeforeEach(func() {
			allowedCallerGuid, disallowedCallerGuid = NewGuid(), NewGuid()
			allowedCaller, disallowedCaller = DesiredLRPWithGuid(allowedCallerGuid), DesiredLRPWithGuid(disallowedCallerGuid)

			Expect(bbsClient.DesireLRP(disallowedCaller)).To(Succeed())
			Eventually(ActualGetter(disallowedCallerGuid, 0)).Should(BeActualLRPWithState(disallowedCallerGuid, 0, models.ActualLRPStateRunning))
			Eventually(EndpointCurler("http://" + RouteForGuid(disallowedCallerGuid) + "/env")).Should(Equal(http.StatusOK))

			allowedCaller.EgressRules = []*models.SecurityGroupRule{
				{
					Protocol:     models.AllProtocol,
					Destinations: []string{"0.0.0.0/0"},
				},
			}

			Expect(bbsClient.DesireLRP(allowedCaller)).To(Succeed())
			Eventually(ActualGetter(allowedCallerGuid, 0)).Should(BeActualLRPWithState(allowedCallerGuid, 0, models.ActualLRPStateRunning))
			Eventually(EndpointCurler("http://" + RouteForGuid(allowedCallerGuid) + "/env")).Should(Equal(http.StatusOK))
		})

		It("should allow access to the opened up location", func() {
			urlToProxyThroughDisallowedCaller := fmt.Sprintf("http://"+RouteForGuid(disallowedCallerGuid)+"/curl?url=%s", protectedURL)
			urlToProxyThroughAllowedCaller := fmt.Sprintf("http://"+RouteForGuid(allowedCallerGuid)+"/curl?url=%s", protectedURL)

			By("verifiying that calling into the VPC is disallowed")
			resp, err := http.Get(urlToProxyThroughDisallowedCaller)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))

			By("asserting that opening up the security group rules allow us to call into the VPC")
			resp, err = http.Get(urlToProxyThroughAllowedCaller)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Context("for Tasks", func() {
		var allowedTask, disallowedTask *models.TaskDefinition
		var allowedTaskGuid, disallowedTaskGuid string

		BeforeEach(func() {
			allowedTaskGuid, disallowedTaskGuid = NewGuid(), NewGuid()
			allowedTask, disallowedTask = Task(), Task()
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
			Expect(bbsClient.DesireTask(allowedTaskGuid, domain, allowedTask)).To(Succeed())
			Expect(bbsClient.DesireTask(disallowedTaskGuid, domain, disallowedTask)).To(Succeed())

			Eventually(TaskGetter(allowedTaskGuid)).Should(HaveTaskState(models.Task_Completed))
			Eventually(TaskGetter(disallowedTaskGuid)).Should(HaveTaskState(models.Task_Completed))

			task, err := bbsClient.TaskByGuid(disallowedTaskGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.Failed).To(Equal(true))

			task, err = bbsClient.TaskByGuid(allowedTaskGuid)
			Expect(err).NotTo(HaveOccurred())
			Expect(task.Failed).To(Equal(false))

			Expect(bbsClient.ResolvingTask(allowedTaskGuid)).To(Succeed())
			Expect(bbsClient.DeleteTask(allowedTaskGuid)).To(Succeed())
			Expect(bbsClient.ResolvingTask(disallowedTaskGuid)).To(Succeed())
			Expect(bbsClient.DeleteTask(disallowedTaskGuid)).To(Succeed())
		})
	})
})
