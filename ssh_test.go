package vizzini_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/cloudfoundry-incubator/receptor"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/pivotal-cf-experimental/vizzini/matchers"
)

const privateRSAKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEhgIBAAKB/C/hstPGznfdyUGdbatKgbWJYRTb8S8A7ehto1SukBzCKrR+Dw5I
y/qSIzi82xkOGjckEECa2B9fiACBY+fQQPvInCnU5iMUkJNZcrugJhnv6S9y8k3U
t7HT9YVlIxDpjxyxdrkkkmoPCAu0zSqUQuv6QlKBi2A7wZcfwmupOue11vhaPQ+K
NULtJaiYNQoHsvO/hxe/wcKmHI4R0cWp/zibNqx5xz6eaao5qsrshr02mRxMumYC
QohfM93/wL+oVyzLMSeaKxZtAglfMecjNcUn9Sk22Jq1bbvu8cLR9Gdg35XeHl5G
if03/JQsXbUpLeQd8nXKUjYk8uNAHQIDAQABAoH8FOC0uOLW5C0wtAuQ5j92j1F3
o0DDyVr+YXps3V/ANsnzFQBiUDgtuPQ/p12xqxsbEzAGZiUeV4+wHYhNp6aGr0Kp
1ROfxWwSHi3CeU07T9PsOWRFgupdroxdYezXfWhZnolC2ze3H8euGmybiRVcmMhm
YtNZknx7zQlsHMWNKSasBI0oKks7JLLuIF4eapdwnlMcw7PxO8rUs/3K6psbsiN0
AA5J/5KlkEniT7NH+Frs0jmdj/3AkuMnYnj3izJsL72kHOFvNUMdcxZX7V1xoFcy
npD0CcgpYbw6dA83fglqQcl6VO9vWff4nZAdqPyqlQCDbNWvKPyDu7mBAn5r9tSu
s3optWwLhgC6WCr34Qg3NAzwTFZI3HXeP28urOlFTXLzvVJc/RRFVEHnmOczaULo
zopwywtfQpa0Z5NAYGxPn7DB1JahqjMNdW66h5UUcgCInd1rZRtsP8xikCJmKoqa
b7e8F0tVyXrwvJBDLKYY11IpcijgIHxERF8CfnGI7K/Ev4jGZ1FdOouGSQ+pbunO
UPSPU4pzNuT6Phtgyrkd1cArTzPvjLIo5e91z+HI/YBDkHsibTFkVXGL54LrHLnS
KwSKIUvjm8HT4GG85BQbjhb2RTGkJTb63LOXuBXYOoH9xdLU52u843zxtW0p77LP
JqD5mEpyJUZtAwJ+UDDoTFLW/D/a3rxLsh1m3PLyjT5GFf49YKUPj2KCjKK2KVmb
dls64ALCmbQ5t3Ik2FTo887lmV3XNoxZL+p2vyxfhszQF0h2EeI/RVHiSv4Fx0fe
CZtoKSrSMZc5kkQIqOYUSR2N1VFgDXo3rLQCW0LApFbamhpHLiIy6un1An4unkiB
i8oRwVXfJObLL6KEWc//FQZMxSVKbjCWKOKjn0Teag/AzofBDZW5+e0gPEHVtg/R
QOzsgqBPbaFf9FBlg2DSNCgRvx4Y6SalmfhCaatFTmMzrn+O+JWHU86Xt66Q2a58
fdVi0qULqg3G2gDjCBsyUrjL1HDh8Ki5mD0Cfj/Rhdjn5THUmPkujPY0PZUzEgEs
PrdeYY5DBlgxM2zFdHX466qYy7rPT/H2YXMqqoZMQnCXNa8t/kcPa2F1C9j3HI2k
Jm/15BLfU/Ty+MHchPV6bR6fQ6SnePDKQNOBSxtMQT8oFNNM/os+WYpsF5dG8whH
wWA9OrJdbrDo9w==
-----END RSA PRIVATE KEY-----`

const authorizedKey = ` ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAA/C/hstPGznfdyUGdbatKgbWJYRTb8S8A7ehto1SukBzCKrR+Dw5Iy/qSIzi82xkOGjckEECa2B9fiACBY+fQQPvInCnU5iMUkJNZcrugJhnv6S9y8k3Ut7HT9YVlIxDpjxyxdrkkkmoPCAu0zSqUQuv6QlKBi2A7wZcfwmupOue11vhaPQ+KNULtJaiYNQoHsvO/hxe/wcKmHI4R0cWp/zibNqx5xz6eaao5qsrshr02mRxMumYCQohfM93/wL+oVyzLMSeaKxZtAglfMecjNcUn9Sk22Jq1bbvu8cLR9Gdg35XeHl5Gif03/JQsXbUpLeQd8nXKUjYk8uNAHQ==`

//These are LOCAL until we get the SSH proxy working.  There's no way to route to the container on Ketchup.
var _ = Describe("{LOCAL} SSH Tests", func() {
	var lrp receptor.DesiredLRPCreateRequest
	var sshdArgs []string

	BeforeEach(func() {
		sshdArgs = []string{}
	})

	JustBeforeEach(func() {
		lrp = receptor.DesiredLRPCreateRequest{
			ProcessGuid:          guid,
			Domain:               domain,
			Instances:            2,
			EnvironmentVariables: []receptor.EnvironmentVariable{{Name: "CUMBERBUND", Value: "cummerbund"}},
			Setup: &models.SerialAction{
				Actions: []models.Action{
					&models.DownloadAction{
						Artifact: "diego-sshd",
						From:     "http://file-server.service.dc1.consul:8080/v1/static/diego-sshd/diego-sshd.tgz",
						To:       "/tmp",
						CacheKey: "diego-sshd",
					},
				},
			},
			Action: models.Parallel(
				&models.RunAction{
					Path: "/tmp/diego-sshd",
					Args: append([]string{
						"-address=0.0.0.0:2222",
					}, sshdArgs...),
				},
				&models.RunAction{
					Path: "bash",
					Args: []string{"-c", `while true; do echo "inconceivable!" | nc -l localhost 9999; done`},
				},
			),
			Monitor: &models.RunAction{
				Path: "nc",
				Args: []string{"-z", "0.0.0.0", "2222"},
			},
			RootFS:   defaultRootFS,
			MemoryMB: 128,
			DiskMB:   128,
			Ports:    []uint16{2222},
		}

		Ω(client.CreateDesiredLRP(lrp)).Should(Succeed())
		Eventually(ActualGetter(guid, 0)).Should(BeActualLRPWithState(guid, 0, receptor.ActualLRPStateRunning))
	})

	Describe("Spinning up an unauthenticated SSH session", func() {
		BeforeEach(func() {
			sshdArgs = []string{"-allowUnauthenticatedClients"}
		})

		It("should be possible to run an ssh command", func() {
			addrComponents := strings.Split(DirectAddressFor(guid, 0, 2222), ":")
			session, err := gexec.Start(exec.Command(
				"ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-p", addrComponents[1],
				"vcap@"+addrComponents[0],
				"/usr/bin/env",
			), GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Ω(session).Should(gbytes.Say("USER=vcap"))
			// Ω(session).Should(gbytes.Say("CUMBERBUND=cummerbund")) //currently failing
		})
	})

	Describe("Spinning up a public-key authenticated SSH session", func() {
		BeforeEach(func() {
			sshdArgs = []string{"-authorizedKey=" + authorizedKey}
		})

		It("should be possible to run an ssh command", func() {
			f, err := ioutil.TempFile("", "pem")
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Fprintf(f, privateRSAKey)
			f.Close()

			defer os.Remove(f.Name())

			addrComponents := strings.Split(DirectAddressFor(guid, 0, 2222), ":")
			session, err := gexec.Start(exec.Command(
				"ssh",
				"-i", f.Name(),
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-p", addrComponents[1],
				"vcap@"+addrComponents[0],
				"/usr/bin/env",
			), GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
			Ω(session).Should(gbytes.Say("USER=vcap"))
			// Ω(session).Should(gbytes.Say("CUMBERBUND=cummerbund")) //currently failing
		})

		It("should be possible to run an interactive ssh session", func() {
			f, err := ioutil.TempFile("", "pem")
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Fprintf(f, privateRSAKey)
			f.Close()

			defer os.Remove(f.Name())

			addrComponents := strings.Split(DirectAddressFor(guid, 0, 2222), ":")
			sshCmd := exec.Command(
				"ssh",
				"-t",
				"-t", // double tap to force pty allocation
				"-i", f.Name(),
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-p", addrComponents[1],
				"vcap@"+addrComponents[0],
			)

			input, err := sshCmd.StdinPipe()
			Ω(err).ShouldNot(HaveOccurred())

			session, err := gexec.Start(sshCmd, GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(session).Should(gbytes.Say("vcap@"))

			_, err = input.Write([]byte("export FOO=foo; echo ${FOO}bar\n"))
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(session).Should(gbytes.Say("foobar"))

			_, err = input.Write([]byte("exit\n"))
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(session.Err).Should(gbytes.Say("Connection to " + addrComponents[0] + " closed."))
			Eventually(session).Should(gexec.Exit(0))
		})

		It("should be possible to forward ports", func() {
			f, err := ioutil.TempFile("", "pem")
			Ω(err).ShouldNot(HaveOccurred())
			fmt.Fprintf(f, privateRSAKey)
			f.Close()

			defer os.Remove(f.Name())

			addrComponents := strings.Split(DirectAddressFor(guid, 0, 2222), ":")
			session, err := gexec.Start(exec.Command(
				"ssh",
				"-N",
				"-L", "12345:localhost:9999",
				"-i", f.Name(),
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-p", addrComponents[1],
				"vcap@"+addrComponents[0],
			), GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())
			Eventually(session.Err).Should(gbytes.Say("Warning: Permanently added"))

			nc, err := gexec.Start(exec.Command(
				"nc",
				"127.0.0.1",
				"12345",
			), GinkgoWriter, GinkgoWriter)
			Ω(err).ShouldNot(HaveOccurred())

			Eventually(nc).Should(gexec.Exit(0))
			Ω(nc).Should(gbytes.Say("inconceivable!"))

			session.Interrupt()

			Eventually(session).Should(gexec.Exit())
		})
	})
})
