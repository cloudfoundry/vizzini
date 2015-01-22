package vizzini_test

import (
	"github.com/cloudfoundry-incubator/receptor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"
)

var _ = Describe("EventStream", func() {
	var desiredLRP receptor.DesiredLRPCreateRequest
	var eventSource receptor.EventSource
	var events chan receptor.Event
	var done chan struct{}

	BeforeEach(func() {
		var err error
		desiredLRP = DesiredLRPWithGuid(guid)
		eventSource, err = client.SubscribeToEvents()
		Ω(err).ShouldNot(HaveOccurred())

		events = make(chan receptor.Event, 10000)
		done = make(chan struct{})

		go func() {
			for {
				event, err := eventSource.Next()
				if err != nil {
					close(done)
					return
				}
				events <- event
			}
		}()
	})

	AfterEach(func() {
		eventSource.Close()
		Eventually(done).Should(BeClosed(), "This fails right now.... #84607000")
	})

	It("should receive events as the LRP goes through its lifecycle", func() {
		client.CreateDesiredLRP(desiredLRP)
		Eventually(events).Should(Receive(MatchDesiredLRPCreatedEvent(guid)))
		Eventually(events).Should(Receive(MatchActualLRPCreatedEvent(guid, 0)))
		Eventually(events).Should(Receive(MatchActualLRPChangedEvent(guid, 0, receptor.ActualLRPStateClaimed)))
		Eventually(events).Should(Receive(MatchActualLRPChangedEvent(guid, 0, receptor.ActualLRPStateRunning)))
		client.DeleteDesiredLRP(guid)
		Eventually(events).Should(Receive(MatchDesiredLRPRemovedEvent(guid)))
		Eventually(events).Should(Receive(MatchActualLRPRemovedEvent(guid, 0)))
	})
})
