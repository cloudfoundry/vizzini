package vizzini_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"syscall"
	"time"

	"github.com/kr/pty"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/cloudfoundry-incubator/bbs/models"
	ssh_routes "github.com/cloudfoundry-incubator/diego-ssh/routes"

	. "github.com/cloudfoundry-incubator/vizzini/matchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const userPrivateRSAKey = `-----BEGIN RSA PRIVATE KEY-----
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

const userAuthorizedKey = ` ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAA/C/hstPGznfdyUGdbatKgbWJYRTb8S8A7ehto1SukBzCKrR+Dw5Iy/qSIzi82xkOGjckEECa2B9fiACBY+fQQPvInCnU5iMUkJNZcrugJhnv6S9y8k3Ut7HT9YVlIxDpjxyxdrkkkmoPCAu0zSqUQuv6QlKBi2A7wZcfwmupOue11vhaPQ+KNULtJaiYNQoHsvO/hxe/wcKmHI4R0cWp/zibNqx5xz6eaao5qsrshr02mRxMumYCQohfM93/wL+oVyzLMSeaKxZtAglfMecjNcUn9Sk22Jq1bbvu8cLR9Gdg35XeHl5Gif03/JQsXbUpLeQd8nXKUjYk8uNAHQ==`

const hostPrivateRSAKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA2/Qt7gMd20+O+sOKxSCk+7AvFxJAqFNfnrB9nlAU1986UR3R
euk5xqTvJokv3yCbzTPP07cQFHenalBrg2/sjCdq9MWKi8vfIBW3malquLr0fBRx
qboukme2rgNXH0HCALxLinjBLPK5ToRMB25FVixkYo8nEvUbfiYH5GIRawXdRo7L
XXIK/Htx40zUPpIu9juB5K+RFovI+MIv6U3QqxNYOUhfbPhaGVV0FdFLO+uQNdLy
/ov7tI1mJVKTvpPR+Eea6v2rL75KdOfJ5prGVsAPrDA6gcvYpuJqaSi3OJe5sEhq
yQs9qQoA60tGQfX0bmRdEhWDiueCXW5/WoU/2wIDAQABAoIBACReF0oHUeR1Hxrv
Qf6eCyliVCboabBrOKAwZlTKwOeAjU/kMkK0VU028CPbAwNNjPU839wNpKb9sbyu
V1iAJQh3bAPUtbevmdDgRl8t1+t7Xfk2GCUMF681Xsse2kTcxosAlyzqEmawK1uE
HF4OKYC6Dk8NhFRqGoWdHCjy3hZnrzZsaNUUlMMCRTkyENCXR67hLT5TQfqEpbHP
5BVSJe3nmvAUPN7Q3Nk98aRAzC1NLv7XGKCvLNFF7L4Kvq3BfoXVOjekvyF/Ssbn
ciRpPRgxCJP3vlkWGFVbtnnZQpQDSc+LIbAxZ4BI5MMcsgX6YyyBbUzRbtub7JVQ
rW18GWECgYEA+95Ye4lsc5Tp+WK9JvxvGUAjgrtO4t8/v6129NnYD1miZCjwb8Sn
Ewk97cNfkWmTnCtsL5V18YXkpdgj3lF9cfnAFdJc3IX9xzpbrykEuJV1kxCCU61P
9LsehAOpNTDAwfM+wX92W6iNnYd+jBAiEPExP2qkKetnHfNVb8a5FB8CgYEA34/R
Bo4/Yb0E2Xi8C1FBOUlmDPhqN8QKKjdpVNSIrD5Q5eV1xpKKpZuB9TaCWQEWvcwT
nbXTGTJ4CUGSMzGMZg7iPREXt41YRdiv/VrBA/zevSC6OfhiJx+Mz32aDWNFfUtV
CwQlSSfiC5lw8PD9uY6q/lJUEBPcDXIBj2JuvMUCgYEAt4mONu+sjQlN+sIeDmPT
XbYkamauFJsUrEvurHx2erEZqh0/IGNQUInii/lcEe26eAoYexBR8x9bwBKiCKaf
YEfb1ssFillF1kFLgHfGje+zzugv4GQiKLeWhCLa0fzl6i+kYoLMr/xCvjF3YP98
o5XvCkRevoFhEi047AwG4IcCgYBz1OwUXXdxiKIOm4OyyXLl36XEaqF+K1Co9vTY
QxZdSBxaQT14mUzE6YG4L3nx66KAzFANkrvBfmi7QwIhDDcWWffWdBi5vb5S0ia9
OlxvWIF/tIlIp+0TIEGw7/71mM3UUUfK4WcANG3mXKYr8HFFxynJg5aSjfeh78Pn
KrT9kQKBgQCy9UfV4Kku8Zk6FeqWZvZP+wYarG9BMvc7C4mT+6bNMCeCmMydueZs
u6FLDjvicUuG1MZywSCoOpI6MZkcZiFXwdgIdRFdhDDcWdewItsJXBmHMjzr+t8P
hqA2YFwsUWCcgAxICpYQyTFVYBnHUVPYAHzctmWRbQuXhMJgWIRNhw==
-----END RSA PRIVATE KEY-----`

const hostFingerprint = `9e:33:35:e0:fe:67:e5:c4:7e:90:53:72:c2:3f:a1:9c`

type sshTarget struct {
	User string
	Host string
	Port string
}

var _ = Describe("SSH Tests", func() {
	var (
		password      string
		target        sshTarget
		lrp           *models.DesiredLRP
		user          string
		rootfs        string
		startTimeout  time.Duration
		sshdArgs      []string
		sshClientArgs []string
		shellServer   models.RunAction
		sshdMonitor   models.RunAction
	)

	secureCommand := func(cmd string, args ...string) *exec.Cmd {
		sshArgs := []string{}
		sshArgs = append(sshArgs, sshClientArgs...)
		sshArgs = append(sshArgs, args...)

		return exec.Command(cmd, sshArgs...)
	}

	ssh := func(target sshTarget, args ...string) *exec.Cmd {
		sshArgs := []string{
			"-o", "User=" + target.User,
			"-p", target.Port,
			target.Host,
		}
		return secureCommand("ssh", append(sshArgs, args...)...)
	}

	sshInteractive := func(target sshTarget) *exec.Cmd {
		return ssh(target,
			"-t",
			"-t", // double tap to force pty allocation
		)
	}

	sshTunnelTo := func(target sshTarget, localport, remoteport int) *exec.Cmd {
		return ssh(target,
			"-N",
			"-L", fmt.Sprintf("%d:127.0.0.1:%d", localport, remoteport),
		)
	}

	scp := func(target sshTarget, args ...string) *exec.Cmd {
		sshArgs := []string{
			"-o", "User=" + target.User,
			"-P", target.Port,
		}
		return secureCommand("scp", append(sshArgs, args...)...)
	}

	BeforeEach(func() {
		password = sshPassword
		target = sshTarget{
			User: "diego:" + guid + "/0",
			Port: sshPort,
			Host: sshHost,
		}

		user = "vcap"
		startTimeout = timeout
		sshdArgs = append(sshdArgs,
			"-hostKey="+hostPrivateRSAKey,
			"-authorizedKey="+userAuthorizedKey,
			"-inheritDaemonEnv",
		)
		sshClientArgs = []string{
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
		}
	})

	JustBeforeEach(func() {
		sshRoutePayload, err := json.Marshal(ssh_routes.SSHRoute{
			ContainerPort:   2222,
			HostFingerprint: hostFingerprint,
			PrivateKey:      userPrivateRSAKey,
		})
		Ω(err).ShouldNot(HaveOccurred())

		sshRouteJSON := json.RawMessage(sshRoutePayload)
		routes := models.Routes{
			ssh_routes.DIEGO_SSH: &sshRouteJSON,
		}

		lrp = &models.DesiredLRP{
			ProcessGuid:          guid,
			Domain:               domain,
			Instances:            1,
			EnvironmentVariables: []*models.EnvironmentVariable{{Name: "CUMBERBUND", Value: "cummerbund"}},
			Setup: models.WrapAction(&models.DownloadAction{
				Artifact: "lifecycle bundle",
				From:     "http://file-server.service.cf.internal:8080/v1/static/buildpack_app_lifecycle/buildpack_app_lifecycle.tgz",
				To:       "/tmp",
				CacheKey: "lifecycle",
				User:     user,
			}),
			Action: models.WrapAction(models.Parallel(
				&models.RunAction{
					Path: "/tmp/diego-sshd",
					Args: append([]string{
						"-address=0.0.0.0:2222",
						"-logLevel=debug",
					}, sshdArgs...),
					User: user,
				},
				&shellServer,
			)),
			Monitor:  models.WrapAction(&sshdMonitor),
			RootFs:   rootfs,
			MemoryMb: 128,
			DiskMb:   128,
			Ports:    []uint32{2222},
			Routes:   &routes,
		}

		Ω(bbsClient.DesireLRP(lrp)).Should(Succeed())
		Eventually(ActualGetter(guid, 0), startTimeout).Should(BeActualLRPWithState(guid, 0, models.ActualLRPStateRunning))
	})

	Context("in a fully-featured preloaded rootfs", func() {
		BeforeEach(func() {
			user = "vcap"
			rootfs = defaultRootFS
			shellServer = models.RunAction{
				Path: "bash",
				Args: []string{"-c", `while true; do echo "inconceivable!" | nc -l localhost 9999; done`},
				User: user,
			}
			sshdMonitor = models.RunAction{
				Path: "nc",
				Args: []string{"-z", "0.0.0.0", "2222"},
				User: user,
			}
		})

		It("runs an ssh command", func() {
			cmd := ssh(target, "/usr/bin/env")

			session := runWithPassword(cmd, password)

			Eventually(session).Should(gexec.Exit(0))
			Ω(session).Should(gbytes.Say("USER=" + user))
			Ω(session).Should(gbytes.Say("CUMBERBUND=cummerbund"))
		})

		It("runs an interactive ssh session", func() {
			cmd := sshInteractive(target)

			session := runInteractiveWithPassword(cmd, password, func(session *gexec.Session, input *os.File) {
				Eventually(session).Should(gbytes.Say(user + "@"))

				_, err := input.Write([]byte("export FOO=foo; echo ${FOO}bar\n"))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(session).Should(gbytes.Say("foobar"))

				_, err = input.Write([]byte("exit\n"))
				Ω(err).ShouldNot(HaveOccurred())

				Eventually(session.Err).Should(gbytes.Say("Connection to " + target.Host + " closed."))
			})

			Eventually(session).Should(gexec.Exit(0))
		})

		It("forwards ports", func() {
			cmd := sshTunnelTo(target, 12345, 9999)

			session := runInteractiveWithPassword(cmd, password, func(session *gexec.Session, _ *os.File) {
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
			})

			Eventually(session).Should(gexec.Exit())
		})

		It("copies files back and forth", func() {
			dir, err := ioutil.TempDir("", "vizzini-ssh")
			Ω(err).ShouldNot(HaveOccurred())

			defer os.RemoveAll(dir)

			inpath := path.Join(dir, "inbound")
			infile, err := os.Create(inpath)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = infile.Write([]byte("hello from vizzini"))
			Ω(err).ShouldNot(HaveOccurred())

			err = infile.Close()
			Ω(err).ShouldNot(HaveOccurred())

			insession := runWithPassword(scp(target,
				inpath,
				target.Host+":in-container",
			), password)

			Eventually(insession).Should(gexec.Exit())

			outpath := path.Join(dir, "outbound")
			outsession := runWithPassword(scp(target,
				target.Host+":in-container",
				outpath,
			), password)

			Eventually(outsession).Should(gexec.Exit())

			contents, err := ioutil.ReadFile(outpath)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(contents).Should(Equal([]byte("hello from vizzini")))
		})
	})

	Context("{DOCKER} in a bare-bones docker image with /bin/sh", func() {
		BeforeEach(func() {
			user = "root"
			rootfs = "docker:///busybox"
			startTimeout = dockerTimeout
			shellServer = models.RunAction{
				Path: "sh",
				Args: []string{"-c", `while true; do echo "inconceivable!" | nc -l 127.0.0.1 -p 9999; done`},
				User: user,
			}
			sshdMonitor = models.RunAction{
				Path: "sh",
				Args: []string{
					"-c",
					"echo -n '' | telnet localhost 2222 >/dev/null 2>&1 && true",
				},
				User: user,
			}
		})

		It("runs an ssh command", func() {
			cmd := ssh(target, "/bin/env")

			session := runWithPassword(cmd, password)

			Eventually(session).Should(gexec.Exit(0))
			Ω(session).Should(gbytes.Say("USER=" + user))
			Ω(session).Should(gbytes.Say("CUMBERBUND=cummerbund"))
		})

		It("forwards ports", func() {
			cmd := sshTunnelTo(target, 12345, 9999)

			session := runInteractiveWithPassword(cmd, password, func(session *gexec.Session, _ *os.File) {
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
			})

			Eventually(session).Should(gexec.Exit())
		})
	})
})

func runWithPassword(cmd *exec.Cmd, password string) *gexec.Session {
	return runInteractiveWithPassword(cmd, password, func(session *gexec.Session, _ *os.File) {
		Eventually(session).Should(gexec.Exit())
	})
}

func runInteractiveWithPassword(cmd *exec.Cmd, password string, actions func(*gexec.Session, *os.File)) *gexec.Session {
	passwordInput := password + "\n"

	ptyMaster, ptySlave, err := pty.Open()
	Ω(err).ShouldNot(HaveOccurred())
	defer ptyMaster.Close()

	cmd.Stdin = ptySlave
	cmd.Stdout = ptySlave
	cmd.Stderr = ptySlave

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}

	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Ω(err).ShouldNot(HaveOccurred())

	// Close our open reference to ptySlave so that PTY Master recieves EOF
	ptySlave.Close()

	sendPassword(ptyMaster, passwordInput)

	done := make(chan struct{})
	go func() {
		io.Copy(GinkgoWriter, ptyMaster)
		close(done)
	}()

	actions(session, ptyMaster)
	Eventually(done).Should(BeClosed())
	return session
}

func sendPassword(pty *os.File, password string) {
	passwordPrompt := []byte("password: ")

	b := make([]byte, 1)
	buf := []byte{}
	done := make(chan struct{})

	go func() {
		defer GinkgoRecover()
		for {
			n, err := pty.Read(b)
			Expect(n).To(Equal(1))
			Expect(err).NotTo(HaveOccurred())
			buf = append(buf, b[0])
			if bytes.HasSuffix(buf, passwordPrompt) {
				break
			}
		}
		n, err := pty.Write([]byte(password))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(password)))

		close(done)
	}()

	Eventually(done).Should(BeClosed())
}
