package tag_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/zenoss/zenoss-go-sdk/tag"
)

func TestTag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tag Suite")
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
