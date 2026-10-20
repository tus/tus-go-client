package checksum_test

import (
	"crypto"
	"hash"

	"github.com/bdragon300/tusgo/checksum"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AlgorithmList", func() {
	data := []byte("Hello world!")

	Describe("Get", func() {
		When("pass a correct name", func() {
			DescribeTable("should return correct algorithm",
				func(name string, expectAlgo checksum.Algorithm) {
					f, ok := checksum.Algorithms.Get(name)
					Ω(ok).Should(BeTrue())
					Ω(f).ShouldNot(BeNil())
					h := f()
					h.Write(data)

					expect := checksum.Algorithms[expectAlgo]()
					expect.Write(data)

					Ω(h.Sum(nil)).Should(Equal(expect.Sum(nil)))
				},
				Entry("sha1", "sha1", checksum.SHA1),
				Entry("SHA-1", "SHA-1", checksum.SHA1),
				Entry("md_5", "md_5", checksum.MD5),
				Entry("sha 256", "sha 256", checksum.SHA256),
				Entry("Blake-2B-256", "Blake-2B-256", checksum.BLAKE2B_256),
				Entry("SHA-512/256", "SHA-512/256", checksum.SHA512_256),
			)
		})
		When("construct a supported algorithm", func() {
			DescribeTable("should construct without panic",
				func(name string) {
					f, ok := checksum.Algorithms.Get(name)
					Ω(ok).Should(BeTrue())

					var h hash.Hash
					Ω(func() { h = f() }).ShouldNot(Panic())
					Ω(h).ShouldNot(BeNil())
					Ω(h.Write(data)).Should(Equal(12))
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
					f, ok := checksum.Algorithms.Get(name)
					Ω(ok).Should(BeFalse())
					Ω(f).Should(BeNil())
				},
				Entry("unknown", "unknown"),
				Entry("sha11", "sha11"),
				Entry("empty", ""),
			)
		})
		When("called on a custom list", func() {
			It("should look up only in that list", func() {
				list := checksum.AlgorithmList{checksum.SHA1: crypto.SHA1.New}

				f, ok := list.Get("SHA-1")
				Ω(ok).Should(BeTrue())
				h := f()
				h.Write(data)

				expect := crypto.SHA1.New()
				expect.Write(data)
				Ω(h.Sum(nil)).Should(Equal(expect.Sum(nil)))

				_, ok = list.Get("md5")
				Ω(ok).Should(BeFalse())
			})
			It("should return not ok on an empty list", func() {
				f, ok := checksum.AlgorithmList{}.Get("sha1")
				Ω(ok).Should(BeFalse())
				Ω(f).Should(BeNil())

				var nilList checksum.AlgorithmList
				_, ok = nilList.Get("sha1")
				Ω(ok).Should(BeFalse())
			})
		})
	})
})
