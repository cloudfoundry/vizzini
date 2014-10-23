package acceptance_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-incubator/runtime-schema/models"

	"github.com/cloudfoundry-incubator/executor"
	"github.com/cloudfoundry-incubator/executor/http/client"
)

var _ = Describe("Executor Performance", func() {
	var c executor.Client

	BeforeEach(func() {
		c = client.New(http.DefaultClient, "http://10.244.17.2.xip.io:1700")
		c.DeleteContainer("download-container")
	})

	It("should handle .tgz", func() {
		_, err := c.AllocateContainer("download-container", executor.ContainerAllocationRequest{
			MemoryMB: 1024,
			DiskMB:   10240,
		})
		Ω(err).ShouldNot(HaveOccurred())

		_, err = c.InitializeContainer("download-container", executor.ContainerInitializationRequest{})
		Ω(err).ShouldNot(HaveOccurred())

		c.Run("download-container", executor.ContainerRunRequest{
			Actions: []models.ExecutorAction{
				{models.DownloadAction{
					From:     "http://onsi-public.s3.amazonaws.com/foo.tar.gz",
					To:       "/tmp/foo-1",
					CacheKey: "container-cache-key-tar-gz",
				}},
			},
		})
	})

	It("should be performant", func() {
		_, err := c.AllocateContainer("download-container", executor.ContainerAllocationRequest{
			MemoryMB: 1024,
			DiskMB:   10240,
		})
		Ω(err).ShouldNot(HaveOccurred())

		_, err = c.InitializeContainer("download-container", executor.ContainerInitializationRequest{})
		Ω(err).ShouldNot(HaveOccurred())

		c.Run("download-container", executor.ContainerRunRequest{
			Actions: []models.ExecutorAction{
				models.Parallel([]models.ExecutorAction{
					{models.DownloadAction{
						From:     "http://onsi-public.s3.amazonaws.com/foo.zip",
						To:       "/tmp/foo-1",
						CacheKey: "container-cache-key-1",
					}},
					{models.DownloadAction{
						From:     "http://onsi-public.s3.amazonaws.com/foo.zip",
						To:       "/tmp/foo-2",
						CacheKey: "container-cache-key-2",
					}},
					{models.DownloadAction{
						From:     "http://onsi-public.s3.amazonaws.com/foo.zip",
						To:       "/tmp/foo-3",
						CacheKey: "container-cache-key-3",
					}},
					{models.DownloadAction{
						From:     "http://onsi-public.s3.amazonaws.com/foo.zip",
						To:       "/tmp/foo-4",
						CacheKey: "container-cache-key-4",
					}},
				}...),
			},
		})
	})
})
