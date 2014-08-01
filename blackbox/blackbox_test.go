package blackbox

import (
	"io/ioutil"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/veritas/say"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var t time.Time

func graceIndices() map[string]int {
	counts := map[string]int{}

	for i := 0; i < 100; i++ {
		response, err := http.Get("http://grace.10.244.0.34.xip.io/index")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusOK))

		if err != nil {
			continue
		}

		if response.StatusCode == http.StatusOK {
			indexStr, _ := ioutil.ReadAll(response.Body)
			counts[string(indexStr)] += 1
		}

		response.Body.Close()
	}

	return counts
}

func stats() {
	session, err := gexec.Start(exec.Command("veritas", "dump-store"), nil, nil)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(session, time.Minute).Should(gexec.Exit(0))
	bbs := session.Out.Contents()

	session, err = gexec.Start(exec.Command("veritas", "vitals"), nil, nil)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(session, time.Minute).Should(gexec.Exit(0))
	vitals := session.Out.Contents()

	session, err = gexec.Start(exec.Command("veritas", "executor-resources"), nil, nil)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))
	resources := session.Out.Contents()

	session, err = gexec.Start(exec.Command("veritas", "executor-containers"), nil, nil)
	Ω(err).ShouldNot(HaveOccurred())
	Eventually(session).Should(gexec.Exit(0))
	containers := session.Out.Contents()

	say.Fprintln(GinkgoWriter, 0, ">>>>>>>>>>")
	say.Fprintln(GinkgoWriter, 0, "Stats: %s", time.Since(t))
	say.Fprintln(GinkgoWriter, 1, say.Green("BBS"))
	say.Fprintln(GinkgoWriter, 2, string(bbs))
	say.Fprintln(GinkgoWriter, 1, say.Green("Vitals"))
	say.Fprintln(GinkgoWriter, 2, string(vitals))
	say.Fprintln(GinkgoWriter, 1, say.Green("Executor Resources"))
	say.Fprintln(GinkgoWriter, 2, string(resources))
	say.Fprintln(GinkgoWriter, 1, say.Green("Executor Containers"))
	say.Fprintln(GinkgoWriter, 2, string(containers))
	say.Fprintln(GinkgoWriter, 0, "<<<<<<<<<<")
}

func withStats(f func()) {
	statsChan := make(chan bool)
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		stats()
		for {
			select {
			case <-ticker.C:
				stats()
			case <-statsChan:
				wg.Done()
				return
			}
		}
	}()

	f()

	close(statsChan)
	wg.Wait()
	stats()
}

var _ = Describe("Blackbox", func() {
	It("should excercise CF", func() {
		count := 0
		graceDir := "/Users/pivotal/go/src/github.com/onsi/grace"

		t = time.Now()

		vizzini.Println("Pushing the first grace")
		session := CF(graceDir, "push", "grace-1", "--no-start", "-c=./bin/grace", "-b=go_buildpack", "-i=4")
		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
		session = CF(graceDir, "set-env", "grace-1", "CF_DIEGO_BETA", "true")
		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		session = CF(graceDir, "set-env", "grace-1", "CF_DIEGO_RUN_BETA", "true")
		Eventually(session, 10*time.Second).Should(gexec.Exit(0))
		session = CF(graceDir, "start", "grace-1")

		Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
		response, err := http.Get("http://grace-1.10.244.0.34.xip.io/ping")
		Ω(err).ShouldNot(HaveOccurred())
		Ω(response.StatusCode).Should(Equal(http.StatusOK))

		for {
			vizzini.Printf("<<<<<<<<<<<<<<<<<<<<<<<<<<<\n")
			vizzini.Printf("\n\n>>>> Pushing grace #%d <<<<\n\n", count)
			//push then delete an app
			session := CF(graceDir, "push", "grace", "--no-start", "-c=./bin/grace", "-b=go_buildpack")
			Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
			session = CF(graceDir, "set-env", "grace", "CF_DIEGO_BETA", "true")
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			session = CF(graceDir, "set-env", "grace", "CF_DIEGO_RUN_BETA", "true")
			Eventually(session, 10*time.Second).Should(gexec.Exit(0))
			session = CF(graceDir, "start", "grace")

			withStats(func() {
				Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
				response, err := http.Get("http://grace.10.244.0.34.xip.io/ping")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			vizzini.Printf("\n\n >>>> Grace %d is up! Scaling up... <<<< \n\n", count)

			session = CF(graceDir, "scale", "grace", "-i", "10")

			withStats(func() {
				Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
				Eventually(graceIndices, time.Minute).Should(HaveLen(10))
			})

			vizzini.Printf("\n\n >>>> Grace %d is at 10! Scaling down... <<<< \n\n", count)

			session = CF(graceDir, "scale", "grace", "-i", "1")

			withStats(func() {
				Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
				Eventually(graceIndices, time.Minute).Should(HaveLen(1))
			})

			vizzini.Printf("\n\n >>>> Grace %d back to 1! Shutting down... <<<< \n\n", count)

			session = CF(graceDir, "delete", "grace", "-f")
			withStats(func() {
				Eventually(session, 5*time.Minute).Should(gexec.Exit(0))
				response, err = http.Get("http://grace.10.244.0.34.xip.io/index")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(response.StatusCode).ShouldNot(Equal(http.StatusOK))
			})
			vizzini.Printf("\n\n >>>> Grace %d is down. <<<< \n\n", count)

			count++
		}
	})
})
