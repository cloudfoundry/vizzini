package acceptance_test

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/cloudfoundry-incubator/garden/api"
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func uniqueHandle() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

var _ = Describe("Garden Acceptance Tests", func() {
	var gardenClient client.Client

	BeforeEach(func() {
		conn := connection.New(gardenNet, gardenAddr)
		gardenClient = client.New(conn)
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

				_, err = gardenClient.Create(api.ContainerSpec{
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
			process, err := container.Run(api.ProcessSpec{
				Path: "bash",
				Args: []string{"-c", "printenv"},
				Env: []string{
					"OVERWRITTEN_ENV=C",
				},
			}, api.ProcessIO{
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

	Describe("Streaming things into a container repeatedly", func() {
		/*
			Turns out... this *is* slow.
			But it's not Garden
			It's not AUFS
			It's linux: https://bugzilla.kernel.org/show_bug.cgi?id=12309
		*/
		var tarBuffer []byte
		var fileSize int64

		var handle string
		var container api.Container

		BeforeEach(func() {
			fileSize = 700 * 1024 * 1024
			tarBufferWriter := &bytes.Buffer{}

			tarWriter := tar.NewWriter(tarBufferWriter)
			Ω(tarWriter.WriteHeader(&tar.Header{
				Name: "big-file",
				Size: fileSize,
			})).Should(Succeed())

			writtenSize, err := io.CopyN(tarWriter, NewRandbo(), fileSize)
			Ω(writtenSize).Should(Equal(fileSize))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(tarWriter.Close()).Should(Succeed())

			tarBuffer = tarBufferWriter.Bytes()

			handle = NewGuid()
			container, err = gardenClient.Create(api.ContainerSpec{
				Handle: handle,
			})
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			Ω(gardenClient.Destroy(handle)).Should(Succeed())
		})

		Measure("should not be slow", func(b Benchmarker) {
			dt := b.Time("streaming in all files", func() {
				for i := 0; i < 14; i++ {
					dt := b.Time("streaming in individual files", func() {
						Ω(container.StreamIn(fmt.Sprintf("/tmp/tarball-%d", i), bytes.NewReader(tarBuffer))).Should(Succeed())
					})

					fmt.Printf("streaming in %d/14 took %s\n", i+1, dt)
				}
			})
			fmt.Printf("streaming them all in took %s\n", dt)
		}, 2)
	})

	/////////////////////////////////////////////////////////////////////////////////////////////////////

	Describe("things that now work", func() {
		Describe("Properties (#81124130)", func() {
			var handle string
			var container api.Container

			BeforeEach(func() {
				var err error
				handle = NewGuid()
				container, err = gardenClient.Create(api.ContainerSpec{
					Handle: handle,
					Properties: api.Properties{
						"A": "blaster",
						"B": "lightsaber",
					},
				})

				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				Ω(gardenClient.Destroy(handle)).Should(Succeed())
			})

			It("should allow CRUDing properties", func() {
				By("getting properties set on creation")
				Ω(container.GetProperty("A")).Should(Equal("blaster"))
				Ω(container.GetProperty("B")).Should(Equal("lightsaber"))

				By("setting and getting new properties")
				Ω(container.SetProperty("C", "x-wing")).Should(Succeed())
				Ω(container.GetProperty("C")).Should(Equal("x-wing"))

				By("modifying existing properties")
				Ω(container.SetProperty("A", "phaser")).Should(Succeed())
				Ω(container.GetProperty("A")).Should(Equal("phaser"))
				Ω(container.SetProperty("C", "enterprise")).Should(Succeed())
				Ω(container.GetProperty("C")).Should(Equal("enterprise"))

				By("fetching non-existing properties")
				property, err := container.GetProperty("D")
				Ω(property).Should(BeZero())
				Ω(err).Should(HaveOccurred())

				By("removing properites")
				Ω(container.RemoveProperty("A")).Should(Succeed())
				Ω(container.RemoveProperty("C")).Should(Succeed())
				_, err = container.GetProperty("A")
				Ω(err).Should(HaveOccurred())
				_, err = container.GetProperty("C")
				Ω(err).Should(HaveOccurred())
				Ω(container.GetProperty("B")).Should(Equal("lightsaber"))
			})
		})

		It("should fail when attempting to delete a container twice (#76616270)", func() {
			_, err := gardenClient.Create(api.ContainerSpec{
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
			container, err := gardenClient.Create(api.ContainerSpec{
				Handle: "cap'n-planet",
				Env: []string{
					"ROOT_ENV=A",
					"OVERWRITTEN_ENV=B",
					"HOME=/nowhere",
				},
			})
			Ω(err).ShouldNot(HaveOccurred())

			buffer := gbytes.NewBuffer()
			process, err := container.Run(api.ProcessSpec{
				Path: "bash",
				Args: []string{"-c", "printenv"},
				Env: []string{
					"OVERWRITTEN_ENV=C",
				},
			}, api.ProcessIO{
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

		It("should fail when creating a container shows rootfs does not have /bin/sh (#77771202)", func() {
			handle := uniqueHandle()
			_, err := gardenClient.Create(api.ContainerSpec{
				Handle:     handle,
				RootFSPath: "docker:///cloudfoundry/empty",
			})
			Ω(err).Should(HaveOccurred())
		})

		Describe("Bugs around the container lifecycle (#77768828)", func() {
			It("should support deleting a container after an errant delete", func() {
				handle := fmt.Sprintf("%d", time.Now().UnixNano())
				err := gardenClient.Destroy(handle)
				Ω(err).Should(HaveOccurred())

				_, err = gardenClient.Create(api.ContainerSpec{
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

				_, err := gardenClient.Create(api.ContainerSpec{
					Handle: handle,
				})
				Ω(err).ShouldNot(HaveOccurred())

				_, err = gardenClient.Create(api.ContainerSpec{
					Handle: handle,
				})
				Ω(err).Should(HaveOccurred(), "Expected an error when creating a Garden container with an existing handle")

				gardenClient.Destroy(handle)
			})
		})

		Describe("mounting docker images", func() {
			It("should mount an ubuntu docker image, just fine", func() {
				container, err := gardenClient.Create(api.ContainerSpec{
					Handle:     "my-ubuntu-based-docker-image",
					RootFSPath: "docker:///onsi/grace",
				})
				Ω(err).ShouldNot(HaveOccurred())

				process, err := container.Run(api.ProcessSpec{
					Path: "ls",
				}, api.ProcessIO{
					Stdout: GinkgoWriter,
					Stderr: GinkgoWriter,
				})
				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				gardenClient.Destroy("my-ubuntu-based-docker-image")
			})

			It("should mount a none-ubuntu docker image, just fine", func() {
				container, err := gardenClient.Create(api.ContainerSpec{
					Handle:     "my-none-ubuntu-based-docker-image",
					RootFSPath: "docker:///onsi/grace-busybox",
				})
				Ω(err).ShouldNot(HaveOccurred())

				process, err := container.Run(api.ProcessSpec{
					Path: "ls",
				}, api.ProcessIO{
					Stdout: GinkgoWriter,
					Stderr: GinkgoWriter,
				})
				Ω(err).ShouldNot(HaveOccurred())

				process.Wait()

				gardenClient.Destroy("my-none-ubuntu-based-docker-image")
			})
		})
	})
})
