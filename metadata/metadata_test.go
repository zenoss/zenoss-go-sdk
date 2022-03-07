package metadata_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_struct "google.golang.org/protobuf/types/known/structpb"

	"github.com/zenoss/zenoss-go-sdk/metadata"
)

func TestMetadata(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	RunSpecs(t, "Metadata Suite")
}

var _ = Describe("Metadata", func() {
	Context("FromMap", func() {
		It("returns correct Fields", func() {
			v, err := metadata.FromMap(map[string]interface{}{
				"bool":     true,
				"nil":      nil,
				"float32":  float32(32),
				"float64":  float64(64),
				"int":      1,
				"int8":     int8(8),
				"int16":    int16(16),
				"int32":    int32(32),
				"int64":    int64(64),
				"uint":     uint(1),
				"uint8":    uint8(8),
				"uint16":   uint16(16),
				"uint32":   uint32(32),
				"uint64":   uint64(64),
				"string":   "test",
				"[]string": []string{"one", "two"},
			})

			Ω(err).ShouldNot(HaveOccurred())
			Ω(v.GetFields()["bool"].GetBoolValue()).Should(Equal(true))
			Ω(v.GetFields()["nil"].GetNullValue()).Should(Equal(_struct.NullValue_NULL_VALUE))
			Ω(v.GetFields()["float32"].GetNumberValue()).Should(Equal(float64(32)))
			Ω(v.GetFields()["float64"].GetNumberValue()).Should(Equal(float64(64)))
			Ω(v.GetFields()["int"].GetNumberValue()).Should(Equal(float64(1)))
			Ω(v.GetFields()["int8"].GetNumberValue()).Should(Equal(float64(8)))
			Ω(v.GetFields()["int16"].GetNumberValue()).Should(Equal(float64(16)))
			Ω(v.GetFields()["int32"].GetNumberValue()).Should(Equal(float64(32)))
			Ω(v.GetFields()["int64"].GetNumberValue()).Should(Equal(float64(64)))
			Ω(v.GetFields()["uint"].GetNumberValue()).Should(Equal(float64(1)))
			Ω(v.GetFields()["uint8"].GetNumberValue()).Should(Equal(float64(8)))
			Ω(v.GetFields()["uint16"].GetNumberValue()).Should(Equal(float64(16)))
			Ω(v.GetFields()["uint32"].GetNumberValue()).Should(Equal(float64(32)))
			Ω(v.GetFields()["uint64"].GetNumberValue()).Should(Equal(float64(64)))
			Ω(v.GetFields()["string"].GetStringValue()).Should(Equal("test"))
			Ω(v.GetFields()["[]string"].GetListValue().GetValues()).Should(HaveLen(2))
			Ω(v.GetFields()["[]string"].GetListValue().GetValues()[0].GetStringValue()).Should(Equal("one"))
			Ω(v.GetFields()["[]string"].GetListValue().GetValues()[1].GetStringValue()).Should(Equal("two"))
		})

		It("returns an error for unsupported types", func() {
			v, err := metadata.FromMap(map[string]interface{}{
				"one": struct{ test string }{test: "test"},
			})

			Ω(err).Should(HaveOccurred())
			Ω(err.Error()).Should(Equal("FromMap can't handle struct { test string }"))
			Ω(v).Should(BeNil())
		})
	})

	Context("MustFromMap", func() {
		Context("with valid input", func() {
			It("succeeds", func() {
				Ω(func() {
					metadata.MustFromMap(map[string]interface{}{
						"test": "value",
					})
				}).ShouldNot(Panic())
			})
		})

		Context("with invalid input", func() {
			It("panics", func() {
				Ω(func() {
					metadata.MustFromMap(map[string]interface{}{
						"test": struct{ test string }{test: "value"},
					})
				}).Should(Panic())
			})
		})
	})

	Context("FromStringMap", func() {
		It("returns correct Fields", func() {
			v := metadata.FromStringMap(map[string]string{
				"first":  "one",
				"second": "two",
			})
			Ω(v.GetFields()).Should(HaveLen(2))
			Ω(v.GetFields()["first"].GetStringValue()).Should(Equal("one"))
			Ω(v.GetFields()["second"].GetStringValue()).Should(Equal("two"))
		})
	})

	Context("FromBool", func() {
		It("returns a correct Value", func() {
			v := metadata.FromBool(true)
			Ω(v.GetBoolValue()).Should(Equal(true))
		})
	})

	Context("ValueFromNil", func() {
		It("returns a correct Value", func() {
			v := metadata.ValueFromNil()
			Ω(v.GetNullValue()).Should(Equal(_struct.NullValue_NULL_VALUE))
		})
	})

	Context("FromNumber", func() {
		It("returns correct Value values", func() {
			Ω(metadata.FromNumber(float32(32)).GetNumberValue()).Should(Equal(float64(32)))
			Ω(metadata.FromNumber(float64(64)).GetNumberValue()).Should(Equal(float64(64)))
			Ω(metadata.FromNumber(1).GetNumberValue()).Should(Equal(float64(1)))
			Ω(metadata.FromNumber(int8(8)).GetNumberValue()).Should(Equal(float64(8)))
			Ω(metadata.FromNumber(int16(16)).GetNumberValue()).Should(Equal(float64(16)))
			Ω(metadata.FromNumber(int32(32)).GetNumberValue()).Should(Equal(float64(32)))
			Ω(metadata.FromNumber(int64(64)).GetNumberValue()).Should(Equal(float64(64)))
			Ω(metadata.FromNumber(uint(1)).GetNumberValue()).Should(Equal(float64(1)))
			Ω(metadata.FromNumber(uint8(8)).GetNumberValue()).Should(Equal(float64(8)))
			Ω(metadata.FromNumber(uint16(16)).GetNumberValue()).Should(Equal(float64(16)))
			Ω(metadata.FromNumber(uint32(32)).GetNumberValue()).Should(Equal(float64(32)))
			Ω(metadata.FromNumber(uint64(64)).GetNumberValue()).Should(Equal(float64(64)))
		})
	})

	Context("FromString", func() {
		It("returns a correct Value", func() {
			v := metadata.FromString("test")
			Ω(v.GetStringValue()).Should(Equal("test"))
		})
	})

	Context("FromStringSlice", func() {
		It("returns a correct Value", func() {
			v := metadata.FromStringSlice([]string{"one", "two"})
			Ω(v.GetListValue().GetValues()).Should(HaveLen(2))
			Ω(v.GetListValue().GetValues()[0].GetStringValue()).Should(Equal("one"))
			Ω(v.GetListValue().GetValues()[1].GetStringValue()).Should(Equal("two"))
		})
	})
})
