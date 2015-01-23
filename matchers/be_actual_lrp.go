package matchers

import (
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func BeActualLRP(processGuid string, index int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		CrashCount:  -1,
	}
}

func BeUnclaimedActualLRPWithPlacementError(processGuid string, index int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid:       processGuid,
		Index:             index,
		CrashCount:        -1,
		State:             receptor.ActualLRPStateUnclaimed,
		HasPlacementError: true,
	}
}

func BeActualLRPWithState(processGuid string, index int, state receptor.ActualLRPState) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
		CrashCount:  -1,
	}
}

func BeActualLRPWithCrashCount(processGuid string, index int, crashCount int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		CrashCount:  crashCount,
	}
}

func BeActualLRPWithStateAndCrashCount(processGuid string, index int, state receptor.ActualLRPState, crashCount int) gomega.OmegaMatcher {
	return &BeActualLRPMatcher{
		ProcessGuid: processGuid,
		Index:       index,
		State:       state,
		CrashCount:  crashCount,
	}
}

type BeActualLRPMatcher struct {
	ProcessGuid       string
	Index             int
	State             receptor.ActualLRPState
	CrashCount        int
	HasPlacementError bool
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
	matchesCrashCount := true
	if matcher.CrashCount != -1 {
		matchesCrashCount = matcher.CrashCount == lrp.CrashCount
	}
	matchesPlacementErrorRequirement := true
	if matcher.HasPlacementError {
		matchesPlacementErrorRequirement = lrp.PlacementError != ""
	}

	return matchesPlacementErrorRequirement && matchesState && matchesCrashCount && lrp.ProcessGuid == matcher.ProcessGuid && lrp.Index == matcher.Index, nil
}

func (matcher *BeActualLRPMatcher) expectedContents() string {
	expectedContents := []string{
		fmt.Sprintf("ProcessGuid: %s", matcher.ProcessGuid),
		fmt.Sprintf("Index: %d", matcher.Index),
	}
	if matcher.State != "" {
		expectedContents = append(expectedContents, fmt.Sprintf("State: %s", matcher.State))
	}
	if matcher.CrashCount != -1 {
		expectedContents = append(expectedContents, fmt.Sprintf("CrashCount: %d", matcher.CrashCount))
	}
	if matcher.HasPlacementError {
		expectedContents = append(expectedContents, fmt.Sprintf("PlacementError Exists"))
	}

	return strings.Join(expectedContents, "\n")
}

func (matcher *BeActualLRPMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nto have:\n%s", format.Object(actual, 1), matcher.expectedContents())
}

func (matcher *BeActualLRPMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n%s\nnot to have:\n%s", format.Object(actual, 1), matcher.expectedContents())
}
