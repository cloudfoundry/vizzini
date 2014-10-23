package acceptance_test

import (
	"net/http"

	"github.com/cloudfoundry-incubator/executor/http/client"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/runtime-schema/models"

	"github.com/cloudfoundry-incubator/executor"
)

var _ = Describe("Executor", func() {
	var c executor.Client

	BeforeEach(func() {
		c = client.New(http.DefaultClient, "http://10.244.17.2.xip.io:1700")
	})

	Describe("Fetching RunResult from the container (#79618100)", func() {
		var allocationGuid string
		BeforeEach(func() {
			allocationGuid = NewGuid()
			_, err := c.AllocateContainer(allocationGuid, executor.ContainerAllocationRequest{
				MemoryMB: 128,
				DiskMB:   128,
			})
			Ω(err).ShouldNot(HaveOccurred())

			_, err = c.InitializeContainer(allocationGuid, executor.ContainerInitializationRequest{})
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			Ω(c.DeleteContainer(allocationGuid)).Should(Succeed())
		})

		Context("when the actions succeed", func() {
			It("should report the run result on the container", func() {
				c.Run(allocationGuid, executor.ContainerRunRequest{
					Actions: []models.ExecutorAction{
						{
							models.RunAction{
								Path: "echo",
								Args: []string{"hello"},
							},
						},
					},
				})

				Eventually(func() string {
					container, err := c.GetContainer(allocationGuid)
					if err != nil {
						return ""
					}
					return container.RunResult.Guid
				}, 10).Should(Equal(allocationGuid))
				container, err := c.GetContainer(allocationGuid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(container.RunResult.Failed).Should(BeFalse())
			})
		})

		Context("when the actions fail", func() {
			It("should report the run result on the container", func() {
				c.Run(allocationGuid, executor.ContainerRunRequest{
					Actions: []models.ExecutorAction{
						{
							models.RunAction{
								Path: "cp",
								Args: []string{"farfignoogan", "madgascar"},
							},
						},
					},
				})

				Eventually(func() string {
					container, err := c.GetContainer(allocationGuid)
					if err != nil {
						return ""
					}
					return container.RunResult.Guid
				}, 10).Should(Equal(allocationGuid))
				container, err := c.GetContainer(allocationGuid)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(container.RunResult.Failed).Should(BeTrue())
				Ω(container.RunResult.FailureReason).Should(ContainSubstring("Exited with status 1"))
			})
		})
	})
})
