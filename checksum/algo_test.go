package checksum_test

import (
	"hash"

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
	When("construct a supported algorithm", func() {
		DescribeTable("should construct without panic",
			func(name string) {
				alg, ok := checksum.GetAlgorithm(name)
				Ω(ok).Should(BeTrue())

				var h hash.Hash
				Ω(func() { h = checksum.Algorithms[alg]() }).ShouldNot(Panic())
				Ω(h).ShouldNot(BeNil())
				Ω(h.Write([]byte("Hello world!"))).Should(Equal(12))
				Ω(h.Sum(nil)).Should(HaveLen(h.Size()))
			},
			Entry("md4", string(checksum.MD4)),
			Entry("md5", string(checksum.MD5)),
			Entry("sha1", string(checksum.SHA1)),
			Entry("sha224", string(checksum.SHA224)),
			Entry("sha256", string(checksum.SHA256)),
			Entry("sha384", string(checksum.SHA384)),
			Entry("sha512", string(checksum.SHA512)),
			Entry("sha512224", string(checksum.SHA512_224)),
			Entry("sha512256", string(checksum.SHA512_256)),
			Entry("sha3224", string(checksum.SHA3_224)),
			Entry("sha3256", string(checksum.SHA3_256)),
			Entry("sha3384", string(checksum.SHA3_384)),
			Entry("sha3512", string(checksum.SHA3_512)),
			Entry("ripemd160", string(checksum.RIPEMD160)),
			Entry("blake2s256", string(checksum.BLAKE2S_256)),
			Entry("blake2b256", string(checksum.BLAKE2B_256)),
			Entry("blake2b384", string(checksum.BLAKE2B_384)),
			Entry("blake2b512", string(checksum.BLAKE2B_512)),
			Entry("adler32", string(checksum.ADLER32)),
			Entry("crc32", string(checksum.CRC32)),
			Entry("crc64", string(checksum.CRC64)),
			Entry("fnv", string(checksum.FNV)),
			Entry("fnv1", string(checksum.FNV1)),
			Entry("fnv1a", string(checksum.FNV1A)),
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
})
