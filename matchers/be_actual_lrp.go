package matchers

import (
	"fmt"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func BeActualLRP(processGuid string, index int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
	}
}

func BeActualLRPWithState(processGuid string, index int, state receptor.ActualLRPState) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
	}
}

type BeActualLRPMatcher struct {
	ProcessGuid string
	Index       int
	State       receptor.ActualLRPState
}

func (matcher *BeActualLRPMatcher) Match(actual interface{}) (success bool, err error) {
	lrp, ok := actual.(receptor.ActualLRPResponse)
	if !ok {
		return false, fmt.Errorf("BeActualLRP matcher expects a receptor.ActualLRPResponse.  Got:\n%s", format.Object(actual, 1))
	}

	matchesState := true
	if matcher.State != "" {
		matchesState = matcher.State == lrp.State
	}

	return matchesState && lrp.ProcessGuid == matcher.ProcessGuid && lrp.Index == matcher.Index, nil
}

func (matcher *BeActualLRPMatcher) FailureMessage(actual interface{}) (message string) {
	if matcher.State == "" {
		return fmt.Sprintf("Expected\n%s\nto have ProcessGuid %s and Index %d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
	} else {
		return fmt.Sprintf("Expected\n%s\nto have ProcessGuid %s, Index %d and State %s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
	}
}

func (matcher *BeActualLRPMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	if matcher.State == "" {
		return fmt.Sprintf("Expected\n%s\nnot to have ProcessGuid %s and Index %d", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index)
	} else {
		return fmt.Sprintf("Expected\n%s\nnot to have ProcessGuid %s, Index %d and State %s", format.Object(actual, 1), matcher.ProcessGuid, matcher.Index, matcher.State)
	}
}
