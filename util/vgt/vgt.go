// Copyright 2015 CoreOS, Inc.
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

// VersionGetterThingy is a utility to determine, in a best-effort
// approach, the latest safe version of a kubelet to run. This is done
// by reconciling the api-server version with kubelet acis available
// from a given appc image URL. Once this tool determines the api-server
// version it will check the available tags for an aci such as
// coreos.com/kubelet:tag. It will choose the latest tag less then the
// api-server version but within the same major revision. If an
// accpetable version can not be reconciled either because of availble
// acis or because of the inability to determine the api-server version,
// VersionGetterThingy will fail.
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/cmd/kubelet/app"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

const (
	envFile           = "/run/coreos/kubelet.version"
	imageURL          = "coreos.com/kubelet"
	bootstrapManifest = "/etc/kubernetes/manifests/"
)

func main() {
	//TODO(pb): make imageURL a flag

	apiVersion, err := getServerVersion()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Code Path to replace the dumb attemptVersionReconcile func by
	// actually discovering available images.
	/*
		available, err := getImageVersion(imageURL)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		version, err := reconcileVersion(apiVersion, available)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	*/

	version, err := attemptVersionReconcile(imageURL, apiVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := writeVersionFile(version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func getServerVersion() (*semver.Version, error) {
	s := app.NewKubeletServer()
	s.AddFlags(pflag.CommandLine)
	util.InitFlags()

	clientConfig, err := s.CreateAPIServerClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed getting kubelet version: %v", err)
	}

	kubeClient, err := client.New(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed getting kubelet version: %v", err)
	}

	serverVersion, err := kubeClient.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed getting kubelet version: %v", err)
	}

	return semver.NewVersion(strings.TrimLeft(serverVersion.String(), "v"))
}

// attemptVersionReconcile takes the desired version and polls the image
// discovery url for that tag. If unavailable it will continue
// decrementing the semver within a major version to find a suitable
// image. This is a best effort heuristic since every time we decrement
// a minor version we can't reasonable set the patch version to the
// maximum int64.
func attemptVersionReconcile(imageName string, api *semver.Version) (*semver.Version, error) {
	const (
		maxPatch = 20
		maxTries = 40
	)
	current := *api
	for i := 0; i < maxTries; i++ {
		cmd := exec.Command("rkt", "fetch", imageName+":"+current.String(), "--no-store")
		if err := cmd.Run(); err == nil {
			return &current, nil
		}

		//decrement
		if current.Patch == 0 {
			if current.Minor == 0 {
				break
			}
			current.Minor--
			current.Patch = maxPatch
		} else {
			current.Patch--
		}
	}

	return nil, fmt.Errorf("unable to discover safe version")
}

func writeVersionFile(v *semver.Version) error {
	b := []byte("KUBELET_VERSION=v" + v.String())
	return ioutil.WriteFile(envFile, b, os.ModePerm)
}

// reconcileVersion finds the latest version in acis that is less then
// api but within one major revision. If nothing meets that criteria
// return error.
/*
func reconcileVersion(api semver.Version, images []semver.Version) (*semver.Version, error) {
	var safeVersion *semver.Version
	for _, image := range images {
		// valid canidate?
		if api.LessThan(image) {
			continue
		}
		if api.Major != image.Major {
			continue
		}

		// later then previous canidates?
		if safeVersion != nil && safeVersion.LessThan(image) {
			safeVersion = image
		}
	}
	if safeVersion == nil {
		return nil, fmt.Errorf("failed to reconcile api version with available images")
	}

	return safeVersion, nil
}
*/
