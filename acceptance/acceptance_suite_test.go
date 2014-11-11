package acceptance_test

import (
	"flag"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var gardenNet, gardenAddr string

func init() {
	flag.StringVar(&gardenNet, "garden-network", "tcp", "http address for the receptor (required)")
	flag.StringVar(&gardenAddr, "garden-address", "10.244.17.2:7777", "receptor username")
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
