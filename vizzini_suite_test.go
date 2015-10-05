package vizzini_test

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/onsi/say"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
	"time"

	"github.com/cloudfoundry-incubator/bbs"
	"github.com/cloudfoundry-incubator/bbs/models"
	"github.com/cloudfoundry-incubator/consuladapter"
)

var bbsClient bbs.Client
var serviceClient bbs.ServiceClient
var domain string
var otherDomain string
var defaultRootFS string
var guid string
var startTime time.Time

var bbsAddress string
var bbsCA string
var bbsClientCert string
var bbsClientKey string
var consulAddress string
var routableDomainSuffix string
var hostAddress string
var logger lager.Logger

func init() {
	flag.StringVar(&bbsAddress, "bbs-address", "http://10.244.16.130:8889", "http address for the bbs (required)")
	flag.StringVar(&bbsCA, "bbs-ca", "", "bbs ca cert")
	flag.StringVar(&bbsClientCert, "bbs-client-cert", "", "bbs client ssl certificate")
	flag.StringVar(&bbsClientKey, "bbs-client-key", "", "bbs client ssl key")
	flag.StringVar(&consulAddress, "consul-address", "http://127.0.0.1:8500", "http address for the consul agent (required)")
	flag.StringVar(&routableDomainSuffix, "routable-domain-suffix", "bosh-lite.com", "suffix to use when constructing FQDN")
	flag.StringVar(&hostAddress, "host-address", "10.0.2.2", "address that a process running in a container on Diego can use to reach the machine running this test.  Typically the gateway on the vagrant VM.")
	flag.Parse()

	if bbsAddress == "" {
		log.Fatal("i need a bbs address to talk to Diego...")
	}

	if consulAddress == "" {
		log.Fatal("i need a consul address to talk to Diego...")
	}
}

func TestVizziniSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Vizzini Suite")
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
	otherDomain = fmt.Sprintf("vizzini-other-%d", GinkgoParallelNode())
	defaultRootFS = models.PreloadedRootFS("cflinuxfs2")

	var err error
	bbsClient = initializeBBSClient()

	consulClient, err := consuladapter.NewClient(consulAddress)
	Ω(err).ShouldNot(HaveOccurred())

	sessionMgr := consuladapter.NewSessionManager(consulClient)
	consulSession, err := consuladapter.NewSession("vizzini", 10*time.Second, consulClient, sessionMgr)
	Ω(err).ShouldNot(HaveOccurred())

	logger = lagertest.NewTestLogger("vizzini")

	serviceClient = bbs.NewServiceClient(consulSession, clock.NewClock())
})

var _ = BeforeEach(func() {
	startTime = time.Now()
	guid = NewGuid()
})

var _ = AfterEach(func() {
	defer func() {
		endTime := time.Now()
		fmt.Fprint(GinkgoWriter, say.Cyan("\n%s\nThis test referenced GUID %s\nStart time: %s (%d)\nEnd time: %s (%d)\n", CurrentGinkgoTestDescription().FullTestText, guid, startTime, startTime.Unix(), endTime, endTime.Unix()))
	}()

	for _, domain := range []string{domain, otherDomain} {
		ClearOutTasksInDomain(domain)
		ClearOutDesiredLRPsInDomain(domain)
	}
})

var _ = AfterSuite(func() {
	for _, domain := range []string{domain, otherDomain} {
		bbsClient.UpsertDomain(domain, 5*time.Minute) //leave the domain around forever so that Diego cleans up if need be
	}

	for _, domain := range []string{domain, otherDomain} {
		ClearOutDesiredLRPsInDomain(domain)
		ClearOutTasksInDomain(domain)
	}
})

func initializeBBSClient() bbs.Client {
	bbsURL, err := url.Parse(bbsAddress)
	Ω(err).ShouldNot(HaveOccurred())

	if bbsURL.Scheme != "https" {
		return bbs.NewClient(bbsAddress)
	}

	bbsClient, err := bbs.NewSecureClient(bbsAddress, bbsCA, bbsClientCert, bbsClientKey, 0, 0)
	Ω(err).ShouldNot(HaveOccurred())
	return bbsClient
}
