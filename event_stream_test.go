package vizzini_test

import (
	"sync"

	"github.com/cloudfoundry-incubator/receptor"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"
)

var _ = Describe("EventStream", func() {
	var desiredLRP receptor.DesiredLRPCreateRequest
	var eventSource receptor.EventSource
	var done chan struct{}
	var lock *sync.Mutex
	var events []receptor.Event

	getEvents := func() []receptor.Event {
		lock.Lock()
		defer lock.Unlock()
		return events
	}

	BeforeEach(func() {
		var err error
		desiredLRP = DesiredLRPWithGuid(guid)
		eventSource, err = client.SubscribeToEvents()
		Ω(err).ShouldNot(HaveOccurred())

		done = make(chan struct{})
		lock = &sync.Mutex{}
		events = []receptor.Event{}

		go func() {
			for {
				event, err := eventSource.Next()
				if err != nil {
					close(done)
					return
				}
				lock.Lock()
				events = append(events, event)
				lock.Unlock()
			}
		}()
	})

	AfterEach(func() {
		eventSource.Close()
		Eventually(done).Should(BeClosed())
	})

	It("should receive events as the LRP goes through its lifecycle", func() {
		client.CreateDesiredLRP(desiredLRP)
		Eventually(getEvents).Should(ContainElement(MatchDesiredLRPCreatedEvent(guid)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPCreatedEvent(guid, 0)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPChangedEvent(guid, 0, receptor.ActualLRPStateClaimed)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPChangedEvent(guid, 0, receptor.ActualLRPStateRunning)))
		client.DeleteDesiredLRP(guid)
		Eventually(getEvents).Should(ContainElement(MatchDesiredLRPRemovedEvent(guid)))
		Eventually(getEvents).Should(ContainElement(MatchActualLRPRemovedEvent(guid, 0)))
	})
})
