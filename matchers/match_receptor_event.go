package matchers

import (
	"fmt"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func MatchDesiredLRPChangedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPChangedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPChangedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPChangedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(receptor.DesiredLRPChangedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPChangedEventMatcher matcher expects a receptor.DesiredLRPChangedEventMatcher.  Got:\n%s", format.Object(actual, 1))
	}

	return event.DesiredLRPResponse.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPChangedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPChangedEvent with\n  ProcessGuid=%s\n", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPChangedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPChangedEvent with\n  ProcessGuid=%s\n", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchDesiredLRPRemovedEvent(processGuid string) gomega.OmegaMatcher {
	return &DesiredLRPRemovedEventMatcher{
		ProcessGuid: processGuid,
	}
}

type DesiredLRPRemovedEventMatcher struct {
	ProcessGuid string
}

func (matcher *DesiredLRPRemovedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(receptor.DesiredLRPRemovedEvent)
	if !ok {
		return false, fmt.Errorf("DesiredLRPRemovedEventMatcher matcher expects a receptor.DesiredLRPRemovedEventMatcher.  Got:\n%s", format.Object(actual, 1))
	}
	return event.DesiredLRPResponse.ProcessGuid == matcher.ProcessGuid, nil
}

func (matcher *DesiredLRPRemovedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a DesiredLRPRemovedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

func (matcher *DesiredLRPRemovedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a DesiredLRPRemovedEvent with\n  ProcessGuid=%s", format.Object(actual, 1), matcher.ProcessGuid)
}

//

func MatchActualLRPChangedEvent(processGuid string, index int, state receptor.ActualLRPState) gomega.OmegaMatcher {
	return &ActualLRPChangedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
	}
}

type ActualLRPChangedEventMatcher struct {
	ProcessGuid   string
	Index         int
	State         receptor.ActualLRPState
	EventToReturn *receptor.ActualLRPChangedEvent
}

func (matcher *ActualLRPChangedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(receptor.ActualLRPChangedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPChangedEventMatcher matcher expects a receptor.ActualLRPChangedEventMatcher.  Got:\n%s", format.Object(actual, 1))
	}
	return event.ActualLRPResponse.ProcessGuid == matcher.ProcessGuid && event.ActualLRPResponse.Index == matcher.Index && event.ActualLRPResponse.State == matcher.State, nil
}

func (matcher *ActualLRPChangedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPChangedEvent with\n  ProcessGuid=%s\n  Index=%d\n  State=%s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
}

func (matcher *ActualLRPChangedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPChangedEvent with\n  ProcessGuid=%s\n  Index=%d\n  State=%s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
}

//

func MatchActualLRPRemovedEvent(processGuid string, index int) gomega.OmegaMatcher {
	return &ActualLRPRemovedEventMatcher{
		ProcessGuid: processGuid,
		Index:       index,
	}
}

type ActualLRPRemovedEventMatcher struct {
	ProcessGuid   string
	Index         int
	EventToReturn *receptor.ActualLRPRemovedEvent
}

func (matcher *ActualLRPRemovedEventMatcher) Match(actual interface{}) (success bool, err error) {
	event, ok := actual.(receptor.ActualLRPRemovedEvent)
	if !ok {
		return false, fmt.Errorf("ActualLRPRemovedEventMatcher matcher expects a receptor.ActualLRPRemovedEventMatcher.  Got:\n%s", format.Object(actual, 1))
	}
	return event.ActualLRPResponse.ProcessGuid == matcher.ProcessGuid && event.ActualLRPResponse.Index == matcher.Index, nil
}

func (matcher *ActualLRPRemovedEventMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto be a ActualLRPRemovedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}

func (matcher *ActualLRPRemovedEventMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to be a ActualLRPRemovedEvent with\n  ProcessGuid=%s\n  Index=%d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
}
