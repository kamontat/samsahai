package config

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	rclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	s2hv1beta1 "github.com/agoda-com/samsahai/api/v1beta1"
	"github.com/agoda-com/samsahai/internal"
	configctrl "github.com/agoda-com/samsahai/internal/config"
	"github.com/agoda-com/samsahai/internal/util"
)

var _ = Describe("[e2e] Config controller", func() {
	var (
		controller internal.ConfigController
		client     rclient.Client
		namespace  string
		teamName   = "teamtest"
	)

	BeforeEach(func(done Done) {
		defer close(done)

		namespace = os.Getenv("POD_NAMESPACE")
		Expect(namespace).NotTo(BeEmpty(), "Please provided POD_NAMESPACE")

		cfg, err := config.GetConfig()
		Expect(err).To(BeNil(), "Please provide credential for accessing k8s cluster")

		client, err = rclient.New(cfg, rclient.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		controller = configctrl.New(nil, configctrl.WithClient(client))
		Expect(controller).NotTo(BeNil(), "Should successfully init Config controller")
	}, 5)

	AfterEach(func(done Done) {
		defer close(done)
		_ = controller.Delete(teamName)
	}, 5)

	It("should successfully get/delete Config", func(done Done) {
		defer close(done)
		ctx := context.TODO()

		By("Creating Config")
		yamlTeam, err := ioutil.ReadFile(path.Join("..", "data", "wordpress-redis", "config.yaml"))
		Expect(err).NotTo(HaveOccurred())

		obj, _ := util.MustParseYAMLtoRuntimeObject(yamlTeam)
		config, _ := obj.(*s2hv1beta1.Config)
		Expect(client.Create(ctx, config)).To(BeNil())

		By("Get Config")
		cfg, err := controller.Get(teamName)
		Expect(err).To(BeNil())
		Expect(cfg.Spec).NotTo(BeNil())
		Expect(len(cfg.Spec.Components)).To(Equal(2))
		Expect(len(cfg.Spec.Envs)).To(Equal(4))
		Expect(cfg.Spec.Staging).NotTo(BeNil())
		Expect(cfg.Spec.ActivePromotion).NotTo(BeNil())

		By("Get components")
		comps, err := controller.GetComponents(teamName)
		Expect(err).To(BeNil())
		Expect(len(comps)).To(Equal(3))

		By("Get parent components")
		parentComps, err := controller.GetParentComponents(teamName)
		Expect(err).To(BeNil())
		Expect(len(parentComps)).To(Equal(2))

		By("Get bundles")
		bundles, err := controller.GetBundles(teamName)
		Expect(err).To(BeNil())
		dbs, ok := bundles["db"]
		Expect(ok).To(BeTrue())
		Expect(len(dbs)).To(Equal(2))

		By("Delete Config")
		err = controller.Delete(teamName)

		By("Config should be deleted")
		err = wait.PollImmediate(1*time.Second, 5*time.Second, func() (ok bool, err error) {
			config = &s2hv1beta1.Config{}
			err = client.Get(context.TODO(), types.NamespacedName{Name: teamName}, config)
			if err != nil && errors.IsNotFound(err) {
				return true, nil
			}

			return false, nil
		})
		Expect(err).NotTo(HaveOccurred(), "Delete config error")
	}, 10)
})
