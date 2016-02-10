package vizzini_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Freshness", func() {
	Describe("Creating a fresh domain", func() {
		Context("with no TTL", func() {
			It("should create a fresh domain that never disappears", func() {
				Expect(bbsClient.UpsertDomain(domain, 0)).To(Succeed())
				Consistently(bbsClient.Domains, 3).Should(ContainElement(domain))
				bbsClient.UpsertDomain(domain, 1*time.Second) //to clear it out
			})
		})

		Context("with a TTL", func() {
			It("should create a fresh domain that eventually disappears", func() {
				Expect(bbsClient.UpsertDomain(domain, 2*time.Second)).To(Succeed())

				Expect(bbsClient.Domains()).To(ContainElement(domain))
				Eventually(bbsClient.Domains, 5).ShouldNot(ContainElement(domain))
			})
		})

		Context("with no domain", func() {
			It("should error", func() {
				Expect(bbsClient.UpsertDomain("", 0)).NotTo(Succeed())
			})
		})
	})
})
