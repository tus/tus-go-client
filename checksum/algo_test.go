package checksum_test

import (
	"github.com/bdragon300/tusgo/checksum"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetAlgorithm", func() {
	When("pass a correct name", func() {
		DescribeTable("should return correct algorithm",
			func(name string, expect checksum.Algorithm) {
				alg, ok := checksum.GetAlgorithm(name)
				Ω(ok).Should(BeTrue())
				Ω(alg).Should(Equal(expect))
				Ω(alg).Should(BeKeyOf(checksum.Algorithms))
			},
			Entry("sha1", "sha1", checksum.SHA1),
			Entry("SHA-1", "SHA-1", checksum.SHA1),
			Entry("md_5", "md_5", checksum.MD5),
			Entry("Blake-2B-256", "Blake-2B-256", checksum.BLAKE2B_256),
		)
	})
	When("pass an unknown name", func() {
		DescribeTable("should return not ok",
			func(name string) {
				_, ok := checksum.GetAlgorithm(name)
				Ω(ok).Should(BeFalse())
			},
			Entry("unknown", "unknown"),
			Entry("sha11", "sha11"),
		)
	})
	When("constructing hashes that live in x/crypto", func() {
		DescribeTable("should not panic",
			func(name string) {
				alg, ok := checksum.GetAlgorithm(name)
				Ω(ok).Should(BeTrue())
				Ω(func() { _ = checksum.Algorithms[alg]() }).ShouldNot(Panic())
				h := checksum.Algorithms[alg]()
				Ω(h).ShouldNot(BeNil())
				_, err := h.Write([]byte("x"))
				Ω(err).Should(Succeed())
				Ω(h.Sum(nil)).ShouldNot(BeEmpty())
			},
			Entry("blake2b256", "blake2b256"),
			Entry("blake2b384", "blake2b384"),
			Entry("blake2b512", "blake2b512"),
			Entry("blake2s256", "blake2s256"),
			Entry("md4", "md4"),
			Entry("ripemd160", "ripemd160"),
			Entry("sha3256", "sha3256"),
		)
	})
})
