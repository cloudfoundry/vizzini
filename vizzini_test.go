package vizzini_test

import (
	"io"
	"os"

	. "github.com/onsi/ginkgo"

	"log"
)

var vizzini *log.Logger

var _ = BeforeEach(func() {
	vizzini = log.New(io.MultiWriter(os.Stdout, GinkgoWriter), "\x1b[36m[Vizzini says]:\x1b[0m", log.LstdFlags)
})
