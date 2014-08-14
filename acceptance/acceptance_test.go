package acceptance_test

import (
	"github.com/cloudfoundry-incubator/garden/client"
	"github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/warden"
	"github.com/cloudfoundry-incubator/runtime-schema/cc_messages"
	"github.com/cloudfoundry/yagnats"
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

	Describe("Docker", func() {
		var natsClient *yagnats.Client
		BeforeEach(func() {
			natsClient = yagnats.NewClient()
			err := natsClient.Connect(&yagnats.ConnectionInfo{
				Addr:     "10.244.0.6.xip.io:4222",
				Username: "nats",
				Password: "nats",
			})
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should immediately respond to diego.docker.staging.start", func() {
			payload := make(chan []byte)
			_, err := natsClient.Subscribe("diego.docker.staging.finished", func(message *yagnats.Message) {
				payload <- message.Payload
			})
			Ω(err).ShouldNot(HaveOccurred())

			err = natsClient.Publish("diego.docker.staging.start", []byte(`{
      "app_id": "some-app-guid",
      "task_id": "some-task-guid"
    }`))
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(payload).Should(Receive(MatchJSON(`{
      "app_id": "some-app-guid",
      "task_id": "some-task-guid"
    }`)))
		})

		It("should run a (heavy) docker-based app", func() {
			desireMessage := cc_messages.DesireAppRequestFromCC{
				ProcessGuid:    "my-grace-docker-app",
				DockerImageUrl: "docker:///onsi/grace",
				Stack:          "lucid64",
				StartCommand:   "/grace",
				Environment: cc_messages.Environment{
					{Name: "custom-env", Value: "grace-upon-grace"},
				},
				Routes:       []string{"docker-grace"},
				MemoryMB:     128,
				DiskMB:       1024,
				NumInstances: 1,
				LogGuid:      "docker-grace",
			}

			err := natsClient.Publish("diego.docker.desire.app", desireMessage.ToJSON())
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("should run a (light-weight) docker-based app", func() {
			desireMessage := cc_messages.DesireAppRequestFromCC{
				ProcessGuid:    "my-lite-grace-docker-app",
				DockerImageUrl: "docker:///onsi/grace-busybox",
				Stack:          "lucid64",
				StartCommand:   "/grace",
				Environment: cc_messages.Environment{
					{Name: "custom-env", Value: "grace-upon-grace"},
				},
				Routes:       []string{"lite-docker-grace"},
				MemoryMB:     128,
				DiskMB:       1024,
				NumInstances: 1,
				LogGuid:      "lite-docker-grace",
			}

			err := natsClient.Publish("diego.docker.desire.app", desireMessage.ToJSON())
			Ω(err).ShouldNot(HaveOccurred())
		})
	})
})
