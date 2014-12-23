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
				Ω(client.UpsertDomain(domain, 0)).Should(Succeed())
				Consistently(client.Domains, 3).Should(ContainElement(domain))
			})
		})

		Context("with a TTL", func() {
			It("should create a fresh domain that eventually disappears", func() {
				Ω(client.UpsertDomain(domain, 2*time.Second)).Should(Succeed())

				Ω(client.Domains()).Should(ContainElement(domain))
				Eventually(client.Domains, 5).ShouldNot(ContainElement(domain))
			})
		})

		Context("with no domain", func() {
			It("should error", func() {
				Ω(client.UpsertDomain("", 0)).ShouldNot(Succeed())
			})
		})
	})
})
