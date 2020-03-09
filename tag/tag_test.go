package tag_test

import (
	"math/rand"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"github.com/zenoss/zenoss-go-sdk/tag"
)

func TestTag(t *testing.T) {
	RegisterFailHandler(Fail)
	rand.Seed(GinkgoRandomSeed())
	junitReporter := reporters.NewJUnitReporter("junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "Tag Suite", []Reporter{junitReporter})
}

var _ = Describe("DiscoverAllTags", func() {
	Context("without environment set", func() {
		BeforeEach(func() {
			os.Clearenv()
		})
		It("returns no tags", func() {
			tags := tag.DiscoverAllTags()
			Ω(tags).Should(Equal(tag.Tags{}))
		})
	})

	Context("with Kubernetes environment set", func() {
		BeforeEach(func() {
			_ = os.Setenv(tag.K8sClusterEnv, "cluster77")
			_ = os.Setenv(tag.K8sNamespaceEnv, "namespace77")
			_ = os.Setenv(tag.K8sPodEnv, "pod77")
		})
		It("returns k8s.* tags", func() {
			tags := tag.DiscoverAllTags()
			Ω(tags).Should(Equal(tag.Tags{
				tag.K8sClusterTagKey:   "cluster77",
				tag.K8sNamespaceTagKey: "namespace77",
				tag.K8sPodTagKey:       "pod77",
			}))
		})
	})
})
