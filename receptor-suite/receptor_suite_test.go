package receptor_suite_test

import (
	"fmt"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"time"

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
	SetDefaultEventuallyTimeout(10 * time.Second)
	domain = fmt.Sprintf("vizzini-%d", GinkgoParallelNode())
	client = receptor.NewClient("receptor.10.244.0.34.xip.io", "", "")
	stack = "lucid64"

	_, err := client.GetAllTasks()
	Ω(err).ShouldNot(HaveOccurred())
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	Ω(client.GetAllTasksByDomain(domain)).Should(BeEmpty())
	Ω(client.GetAllDesiredLRPsByDomain(domain)).Should(BeEmpty())
})
