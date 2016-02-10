package oneoffs_test

import (
	"fmt"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Garden fails to delete multiple Docker containers", func() {
	var gardenClient client.Client

	BeforeEach(func() {
		conn := connection.New(gardenNet, gardenAddr)
		gardenClient = client.New(conn)
	})

	runAndDeleteADockerContainer := func() error {
		defer GinkgoRecover()
		handle := NewGuid()
		_, err := gardenClient.Create(garden.ContainerSpec{
			Handle:     handle,
			RootFSPath: "docker:///onsi/grace-busybox",
		})
		Expect(err).NotTo(HaveOccurred())

		return gardenClient.Destroy(handle)
	}

	It("should fail to delete at least one of these Docker containers", func() {
		nDocker := 10

		lock := &sync.Mutex{}
		dockerDeleteErrors := []error{}

		wg := &sync.WaitGroup{}
		wg.Add(nDocker)

		for i := 0; i < nDocker; i++ {
			go func() {
				defer wg.Done()
				err := runAndDeleteADockerContainer()
				if err != nil {
					lock.Lock()
					dockerDeleteErrors = append(dockerDeleteErrors, err)
					lock.Unlock()
				}
			}()
		}

		wg.Wait()

		Expect(dockerDeleteErrors).NotTo(BeEmpty(), "Looks like the deletes worked -- try again!")

		fmt.Println("Succesfully entered The Bad State™")
		for _, err := range dockerDeleteErrors {
			fmt.Printf("  %s\n", err.Error())
		}
	})
})
