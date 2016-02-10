package oneoffs_test

import (
	"flag"

	"github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var gardenNet, gardenAddr string

func init() {
	flag.StringVar(&gardenNet, "garden-network", "tcp", "garden's network configuration")
	flag.StringVar(&gardenAddr, "garden-address", "10.244.17.6:7777", "garden's network address ")

	flag.Parse()
}

func TestOneOffs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

func NewGuid() string {
	u, err := uuid.NewV4()
	Expect(err).NotTo(HaveOccurred())
	return u.String()
}
