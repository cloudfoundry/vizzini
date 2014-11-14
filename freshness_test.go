package vizzini_test

import (
	"time"

	"github.com/cloudfoundry-incubator/receptor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Freshness", func() {
	Describe("Creating a fresh domain", func() {
		Context("with no TTL", func() {
			It("should create a fresh domain that never disappears", func() {
				Ω(client.BumpFreshDomain(receptor.FreshDomainBumpRequest{
					Domain:       domain,
					TTLInSeconds: 0,
				})).Should(Succeed())

				Consistently(client.FreshDomains, 3).Should(ContainElement(receptor.FreshDomainResponse{
					Domain:       domain,
					TTLInSeconds: 0,
				}))
			})
		})

		Context("with a TTL", func() {
			It("should create a fresh domain that eventually disappears", func() {
				Ω(client.BumpFreshDomain(receptor.FreshDomainBumpRequest{
					Domain:       domain,
					TTLInSeconds: 2,
				})).Should(Succeed())

				domains, err := client.FreshDomains()
				Ω(err).ShouldNot(HaveOccurred())

				var domainInQuestion receptor.FreshDomainResponse
				for _, freshDomain := range domains {
					if freshDomain.Domain == domain {
						domainInQuestion = freshDomain
					}
				}

				Ω(domainInQuestion).ShouldNot(BeZero())
				Ω(domainInQuestion.TTLInSeconds).Should(BeNumerically("<=", 2))

				time.Sleep(3 * time.Second)

				domains, err = client.FreshDomains()
				Ω(err).ShouldNot(HaveOccurred())

				for _, freshDomain := range domains {
					if freshDomain.Domain == domain {
						Fail("Did not expect to see the domain!")
					}
				}
			})
		})

		Context("with no domain", func() {
			It("should error", func() {
				Ω(client.BumpFreshDomain(receptor.FreshDomainBumpRequest{
					TTLInSeconds: 0,
				})).ShouldNot(Succeed())
			})
		})
	})
})
