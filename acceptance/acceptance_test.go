package acceptance_test

import (
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Acceptance", func() {
	It("should immediately respond to diego.docker.staging.start", func() {
		client := yagnats.NewClient()
		err := client.Connect(&yagnats.ConnectionInfo{
			Addr:     "10.244.0.6.xip.io:4222",
			Username: "nats",
			Password: "nats",
		})
		Ω(err).ShouldNot(HaveOccurred())

		payload := make(chan []byte)
		_, err = client.Subscribe("diego.docker.staging.finished", func(message *yagnats.Message) {
			payload <- message.Payload
		})
		Ω(err).ShouldNot(HaveOccurred())

		err = client.Publish("diego.docker.staging.start", []byte(`{
      "app_id": "some-app-guid",
      "task_id": "some-task-guid"
    }`))
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(payload).Should(Receive(MatchJSON(`{
      "app_id": "some-app-guid",
      "task_id": "some-task-guid"
    }`)))
	})
})
