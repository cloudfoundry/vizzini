package blackbox

import (
	"log"

	"github.com/cloudfoundry-incubator/veritas/say"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/internal/codelocation"
	. "github.com/onsi/gomega"

	"testing"
)

var vizzini *log.Logger

func TestBlackbox(t *testing.T) {
	RegisterFailHandler(func(message string, callerSkip ...int) {
		skip := 0
		if len(callerSkip) > 0 {
			skip = callerSkip[0]
		}
		cl := codelocation.New(skip + 1)

		vizzini.Printf("%s\n%s\n%s", say.Red("FAILURE"), say.Red(cl.String()), say.Red(message))
	})
	RunSpecs(t, "Blackbox Suite")
}

var _ = BeforeEach(func() {
	vizzini = log.New(GinkgoWriter, "\x1b[36m[Vizzini says]:\x1b[0m", log.LstdFlags)
})
