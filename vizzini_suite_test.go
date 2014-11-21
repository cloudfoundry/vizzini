package vizzini_test

import (
	"flag"
	"fmt"
	"log"

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
var rootFS string

var receptorAddress string

func init() {
	var onEdge bool
	flag.StringVar(&receptorAddress, "receptor-address", "http://receptor.10.244.0.34.xip.io", "http address for the receptor (required)")
	flag.BoolVar(&onEdge, "edge", false, "if true, will use a docker-image based rootfs for Diego-Edge")
	flag.Parse()

	if onEdge {
		rootFS = "docker:///cloudfoundry/lucid64"
	}

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
	return u.String()
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyTimeout(10 * time.Second)
	domain = fmt.Sprintf("vizzini-%d", GinkgoParallelNode())
	stack = "lucid64"

	client = receptor.NewClient(receptorAddress)
})

var _ = AfterSuite(func() {
	Ω(client.TasksByDomain(domain)).Should(BeEmpty())
	Ω(client.DesiredLRPsByDomain(domain)).Should(BeEmpty())
})
