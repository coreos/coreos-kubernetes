// Copyright 2016 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Kubelet agent is a daemon responsible for both running a kubelet
// service and updating it when a new version is suggested by the
// api-server. The agent launches the kubelet as a transient systemd
// unit file bound to itself such that when the agent restarts itself,
// the kubelet dies with it and it is up to systemd to restart the agent
// to complete the upgrade. Once the kubelet is running the agent will
// only restart if a newer version is found.
//
// The agent will first check for a pinned version via an environment
// variable. If present, the kubelet will be launched at that pinned
// version and no updating will occur until the agent is restarted.
//
// The update logic otherwise will attempt to retrieve a version from
// the api-server via these methods in order by precedence:
//
// - Check the api-server's node object for an annotation (expected to
//   be set by administrator or version controller)
//
// - Check the api-server's node object info for the last run
//   kubelet-version for that node.
//
// - Get the api-server's version and use that.
//
// Once the kubelet is running, the agent will periodically check for a
// newer version to run via the node annotation only. If a newer version
// is found and the aci is fetchable, the agent exits triggering an
// upgrade on next start up.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/blang/semver"
	"github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/dbus"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/cmd/kubelet/app"
	"k8s.io/kubernetes/pkg/api/errors"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/pkg/version"
)

const (
	pinnedENV         = "KUBELET_PINNED_VERSION"
	aciENV            = "KUBELET_ACI"
	defaultACI        = "coreos.com/hyperkube"
	agentName         = "kubelet.service"
	versionAnnotation = "coreos.com/expected-kubelet-version"
	pollSleep         = time.Second * 30
)

// TODO(pb):
// - add restart policy for kubelet
// - do we want to update on any version != current or any version
// greater then current?

func main() {
	var pinned bool
	v, err := getVersionFromEnv()
	if err == nil && v != nil {
		pinned = true
	}

	vc, err := newVersionClient()
	if err != nil {
		log.Fatal(err)
	}

	version, err := getVersion(vc)
	if err != nil || version == nil {
		log.Fatalf("failed getting initial version: %v", err)
	}

	aciName := getACIName()

	if err := startKubelet(version, aciName); err != nil {
		log.Fatal(err)
	}

	if pinned {
		select {} // sleep forever
	}
	agentLoop(version, vc, aciName)
}

// agentLoop will loop over getVersionFromNode until it finds a newer version.
// It will then fetch the aci and exit to trigger the upgrade.
func agentLoop(current *semver.Version, vc *versionClient, aciName string) {
	for {
		time.Sleep(pollSleep)

		newVersion, err := vc.getVersionFromNode()
		if err != nil {
			continue
		}
		if newVersion == nil {
			log.Println("getVersionFromNode returned no error but version is nil")
			continue
		}

		// Restart on newer version iff aci is fetchable
		if newVersion.GT(*current) {
			cmd := exec.Command("rkt", "fetch", fmt.Sprintf("%s:%s", aciName, current))
			if err := cmd.Run(); err == nil {
				log.Printf("restarting kubelet to upgrade from %v to %v", current, newVersion)
				return
			}
			log.Printf("detected new kubelet version: %v but failed to fetch aci: %v", newVersion, err)
		}
	}
}

// Launch kubelet as an aci via systemd transient unit.
func startKubelet(ver *semver.Version, aci string) error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}
	// NOTE(pb): add a defer conn.Close() if vendored dbus package ever
	// updates to require/support that func.

	log.Printf("starting kubelet at version %s", ver)

	execParams := []string{"rkt", "run", "--stage1-image=/usr/share/rkt/stage1-fly.aci",
		fmt.Sprintf("%s:%s", aci, ver),
		"--exec", "/hyperkube", "--", "kubelet",
	}
	execParams = append(execParams, os.Args[1:]...)

	var props []dbus.Property
	props = append(props, dbus.PropExecStart(execParams, true)) // uncleanIsFailure=true
	props = append(props, dbus.PropBindsTo(agentName))

	kubeletServiceName := fmt.Sprintf("kubelet-%s.service", ver)
	_, err = conn.StartTransientUnit(kubeletServiceName, "replace", props...)
	if err != nil {
		return err
	}

	// notify systemd we have started kubelet
	return daemon.SdNotify("READY=1")
}

func getVersionFromEnv() (*semver.Version, error) {
	v := os.Getenv(pinnedENV)
	if v == "" {
		return nil, nil
	}
	return newVersion(v)
}

// get App Container Image name from env
func getACIName() string {
	name := os.Getenv(aciENV)
	if name == "" {
		return defaultACI
	}
	return name
}

// versionClient talks to the api-server using the same flag set and
// methods that the kubelet would.
type versionClient struct {
	apiClient client.Interface
	nodeName  string
}

func newVersionClient() (*versionClient, error) {
	s := app.NewKubeletServer()
	s.AddFlags(pflag.CommandLine)
	util.InitFlags()

	var vc versionClient
	//TODO(aaron): Support cloud-provider nodeName resolution? see RunKubelet() in /cmd/kubelet/app/server.go
	vc.nodeName = nodeutil.GetHostname(s.HostnameOverride)

	apiConfig, err := s.CreateAPIServerClientConfig()
	if err != nil {
		return nil, err
	}

	apiClient, err := client.New(apiConfig)
	if err != nil {
		return nil, err
	}
	vc.apiClient = apiClient

	return &vc, nil
}

// getVersion gets a safe kubelet version to run via the api-server.
// First, the node object is checked for annotations. Second, the node
// object is checked for the last version. Third, getVersion will try to
// return the version of the api-server itself.
func getVersion(vc *versionClient) (*semver.Version, error) {
	if vc == nil {
		return nil, fmt.Errorf("versionClient is nil")
	}

	// From Node Object
	v, err := vc.getVersionFromNode()
	if err == nil {
		return v, nil
	}

	// From API Server
	return vc.getVersionFromAPIServer()
}

// getVersionFromNode looks for a version set on the the kubelet's node
// object on the api-server. This will exist as an annotation set by its
// the administrator or a version controller. In absense of an
// annotation, getVersionFromNode will simply return the kubelets last
// version in the node object. Temporary errors should return a nil
// version and nil error so that getVersion doesn't move onto the next
// case and cause a restart in the agentLoop over something like a
// network failure.
func (vc *versionClient) getVersionFromNode() (*semver.Version, error) {
	node, err := vc.apiClient.Nodes().Get(vc.nodeName)
	// node is found but temporary error
	if err != nil && !errors.IsNotFound(err) {
		log.Printf("temp error getting version from node %q: %v\n", vc.nodeName, err)
		return nil, nil
	}
	// node isn't found
	if err != nil {
		return nil, fmt.Errorf("error getting node %q: %v\n", vc.nodeName, err)
	}
	if node == nil {
		return nil, fmt.Errorf("no node instance returned for %q\n", vc.nodeName)
	}

	// Version from annotation
	if v, ok := node.ObjectMeta.Annotations[versionAnnotation]; ok {
		return newVersion(v)
	}

	// Last version from node
	return newVersion(node.Status.NodeInfo.KubeletVersion)
}

func (vc *versionClient) getVersionFromAPIServer() (*semver.Version, error) {
	serverVersion, err := vc.apiClient.ServerVersion()
	if err != nil {
		return nil, err
	}
	return newVersion(serverVersion.String())
}

func newVersion(v string) (*semver.Version, error) {
	vs, err := version.Parse(v)
	if err != nil {
		return nil, err
	}
	return &vs, nil
}
