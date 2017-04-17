package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/blang/semver"
	"github.com/golang/glog"
	pflag "github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	ctlUtil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/version"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	expectedVersionKey = "coreos.com/expected-kubelet-version"
	syncPeriod         = 30 * time.Second
)

func main() {
	//TODO(aaron): Change this to accept int or percentage. See Kubernetes autoscaler for prior art.
	var maxNotReady, maxUpdates int

	flag.IntVar(&maxNotReady, "max-not-ready", 1, "maximum number of nodes that can be in NotReady state before updates are halted")
	flag.IntVar(&maxUpdates, "max-updates", 1, "maximum number of nodes that are currently updating before new updates are halted")
	flag.Set("logtostderr", "true")
	flag.Parse()

	util.InitLogs()
	defer util.FlushLogs()

	kubeClient, err := newKubeClient()
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}

	controller := NewVersionController(kubeClient, maxNotReady, maxUpdates)
	controller.Run(syncPeriod)

	select {}
}

func newKubeClient() (client.Interface, error) {
	f := pflag.NewFlagSet("", pflag.ExitOnError)
	config, err := ctlUtil.DefaultClientConfig(f).ClientConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config)
}

type versionController struct {
	client         client.Interface
	nodeController *framework.Controller
	nodeStore      cache.StoreToNodeLister
	maxUpdates     int
	maxNotReady    int
}

func NewVersionController(kubeClient client.Interface, maxNotReady int, maxUpdates int) *versionController {
	vc := &versionController{
		client:      kubeClient,
		maxNotReady: maxNotReady,
		maxUpdates:  maxUpdates,
	}

	vc.nodeStore.Store, vc.nodeController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return vc.client.Nodes().List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(rv string) (watch.Interface, error) {
				return vc.client.Nodes().Watch(labels.Everything(), fields.Everything(), rv)
			},
		},
		&api.Node{},
		controller.NoResyncPeriodFunc(),
		framework.ResourceEventHandlerFuncs{},
	)
	return vc
}

func (vc *versionController) Run(syncPeriod time.Duration) {
	glog.Infof("Starting Version Controller: maxUpdates:%d maxNotReady:%d", vc.maxUpdates, vc.maxNotReady)

	go vc.nodeController.Run(util.NeverStop)
	go util.Until(func() {
		if err := vc.reconcileNodeVersions(); err != nil {
			glog.Errorf("Error reconciling node versions: %v", err)
		}
	}, syncPeriod, util.NeverStop)
}

func (vc *versionController) reconcileNodeVersions() error {
	version, err := vc.getServerVersion()
	if err != nil {
		return fmt.Errorf("failed to determine Kuberntes api-server version: %v", err)
	}

	nodes, err := vc.nodeStore.List()
	if err != nil {
		return fmt.Errorf("failed to retrieve list of nodes to update: %v", err)
	}

	for _, n := range nodes.Items {
		glog.V(6).Infof("Reconciling version for node %s", n.Name)

		if n.Status.NodeInfo.KubeletVersion == version.String() {
			glog.V(4).Infof("Node %s has correct kubelet version: %s", n.Name, version.String())
			continue
		}

		//TODO(aaron): make sure the nodes slice will be updated by the nodeController underneath
		notReady := vc.getNotReadyCount(nodes)
		if notReady >= vc.maxNotReady {
			glog.Infof("Maximum number of nodes in NotReady state (%d/%d)", notReady, vc.maxNotReady)
			return nil
		}

		updating := vc.getUpdatingCount(nodes)
		if updating >= vc.maxUpdates {
			glog.Infof("Maximum number of nodes in update state (%d/%d)", updating, vc.maxUpdates)
			return nil
		}

		return vc.setNodeVersion(n, version)
	}
	return nil
}

func (vc *versionController) setNodeVersion(node api.Node, version semver.Version) error {
	if node.Status.NodeInfo.KubeletVersion == version.String() {
		return nil
	}

	expected, ok := node.ObjectMeta.Annotations[expectedVersionKey]
	if ok && expected == version.String() {
		glog.V(6).Infof("Expected version for node %s already set:%s", node.Name, expected)
		return nil
	}

	updateNode, err := vc.client.Nodes().Get(node.Name)
	if err != nil {
		return err
	}

	if updateNode.ObjectMeta.Annotations == nil {
		updateNode.ObjectMeta.Annotations = make(map[string]string)
	}

	glog.Infof("Setting node %s kubelet version: current=%s expected=%s",
		updateNode.Name, updateNode.Status.NodeInfo.KubeletVersion, version.String())

	updateNode.ObjectMeta.Annotations[expectedVersionKey] = version.String()
	_, err = vc.client.Nodes().Update(updateNode)
	return err
}

func (vc *versionController) getServerVersion() (semver.Version, error) {
	//TODO(aaron): This isn't smart about multiple API servers.
	//    Need a way to safely determine version of all api-servers.
	//    See: https://github.com/kubernetes/kubernetes/issues/18535
	v, err := vc.client.ServerVersion()
	if err != nil {
		return semver.Version{}, err
	}
	return version.Parse(v.String())
}

func (vc *versionController) getNotReadyCount(nodes api.NodeList) int {
	var count int
	for _, n := range nodes.Items {
		for _, condition := range n.Status.Conditions {
			if condition.Type == api.NodeReady && condition.Status != api.ConditionTrue {
				count++
			}
		}
	}
	return count
}

func (vc *versionController) getUpdatingCount(nodes api.NodeList) int {
	var count int
	for _, n := range nodes.Items {
		expected, ok := n.ObjectMeta.Annotations[expectedVersionKey]
		if ok && expected != n.Status.NodeInfo.KubeletVersion {
			count++
		}
	}
	return count
}
