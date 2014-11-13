package vizzini_test

import (
	"github.com/cloudfoundry-incubator/receptor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cells", func() {
	It("should return all cells", func() {
		cells, err := client.Cells()
		Ω(err).ShouldNot(HaveOccurred())
		Ω(len(cells)).Should(BeNumerically(">=", 1))
		Ω(cells).Should(ContainElement(receptor.CellResponse{
			CellID: "cell_z1-0",
			Stack:  "lucid64",
		}))
	})
})
