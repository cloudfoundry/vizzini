package vizzini_test

import (
	"github.com/cloudfoundry-incubator/bbs/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cells", func() {
	It("should return all cells", func() {
		cells, err := bbsClient.Cells()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(cells)).To(BeNumerically(">=", 1))

		var cell_z1_0 *models.CellPresence
		for _, cell := range cells {
			if cell.CellId == "cell_z1-0" {
				cell_z1_0 = cell
				break
			}
		}

		Expect(cell_z1_0).NotTo(BeNil())
		Expect(cell_z1_0.CellId).To(Equal("cell_z1-0"))
		Expect(cell_z1_0.Zone).To(Equal("z1"))
		Expect(cell_z1_0.Capacity.MemoryMb).To(BeNumerically(">", 0))
		Expect(cell_z1_0.Capacity.DiskMb).To(BeNumerically(">", 0))
		Expect(cell_z1_0.Capacity.Containers).To(BeNumerically(">", 0))
	})
})
