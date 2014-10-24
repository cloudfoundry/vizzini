package receptor_suite_test

import (
	"fmt"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"

	"github.com/cloudfoundry-incubator/receptor"
)

var client receptor.Client
var domain string
var stack string

func TestReceptorSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ReceptorSuite Suite")
}

func NewGuid() string {
	u, err := uuid.NewV4()
	Ω(err).ShouldNot(HaveOccurred())
	return u.String()
}

var _ = BeforeSuite(func() {
	domain = fmt.Sprintf("vizzini-%d", GinkgoParallelNode())
	client = receptor.NewClient("10.244.17.2:8888", "", "")
	stack = "lucid64"

	_, err := client.GetAllTasks()
	Ω(err).ShouldNot(HaveOccurred())
})
