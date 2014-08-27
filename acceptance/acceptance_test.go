package acceptance_test

import (
	"fmt"
	"io"
	"time"

	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Acceptance", func() {
	Describe("Garden", func() {
		var gardenClient client.Client

		BeforeEach(func() {
			conn := connection.New("tcp", "10.244.17.2.xip.io:7777")
			gardenClient = client.New(conn)
		})

		It("should fail when attempting to delete a container twice (#76616270)", func() {
			_, err := gardenClient.Create(warden.ContainerSpec{
				Handle: "my-fun-handle",
			})
			Ω(err).ShouldNot(HaveOccurred())

			var errors = make(chan error)
			go func() {
				errors <- gardenClient.Destroy("my-fun-handle")
			}()
			go func() {
				errors <- gardenClient.Destroy("my-fun-handle")
			}()

			results := []error{
				<-errors,
				<-errors,
			}

			Ω(results).Should(ConsistOf(BeNil(), HaveOccurred()))
		})

		It("should support setting environment variables on the container (#77303456)", func() {
			container, err := gardenClient.Create(warden.ContainerSpec{
				Handle: "cap'n-planet",
				Env: []string{
					"ROOT_ENV=A",
					"OVERWRITTEN_ENV=B",
					"HOME=/nowhere",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			buffer := gbytes.NewBuffer()
			process, err := container.Run(warden.ProcessSpec{
				Path: "bash",
				Args: []string{"-c", "printenv"},
				Env: []string{
					"OVERWRITTEN_ENV=C",
				},
			}, warden.ProcessIO{
				Stdout: io.MultiWriter(buffer, GinkgoWriter),
				Stderr: io.MultiWriter(buffer, GinkgoWriter),
			})

			Ω(err).ShouldNot(HaveOccurred())

			process.Wait()

			gardenClient.Destroy("cap'n-planet")

			Ω(buffer.Contents()).Should(ContainSubstring("OVERWRITTEN_ENV=C"))
			Ω(buffer.Contents()).ShouldNot(ContainSubstring("OVERWRITTEN_ENV=B"))
			Ω(buffer.Contents()).Should(ContainSubstring("HOME=/home/vcap"))
			Ω(buffer.Contents()).ShouldNot(ContainSubstring("HOME=/nowhere"))
			Ω(buffer.Contents()).Should(ContainSubstring("ROOT_ENV=A"))
		})

		XDescribe("Bugs around the container lifecycle (#77768828)", func() {
			It("should support deleting a container after an errant delete", func() {
				handle := fmt.Sprintf("%d", time.Now().UnixNano())
				err := gardenClient.Destroy(handle)
				Ω(err).Should(HaveOccurred())

				_, err = gardenClient.Create(warden.ContainerSpec{
					Handle: handle,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = gardenClient.Lookup(handle)
				Ω(err).ShouldNot(HaveOccurred())

				err = gardenClient.Destroy(handle)
				Ω(err).ShouldNot(HaveOccurred(), "Expected no error when attempting to destroy this container")

				_, err = gardenClient.Lookup(handle)
				Ω(err).Should(HaveOccurred())
			})

			It("should not allow creating an already existing container", func() {
				handle := fmt.Sprintf("%d", time.Now().UnixNano())

				_, err := gardenClient.Create(warden.ContainerSpec{
					Handle: handle,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = gardenClient.Create(warden.ContainerSpec{
					Handle: handle,
				})
				Ω(err).Should(HaveOccurred(), "Expected an error when creating a Garden container with an existing handle")
			})
		})

		XDescribe("Bugs with snapshotting (#77767958)", func() {
			BeforeEach(func() {
				fmt.Println(`
!!!READ THIS!!!
Using this test is non-trivial.  You must:

- Focus the "Bugs with snapshotting" Describe
- Make sure you are running bosh-lite
- Make sure the -snapshots flag is set in the control script for the warden running in your cell.
- Run this test the first time: this will create containers and both tests should pass.
- Run this test again: it should say that it will NOT create the container and still pass.
- bosh ssh to the cell and monit restart warden
- wait a bit and make sure warden is back up
- Run this test again -- this time these tests will fail with 500.
- Run it a few more times, eventually (I've found) it starts passing again.
`)
			})

			It("should support snapshotting", func() {
				handle := "snapshotable-container"
				_, err := gardenClient.Lookup(handle)
				if err != nil {
					fmt.Println("CREATING CONTAINER")

					_, err = gardenClient.Create(warden.ContainerSpec{
						Handle: handle,
						Env: []string{
							"ROOT_ENV=A",
							"OVERWRITTEN_ENV=B",
							"HOME=/nowhere",
						},
					})
					Ω(err).ShouldNot(HaveOccurred())
				} else {
					fmt.Println("NOT CREATING CONTAINER")
				}

				container, err := gardenClient.Lookup(handle)
				Ω(err).ShouldNot(HaveOccurred())
				buffer := gbytes.NewBuffer()
				process, err := container.Run(warden.ProcessSpec{
					Path: "bash",
					Args: []string{"-c", "printenv"},
					Env: []string{
						"OVERWRITTEN_ENV=C",
					},
				}, warden.ProcessIO{
					Stdout: io.MultiWriter(buffer, GinkgoWriter),
					Stderr: io.MultiWriter(buffer, GinkgoWriter),
				})

				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				Ω(buffer.Contents()).Should(ContainSubstring("OVERWRITTEN_ENV=C"))
				Ω(buffer.Contents()).ShouldNot(ContainSubstring("OVERWRITTEN_ENV=B"))
				Ω(buffer.Contents()).Should(ContainSubstring("HOME=/home/vcap"))
				Ω(buffer.Contents()).ShouldNot(ContainSubstring("HOME=/nowhere"))
				Ω(buffer.Contents()).Should(ContainSubstring("ROOT_ENV=A"))
			})
		})

		It("should mount an ubuntu docker image, just fine", func() {
			container, err := gardenClient.Create(warden.ContainerSpec{
				Handle:     "my-ubuntu-based-docker-image",
				RootFSPath: "docker:///onsi/grace",
			})
			Ω(err).ShouldNot(HaveOccurred())

			process, err := container.Run(warden.ProcessSpec{
				Path: "ls",
			}, warden.ProcessIO{
				Stdout: GinkgoWriter,
				Stderr: GinkgoWriter,
			})
			Ω(err).ShouldNot(HaveOccurred())

			process.Wait()

			gardenClient.Destroy("my-ubuntu-based-docker-image")
		})

		It("should mount a none-ubuntu docker image, just fine", func() {
			container, err := gardenClient.Create(warden.ContainerSpec{
				Handle:     "my-none-ubuntu-based-docker-image",
				RootFSPath: "docker:///onsi/grace-busybox",
			})
			Ω(err).ShouldNot(HaveOccurred())

			process, err := container.Run(warden.ProcessSpec{
				Path: "ls",
			}, warden.ProcessIO{
				Stdout: GinkgoWriter,
				Stderr: GinkgoWriter,
			})
			Ω(err).ShouldNot(HaveOccurred())

			process.Wait()

			gardenClient.Destroy("my-none-ubuntu-based-docker-image")
		})
	})
})
