package acceptance_test

import (
	"flag"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var gardenNet, gardenAddr string
var receptorAddress, receptorUsername, receptorPassword string
var client receptor.Client

func init() {
	flag.StringVar(&gardenNet, "garden-network", "tcp", "http address for the receptor (required)")
	flag.StringVar(&gardenAddr, "garden-address", "10.244.17.2:7777", "receptor username")

	flag.StringVar(&receptorAddress, "receptor-address", "receptor.10.244.0.34.xip.io", "http address for the receptor (required)")
	flag.StringVar(&receptorUsername, "receptor-username", "", "receptor username")
	flag.StringVar(&receptorUsername, "receptor-password", "", "receptor password")
	flag.Parse()
}

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

func NewGuid() string {
	u, err := uuid.NewV4()
	Ω(err).ShouldNot(HaveOccurred())
	return u.String()
}

var _ = BeforeSuite(func() {
	client = receptor.NewClient(receptorAddress, receptorUsername, receptorPassword)
})
