package rufh_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bdragon300/tusgo/v2/rufh"
)

func ptr(v int64) *int64 { return &v }

var _ = Describe("Limits", func() {
	Context("UnmarshalText", func() {
		DescribeTable("should deserialize dictionary into fields",
			func(text string, expected rufh.Limits) {
				var res rufh.Limits
				Expect(res.UnmarshalText([]byte(text))).Should(Succeed())
				Expect(res).Should(Equal(expected))
			},
			Entry("empty string", "", rufh.Limits{}),
			Entry("all fields",
				"max-size=1000, min-size=10, max-append-size=500, min-append-size=5, max-age=3600",
				rufh.Limits{
					MaxSize:       ptr(1000),
					MinSize:       10,
					MaxAppendSize: ptr(500),
					MinAppendSize: 5,
					MaxAge:        time.Hour,
				},
			),
			Entry("zero pointer fields are set", "max-size=0, max-append-size=0",
				rufh.Limits{MaxSize: ptr(0), MaxAppendSize: ptr(0)}),
			Entry("no whitespace and extra whitespace", "max-size=2,min-size=1 ,   max-age=3",
				rufh.Limits{MaxSize: ptr(2), MinSize: 1, MaxAge: 3 * time.Second}),
			Entry("unknown fields are ignored", `foo="bar", max-size=2, baz, qux=?1, quux=(1 2), min-size=1`,
				rufh.Limits{MaxSize: ptr(2), MinSize: 1}),
			Entry("parameters are ignored", "max-size=1;a=2;b, max-age=5;x",
				rufh.Limits{MaxSize: ptr(1), MaxAge: 5 * time.Second}),
			Entry("last duplicate key wins", "max-size=1, max-size=2",
				rufh.Limits{MaxSize: ptr(2)}),
		)

		It("should reset fields missing in the dictionary", func() {
			res := rufh.Limits{MaxSize: ptr(1), MinSize: 2, MaxAppendSize: ptr(3), MinAppendSize: 4, MaxAge: time.Second}
			Expect(res.UnmarshalText([]byte("min-size=10"))).Should(Succeed())
			Expect(res).Should(Equal(rufh.Limits{MinSize: 10}))
		})

		It("should not share pointers between fields", func() {
			var res rufh.Limits
			Expect(res.UnmarshalText([]byte("max-size=1, max-append-size=2"))).Should(Succeed())
			Expect(*res.MaxSize).Should(Equal(int64(1)))
			Expect(*res.MaxAppendSize).Should(Equal(int64(2)))
		})

		DescribeTable("should return error on invalid value of a known field",
			func(text string) {
				orig := rufh.Limits{MinSize: 42}
				res := orig
				Expect(res.UnmarshalText([]byte(text))).ShouldNot(Succeed())
				Expect(res).Should(Equal(orig), "receiver must not be modified on error")
			},
			Entry("negative integer", "max-size=-1"),
			Entry("negative max-age", "max-age=-5"),
			Entry("string", `max-size="1"`),
			Entry("decimal", "min-size=1.5"),
			Entry("token", "max-append-size=abc"),
			Entry("boolean", "min-append-size=?1"),
			Entry("bare key", "max-age"),
			Entry("empty value", "max-size="),
			Entry("invalid field after valid one", "max-size=1, min-size=x"),
			Entry("min-size greater than max-size", "min-size=11, max-size=10"),
			Entry("min-append-size greater than max-append-size", "min-append-size=11, max-append-size=10"),
		)

		DescribeTable("should accept min values not greater than max values",
			func(text string, expected rufh.Limits) {
				var res rufh.Limits
				Expect(res.UnmarshalText([]byte(text))).Should(Succeed())
				Expect(res).Should(Equal(expected))
			},
			Entry("min-size equal to max-size", "min-size=10, max-size=10",
				rufh.Limits{MinSize: 10, MaxSize: ptr(10)}),
			Entry("min-append-size equal to max-append-size", "min-append-size=10, max-append-size=10",
				rufh.Limits{MinAppendSize: 10, MaxAppendSize: ptr(10)}),
			Entry("min-size without max-size", "min-size=10",
				rufh.Limits{MinSize: 10}),
			Entry("min-append-size without max-append-size", "min-append-size=10",
				rufh.Limits{MinAppendSize: 10}),
		)
	})
})
