package vizzini_test

import (
	. "github.com/onsi/ginkgo"

	"log"
)

var vizzini *log.Logger

var _ = BeforeEach(func() {
	vizzini = log.New(GinkgoWriter, "[Vizzini says]:", log.LstdFlags)
})
