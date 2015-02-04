package vizzini_test

import (
	"flag"
	"fmt"
	"log"
	"os"

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
var guid string

var receptorAddress string
var routableDomainSuffix string
var hostAddress string

func init() {
	flag.StringVar(&receptorAddress, "receptor-address", "http://receptor.10.244.0.34.xip.io", "http address for the receptor (required)")
	flag.StringVar(&routableDomainSuffix, "routable-domain-suffix", "10.244.0.34.xip.io", "suffix to use when constructing FQDN")
	flag.StringVar(&hostAddress, "host-address", "10.0.2.2", "address that a process running in a container on Diego can use to reach the machine running this test.  Typically the gateway on the vagrant VM.")
	flag.Parse()

	if receptorAddress == "" {
		log.Fatal("i need a receptor-address to talk to Diego...")
	}
}

func TestReceptorSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ReceptorSuite Suite")
}

func NewGuid() string {
	u, err := uuid.NewV4()
	Ω(err).ShouldNot(HaveOccurred())
	return domain + "-" + u.String()[:8]
}

var _ = BeforeSuite(func() {
	timeout := os.Getenv("DEFAULT_EVENTUALLY_TIMEOUT")
	if timeout == "" {
		SetDefaultEventuallyTimeout(10 * time.Second)
	} else {
		duration, err := time.ParseDuration(timeout)
		Ω(err).ShouldNot(HaveOccurred(), "invalid timeout")
		fmt.Printf("Setting Default Eventually Timeout to %s\n", duration)
		SetDefaultEventuallyTimeout(duration)
	}
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)
	SetDefaultConsistentlyPollingInterval(200 * time.Millisecond)
	domain = fmt.Sprintf("vizzini-%d", GinkgoParallelNode())
	stack = "lucid64"

	client = receptor.NewClient(receptorAddress)
})

var _ = BeforeEach(func() {
	guid = NewGuid()
})

var _ = AfterEach(func() {
	ClearOutTasksInDomain(domain)
	ClearOutDesiredLRPsInDomain(domain)
})

var _ = AfterSuite(func() {
	ClearOutDesiredLRPsInDomain(domain)
	ClearOutTasksInDomain(domain)
	client.UpsertDomain(domain, 1*time.Second) //clear out the domain

	Ω(client.TasksByDomain(domain)).Should(BeEmpty())
	Ω(client.DesiredLRPsByDomain(domain)).Should(BeEmpty())
	Eventually(ActualByDomainGetter(domain)).Should(BeEmpty())
})
