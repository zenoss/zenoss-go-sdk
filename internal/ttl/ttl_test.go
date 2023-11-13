package ttl_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/internal/ttl"
	"github.com/zenoss/zenoss-go-sdk/metadata"
)

func TestTTL(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TTL Suite")
}

var _ = Describe("Tracker", func() {
	var checker *ttl.Tracker
	var model *data_receiver.Model
	var now time.Time

	BeforeEach(func() {
		checker = ttl.NewTracker(time.Hour, time.Second)

		model = &data_receiver.Model{
			Timestamp: time.Now().UnixNano() / 1e6,
			Dimensions: map[string]string{
				"source": "bob",
				"app":    "toolbox",
			},
			MetadataFields: metadata.FromStringMap(map[string]string{
				"name":        "Bob's Toolbox",
				"source-type": "test",
			}),
		}

		now = time.Now()
	})

	Describe("IsFresh", func() {
		Context("when model hasn't been seen before", func() {
			It("returns false", func() {
				Ω(checker.IsModelAlive(model)).Should(BeFalse())
			})
		})

		Context("when model has been seen recently", func() {
			BeforeEach(func() {
				_ = checker.IsModelAlive(model)
				checker.AgeAll(now.Add(time.Hour / 2))
			})

			Context("when nothing has changed", func() {
				It("returns true", func() {
					Ω(checker.IsModelAlive(model)).Should(BeTrue())
				})
			})

			Context("when timestamp has changed", func() {
				BeforeEach(func() {
					model.Timestamp = model.Timestamp + 1
				})
				It("returns true", func() {
					Ω(checker.IsModelAlive(model)).Should(BeTrue())
				})
			})

			Context("when dimensions have changed", func() {
				BeforeEach(func() {
					model.Dimensions["source"] = "joe"
				})
				It("returns false", func() {
					Ω(checker.IsModelAlive(model)).Should(BeFalse())
				})
			})

			Context("when metadataFields have changed", func() {
				BeforeEach(func() {
					model.MetadataFields = metadata.FromStringMap(map[string]string{
						"name":        "Joe's Toolbox",
						"source-type": "test",
					})
				})
				It("returns false", func() {
					Ω(checker.IsModelAlive(model)).Should(BeFalse())
				})
			})
		})

		Context("when model hasn't been seen recently", func() {
			BeforeEach(func() {
				_ = checker.IsModelAlive(model)
				checker.AgeAll(now.Add(time.Hour * 2))
			})

			It("returns false", func() {
				Ω(checker.IsModelAlive(model)).Should(BeFalse())
			})
		})
	})
})
