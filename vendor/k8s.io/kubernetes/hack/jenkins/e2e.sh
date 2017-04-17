#!/bin/bash

# Copyright 2015 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Sets up environment variables for e2e-runner.sh on the master branch.

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

# Join all args with |
#   Example: join_regex_allow_empty a b "c d" e  =>  a|b|c d|e
function join_regex_allow_empty() {
    local IFS="|"
    echo "$*"
}

# Join all args with |, butin case of empty result prints "EMPTY\sSET" instead.
#   Example: join_regex_no_empty a b "c d" e  =>  a|b|c d|e
#            join_regex_no_empty => EMPTY\sSET
function join_regex_no_empty() {
    local IFS="|"
    if [ -z "$*" ]; then
        echo "EMPTY\sSET"
    else
        echo "$*"
    fi
}

# GCP Project to fetch Trusty images.
TRUSTY_IMAGE_PROJECT="${TRUSTY_IMAGE_PROJECT:-}"

# Get the latest Trusty image for a Jenkins job.
function get_latest_trusty_image() {
    local job_name="${1:-${JOB_NAME}}"
    local image_index=""
    if [[ "${job_name}" =~ trusty-dev ]]; then
      image_index="trusty-dev"
    elif [[ "${job_name}" =~ trusty-beta ]]; then
      image_index="trusty-beta"
    elif [[ "${job_name}" =~ trusty ]]; then
      image_index="trusty"
    fi
    gsutil cat "gs://${TRUSTY_IMAGE_PROJECT}/image-indices/latest-test-image-${image_index}"
    # Clean up gsutil artifacts otherwise the later test stage will complain.
    rm -rf .config &> /dev/null
    rm -rf .gsutil &> /dev/null
}

# Properly configure globals for an upgrade step in a GKE or GCE upgrade suite
#
# These suites:
#   step1: launch a cluster at $old_version,
#   step2: runs $new_version Kubectl e2es,
#   step3: upgrades the master to $new_version,
#   step4: runs $old_version e2es,
#   step5: upgrades the rest of the cluster,
#   step6: runs $old_version e2es again, then
#   step7: runs $new_version e2es and tears down the cluster.
#
# Assumes globals:
#   $JOB_NAME
#   $KUBERNETES_PROVIDER
#   $GKE_DEFAULT_SKIP_TESTS
#   $GCE_DEFAULT_SKIP_TESTS
#   $GCE_FLAKY_TESTS
#   $GCE_SLOW_TESTS
#
# Args:
#   $1 old_version:  the version to deploy a cluster at, and old e2e tests to run
#                    against the upgraded cluster (should be something like
#                    'release/latest', to work with JENKINS_PUBLISHED_VERSION logic)
#   $2 new_version:  the version to upgrade the cluster to, and new e2e tests to run
#                    against the upgraded cluster (should be something like
#                    'ci/latest', to work with JENKINS_PUBLISHED_VERSION logic)
#   $3 cluster_name: determines E2E_CLUSTER_NAME and E2E_NETWORK
#   $4 project:      determines PROJECT

function configure_upgrade_step() {
  local -r old_version="$1"
  local -r new_version="$2"
  local -r cluster_name="$3"
  local -r project="$4"

  [[ "${JOB_NAME}" =~ .*-(step[1-7])-.* ]] || {
    echo "JOB_NAME ${JOB_NAME} is not a valid upgrade job name, could not parse"
    exit 1
  }
  local -r step="${BASH_REMATCH[1]}"

  local -r gce_test_args="--ginkgo.skip=$(join_regex_allow_empty \
        ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
        ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
        ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
        )"
  local -r gke_test_args="--ginkgo.skip=$(join_regex_allow_empty \
        ${GKE_DEFAULT_SKIP_TESTS[@]:+${GKE_DEFAULT_SKIP_TESTS[@]}} \
        ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
        ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
        ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
        )"

  if [[ "${KUBERNETES_PROVIDER}" == "gce" ]]; then
    KUBE_GCE_INSTANCE_PREFIX="$cluster_name"
    NUM_NODES=5
    KUBE_ENABLE_DEPLOYMENTS=true
    KUBE_ENABLE_DAEMONSETS=true
  fi

  E2E_CLUSTER_NAME="$cluster_name"
  E2E_NETWORK="$cluster_name"
  PROJECT="$project"

  case $step in
    step1)
      # Deploy at old version
      JENKINS_PUBLISHED_VERSION="${old_version}"

      E2E_UP="true"
      E2E_TEST="false"
      E2E_DOWN="false"

      if [[ "${KUBERNETES_PROVIDER}" == "gke" ]]; then
        E2E_SET_CLUSTER_API_VERSION=y
      fi
      ;;

    step2)
      # Run new e2e kubectl tests
      JENKINS_PUBLISHED_VERSION="${new_version}"
      JENKINS_FORCE_GET_TARS=y

      E2E_OPT="--check_version_skew=false"
      E2E_UP="false"
      E2E_TEST="true"
      E2E_DOWN="false"
      GINKGO_TEST_ARGS="--ginkgo.focus=Kubectl"
      ;;

    step3)
      # Use upgrade logic of version we're upgrading to.
      JENKINS_PUBLISHED_VERSION="${new_version}"
      JENKINS_FORCE_GET_TARS=y

      E2E_OPT="--check_version_skew=false"
      E2E_UP="false"
      E2E_TEST="true"
      E2E_DOWN="false"
      GINKGO_TEST_ARGS="--ginkgo.focus=Cluster\supgrade.*upgrade-master --upgrade-target=${new_version}"
      ;;

    step4)
      # Run old e2es
      JENKINS_PUBLISHED_VERSION="${old_version}"
      JENKINS_FORCE_GET_TARS=y

      E2E_OPT="--check_version_skew=false"
      E2E_UP="false"
      E2E_TEST="true"
      E2E_DOWN="false"

      if [[ "${KUBERNETES_PROVIDER}" == "gke" ]]; then
        GINKGO_TEST_ARGS="${gke_test_args}"
      else
        GINKGO_TEST_ARGS="${gce_test_args}"
      fi
      ;;

    step5)
      # Use upgrade logic of version we're upgrading to.
      JENKINS_PUBLISHED_VERSION="${new_version}"
      JENKINS_FORCE_GET_TARS=y

      E2E_OPT="--check_version_skew=false"
      E2E_UP="false"
      E2E_TEST="true"
      E2E_DOWN="false"
      GINKGO_TEST_ARGS="--ginkgo.focus=Cluster\supgrade.*upgrade-cluster --upgrade-target=${new_version}"
      ;;

    step6)
      # Run old e2es
      JENKINS_PUBLISHED_VERSION="${old_version}"
      JENKINS_FORCE_GET_TARS=y

      E2E_OPT="--check_version_skew=false"
      E2E_UP="false"
      E2E_TEST="true"
      E2E_DOWN="false"

      if [[ "${KUBERNETES_PROVIDER}" == "gke" ]]; then
        GINKGO_TEST_ARGS="${gke_test_args}"
      else
        GINKGO_TEST_ARGS="${gce_test_args}"
      fi
      ;;

    step7)
      # Run new e2es
      JENKINS_PUBLISHED_VERSION="${new_version}"
      JENKINS_FORCE_GET_TARS=y

      # TODO(15011): these really shouldn't be (very) version skewed, but
      # because we have to get ci/latest again, it could get slightly out of
      # whack.
      E2E_OPT="--check_version_skew=false"
      E2E_UP="false"
      E2E_TEST="true"
      E2E_DOWN="true"

      if [[ "${KUBERNETES_PROVIDER}" == "gke" ]]; then
        GINKGO_TEST_ARGS="${gke_test_args}"
      else
        GINKGO_TEST_ARGS="${gce_test_args}"
      fi
      ;;
  esac
}

echo "--------------------------------------------------------------------------------"
echo "Initial Environment:"
printenv | sort
echo "--------------------------------------------------------------------------------"

if [[ "${CIRCLECI:-}" == "true" ]]; then
    JOB_NAME="circleci-${CIRCLE_PROJECT_USERNAME}-${CIRCLE_PROJECT_REPONAME}"
    BUILD_NUMBER=${CIRCLE_BUILD_NUM}
    WORKSPACE=`pwd`
else
    # Jenkins?
    export HOME=${WORKSPACE} # Nothing should want Jenkins $HOME
fi

# Additional parameters that are passed to hack/e2e.go
E2E_OPT=${E2E_OPT:-""}

# Set environment variables shared for all of the GCE Jenkins projects.
if [[ ${JOB_NAME} =~ ^kubernetes-.*-gce ]]; then
  KUBERNETES_PROVIDER="gce"
  : ${E2E_MIN_STARTUP_PODS:="1"}
  : ${E2E_ZONE:="us-central1-f"}
  : ${NUM_MINIONS_PARALLEL:="6"}  # Number of nodes required to run all of the tests in parallel

elif [[ ${JOB_NAME} =~ ^kubernetes-.*-gke ]]; then
  KUBERNETES_PROVIDER="gke"
  : ${E2E_ZONE:="us-central1-f"}
  # By default, GKE tests run against the GKE test endpoint using CI Cloud SDK.
  # Release jobs (e.g. prod, staging, and test) override these two variables.
  : ${CLOUDSDK_BUCKET:="gs://cloud-sdk-build/testing/staging"}
  : ${GKE_API_ENDPOINT:="https://test-container.sandbox.googleapis.com/"}
elif [[ ${JOB_NAME} =~ ^kubernetes-.*-aws ]]; then
  KUBERNETES_PROVIDER="aws"
  : ${E2E_MIN_STARTUP_PODS:="1"}
  : ${E2E_ZONE:="us-east-1a"}
  : ${NUM_MINIONS_PARALLEL:="6"}  # Number of nodes required to run all of the tests in parallel
fi

if [[ "${KUBERNETES_PROVIDER}" == "aws" ]]; then
  if [[ "${PERFORMANCE:-}" == "true" ]]; then
    : ${MASTER_SIZE:="m3.xlarge"}
    : ${NUM_MINIONS:="100"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.focus=\[Performance\ssuite\]"}
  else
    : ${MASTER_SIZE:="m3.large"}
    : ${MINION_SIZE:="m3.large"}
    : ${NUM_MINIONS:="3"}
  fi
fi

# CURRENT_RELEASE_PUBLISHED_VERSION is the JENKINS_PUBLISHED_VERSION for the
# release we are currently pointing our release testing infrastructure at.
# When 1.2.0-beta.0 comes out, e.g., this will become "ci/latest-1.2"
CURRENT_RELEASE_PUBLISHED_VERSION="ci/latest-1.1"

# Specialized to skip when running reboot tests.
REBOOT_SKIP_TESTS=(
    "Skipped"
    "Restart\sshould\srestart\sall\snodes"
    "Example"
    )

# Specialized tests which should be skipped by default for projects.
GCE_DEFAULT_SKIP_TESTS=(
    "${REBOOT_SKIP_TESTS[@]}"
    "Reboot"
    "ServiceLoadBalancer"
    )

# Tests which cannot be run on GKE, e.g. because they require
# master ssh access.
GKE_REQUIRED_SKIP_TESTS=(
    "Nodes"
    "Etcd\sFailure"
    "MasterCerts"
    "experimental\sresource\susage\stracking" # Expect --max-pods=100
    "ServiceLoadBalancer" # issue: #16602
    "Shell"
    # Alpha features, remove from skip when these move to beta
    "Daemon\sset"
    "Deployment"
    )

# Specialized tests which should be skipped by default for GKE.
GKE_DEFAULT_SKIP_TESTS=(
    "Autoscaling\sSuite"
    # Perf test, slow by design
    "resource\susage\stracking"
    "${GKE_REQUIRED_SKIP_TESTS[@]}"
    )

# Tests which cannot be run on AWS.
AWS_REQUIRED_SKIP_TESTS=(
    "experimental\sresource\susage\stracking" # Expect --max-pods=100
    "GCE\sL7\sLoadBalancer\sController" # GCE L7 loadbalancing
)


# Tests which kills or restarts components and/or nodes.
DISRUPTIVE_TESTS=(
    "DaemonRestart"
    "Etcd\sfailure"
    "Nodes\sResize"
    "Reboot"
    "Services.*restarting"
)

# The following tests are known to be flaky, and are thus run only in their own
# -flaky- build variants.
GCE_FLAKY_TESTS=(
    "DaemonRestart\sController\sManager"
    "Daemon\sset\sshould\srun\sand\sstop\scomplex\sdaemon"
    "Resource\susage\sof\ssystem\scontainers"
    "allows\sscheduling\sof\spods\son\sa\sminion\safter\sit\srejoins\sthe\scluster" # file: resize_nodes.go, issue: #13258
    )

# The following tests are known to be slow running (> 2 min), and are
# thus run only in their own -slow- build variants.  Note that tests
# can be slow by explicit design (e.g. some soak tests), or slow
# through poor implementation.  Please indicate which applies in the
# comments below, and for poorly implemented tests, please quote the
# issue number tracking speed improvements.
GCE_SLOW_TESTS=(
    # Before enabling this loadbalancer test in any other test list you must
    # make sure the associated project has enough quota. At the time of this
    # writing a GCE project is allowed 3 backend services by default. This
    # test requires at least 5.
    "GCE\sL7\sLoadBalancer\sController"               # 10 min,       file: ingress.go,              slow by design
    "SchedulerPredicates\svalidates\sMaxPods\slimit " # 8 min,        file: scheduler_predicates.go, PR:    #13315
    "Nodes\sResize"                                   # 3 min 30 sec, file: resize_nodes.go,         issue: #13323
    "resource\susage\stracking"                       # 1 hour,       file: kubelet_perf.go,         slow by design
    "monotonically\sincreasing\srestart\scount"       # 1.5 to 5 min, file: pods.go,                 slow by design
    "Garbage\scollector\sshould"                      # 7 min,        file: garbage_collector.go,    slow by design
    "KubeProxy\sshould\stest\skube-proxy"             # 9 min 30 sec, file: kubeproxy.go,            issue: #14204
    "cap\sback-off\sat\sMaxContainerBackOff"          # 20 mins       file: manager.go,              PR:    #12648
    )

# Tests which are not able to be run in parallel.
GCE_PARALLEL_SKIP_TESTS=(
    "GCE\sL7\sLoadBalancer\sController" # namespaced watch flakes, issue: #17805
    "Nodes\sNetwork"
    "MaxPods"
    "Resource\susage\sof\ssystem\scontainers"
    "SchedulerPredicates"
    "resource\susage\stracking"
    "${DISRUPTIVE_TESTS[@]}"
    )

# Tests which are known to be flaky when run in parallel.
GCE_PARALLEL_FLAKY_TESTS=(
    "DaemonRestart"
    "Elasticsearch"
    "Namespaces.*should\sdelete\sfast"
    "ServiceAccounts"
    "Services.*identically\snamed" # error waiting for reachability, issue: #16285
    )

# Tests that should not run on soak cluster.
GCE_SOAK_CONTINUOUS_SKIP_TESTS=(
    "GCE\sL7\sLoadBalancer\sController" # issue: #17119
    "Density.*30\spods"
    "Elasticsearch"
    "external\sload\sbalancer"
    "identically\snamed\sservices"
    "network\spartition"
    "Services.*Type\sgoes\sfrom"
    "${DISRUPTIVE_TESTS[@]}"       # avoid component restarts.
    )

GCE_RELEASE_SKIP_TESTS=(
    )

TRUSTY_DEFAULT_SKIP_TESTS=(
    # TODO(wonderfly): Remove this once
    # https://github.com/kubernetes/kubernetes/issues/12689 is fixed.
    "Services.*should\swork\safter\srestarting\skube-proxy"
)

TRUSTY_SKIP_TESTS=(
    "${TRUSTY_DEFAULT_SKIP_TESTS[@]}"
    # These tests rely on cadvisor, which doesn't quite work with Trusty and
    # Trusty dev images yet. An internal issue has been filed to track the fix.
    # TODO(wonderfly): Remove this once the internal issue is fixed.
    "Monitoring\sshould\sverify\smonitoring\spods\sand\sall\scluster\snodes\sare\savailable\son\sinfluxdb\susing\sheapster"
    "Horizontal\spod\sautoscaling"
)

TRUSTY_DEV_SKIP_TESTS=(
    "${TRUSTY_DEFAULT_SKIP_TESTS[@]}"
    # These tests rely on cadvisor, which doesn't quite work with Trusty and
    # Trusty dev images yet. An internal issue has been filed to track the fix.
    # TODO(wonderfly): Remove this once the internal issue is fixed.
    "Monitoring\sshould\sverify\smonitoring\spods\sand\sall\scluster\snodes\sare\savailable\son\sinfluxdb\susing\sheapster"
    "Horizontal\spod\sautoscaling"
)

TRUSTY_BETA_SKIP_TESTS=(
    "${TRUSTY_DEFAULT_SKIP_TESTS[@]}"
)

# Define environment variables based on the Jenkins project name.
# NOTE: Not all jobs are defined here. The hack/jenkins/e2e.sh in master and
# release branches defines relevant jobs for that particular version of
# Kubernetes.
case ${JOB_NAME} in
  # Runs a subset of tests on GCE in parallel. Run against all pending PRs.
  kubernetes-pull-build-test-e2e-gce)
    : ${E2E_CLUSTER_NAME:="jnks-e2e-gce-${NODE_NAME}-${EXECUTOR_NUMBER}"}
    : ${E2E_NETWORK:="e2e-gce-${NODE_NAME}-${EXECUTOR_NUMBER}"}
    : ${GINKGO_PARALLEL:="y"}
    # This list should match the list in kubernetes-e2e-gce-parallel.
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_PARALLEL_SKIP_TESTS[@]:+${GCE_PARALLEL_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          ${GCE_PARALLEL_FLAKY_TESTS[@]:+${GCE_PARALLEL_FLAKY_TESTS[@]}} \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          )"}
    : ${KUBE_GCE_INSTANCE_PREFIX:="e2e-gce-${NODE_NAME}-${EXECUTOR_NUMBER}"}
    : ${KUBE_GCS_STAGING_PATH_SUFFIX:="-${NODE_NAME}-${EXECUTOR_NUMBER}"}
    : ${PROJECT:="kubernetes-jenkins-pull"}
    : ${ENABLE_DEPLOYMENTS:=true}
    # Override GCE defaults
    NUM_MINIONS=${NUM_MINIONS_PARALLEL}
    ;;

  # Runs the performance/scalability tests on GCE using the latest 1.1 ci
  # release. A larger cluster is used.
  kubernetes-e2e-gce-scalability-1.1)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-scalability-1.1"}
    : ${E2E_NETWORK:="e2e-scalability-1-1"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.focus=Performance\ssuite"}
    : ${JENKINS_PUBLISHED_VERSION:="ci/latest-1.1"}
    : ${KUBE_GCE_INSTANCE_PREFIX:="e2e-scalability-1-1"}
    : ${KUBE_GCS_STAGING_PATH_SUFFIX:="scalability-1.1"}
    : ${PROJECT:="k8s-e2e-gce-scalability-1-1"}
    # Override GCE defaults.
    E2E_ZONE="us-east1-b"
    MASTER_SIZE="n1-standard-4"
    MINION_SIZE="n1-standard-2"
    MINION_DISK_SIZE="50GB"
    NUM_MINIONS="100"
    # Reduce logs verbosity
    TEST_CLUSTER_LOG_LEVEL="--v=2"
    # Increase resync period to simulate production
    TEST_CLUSTER_RESYNC_PERIOD="--min-resync-period=12h"
    ;;

  # Sets up the GCE soak cluster weekly using the latest 1.1 ci release.
  kubernetes-soak-weekly-deploy-gce-1.1)
    : ${E2E_CLUSTER_NAME:="gce-soak-weekly-1.1"}
    : ${E2E_DOWN:="false"}
    : ${E2E_NETWORK:="gce-soak-weekly-1-1"}
    : ${E2E_TEST:="false"}
    : ${E2E_UP:="true"}
    : ${JENKINS_PUBLISHED_VERSION:="ci/latest-1.1"}
    : ${KUBE_GCE_INSTANCE_PREFIX:="gce-soak-weekly-1-1"}
    : ${KUBE_GCS_STAGING_PATH_SUFFIX:="soak-1.1"}
    : ${PROJECT:="kubernetes-jenkins"}
    ;;

  # Runs tests on GCE soak cluster for latest 1.1 ci release.
  kubernetes-soak-continuous-e2e-gce-1.1)
    : ${E2E_CLUSTER_NAME:="gce-soak-weekly-1.1"}
    : ${E2E_DOWN:="false"}
    : ${E2E_NETWORK:="gce-soak-weekly-1-1"}
    : ${E2E_UP:="false"}
    # Clear out any orphaned namespaces in case previous run was interrupted.
    : ${E2E_CLEAN_START:="true"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          ${GCE_SOAK_CONTINUOUS_SKIP_TESTS[@]:+${GCE_SOAK_CONTINUOUS_SKIP_TESTS[@]}} \
          )"}
    : ${KUBE_GCE_INSTANCE_PREFIX:="gce-soak-weekly-1-1"}
    : ${PROJECT:="kubernetes-jenkins"}
    ;;

  # Runs non-flaky tests on GCE with Trusty as base image for minions,
  # sequentially.
  kubernetes-e2e-gce-trusty-release)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-trusty-release"}
    : ${E2E_DOWN:="false"}
    : ${E2E_NETWORK:="e2e-gce-trusty-release"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_RELEASE_SKIP_TESTS[@]:+${GCE_RELEASE_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          ${TRUSTY_SKIP_TESTS[@]:+${TRUSTY_SKIP_TESTS[@]}} \
          )"}
    : ${KUBE_GCE_INSTANCE_PREFIX="e2e-gce"}
    : ${PROJECT:="kubekins-e2e-gce-trusty-rls"}
    : ${KUBE_GCE_MINION_PROJECT:="${TRUSTY_IMAGE_PROJECT}"}
    : ${KUBE_GCE_MINION_IMAGE:="$(get_latest_trusty_image ${JOB_NAME})"}
    : ${KUBE_OS_DISTRIBUTION:="trusty"}
    : ${ENABLE_CLUSTER_REGISTRY:=false}
    : ${JENKINS_PUBLISHED_VERSION:="release/stable-1.1"}
    ;;

  # Runs slow tests on GCE with Trusy as base image for minions, sequentially.
  kubernetes-e2e-gce-trusty-slow)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-trusty-slow"}
    : ${E2E_NETWORK:="e2e-gce-trusty-slow"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.focus=$(join_regex_no_empty \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          ) --ginkgo.skip=$(join_regex_allow_empty \
          ${TRUSTY_SKIP_TESTS[@]:+${TRUSTY_SKIP_TESTS[@]}} \
          )"}
    : ${KUBE_GCE_INSTANCE_PREFIX="e2e-trusty-slow"}
    : ${PROJECT:="k8s-e2e-gce-trusty-slow"}
    : ${KUBE_GCE_MINION_PROJECT:="${TRUSTY_IMAGE_PROJECT}"}
    : ${KUBE_GCE_MINION_IMAGE:="$(get_latest_trusty_image ${JOB_NAME})"}
    : ${KUBE_OS_DISTRIBUTION:="trusty"}
    : ${ENABLE_CLUSTER_REGISTRY:=false}
    : ${JENKINS_PUBLISHED_VERSION:="release/stable-1.1"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    ;;

  # Runs non-flaky tests on GCE with Trusty-dev as base image for minions,
  # sequentially.
  kubernetes-e2e-gce-trusty-dev-release)
   : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-trusty-dev-release"}
   : ${E2E_DOWN:="false"}
   : ${E2E_NETWORK:="e2e-gce-trusty-dev-release"}
   : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
         ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
         ${GCE_RELEASE_SKIP_TESTS[@]:+${GCE_RELEASE_SKIP_TESTS[@]}} \
         ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
         ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
         ${TRUSTY_DEV_SKIP_TESTS[@]:+${TRUSTY_DEV_SKIP_TESTS[@]}} \
         )"}
   : ${KUBE_GCE_INSTANCE_PREFIX="e2e-gce"}
   : ${PROJECT:="k8s-e2e-gce-trusty-dev"}
   : ${KUBE_GCE_MINION_PROJECT:="${TRUSTY_IMAGE_PROJECT}"}
   : ${KUBE_GCE_MINION_IMAGE:="$(get_latest_trusty_image ${JOB_NAME})"}
   : ${KUBE_OS_DISTRIBUTION:="trusty"}
   : ${ENABLE_CLUSTER_REGISTRY:=false}
   : ${JENKINS_PUBLISHED_VERSION:="release/stable-1.1"}
   ;;

  # Runs slow tests on GCE with Trusy-dev as base image for minions,
  # sequentially.
  kubernetes-e2e-gce-trusty-dev-slow)
   : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-trusty-dev-slow"}
   : ${E2E_NETWORK:="e2e-gce-trusty-dev-slow"}
   : ${GINKGO_TEST_ARGS:="--ginkgo.focus=$(join_regex_no_empty \
         ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
         ) --ginkgo.skip=$(join_regex_allow_empty \
         ${TRUSTY_DEV_SKIP_TESTS[@]:+${TRUSTY_DEV_SKIP_TESTS[@]}} \
         )"}
   : ${KUBE_GCE_INSTANCE_PREFIX="e2e-trusty-dev-slow"}
   : ${PROJECT:="k8s-e2e-gce-trusty-dev-slow"}
   : ${KUBE_GCE_MINION_PROJECT:="${TRUSTY_IMAGE_PROJECT}"}
   : ${KUBE_GCE_MINION_IMAGE:="$(get_latest_trusty_image ${JOB_NAME})"}
   : ${KUBE_OS_DISTRIBUTION:="trusty"}
   : ${ENABLE_CLUSTER_REGISTRY:=false}
   : ${JENKINS_PUBLISHED_VERSION:="release/stable-1.1"}
   : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
   ;;

  # Runs non-flaky tests on GCE with Trusty-beta as base image for minions,
  # sequentially.
  kubernetes-e2e-gce-trusty-beta-release)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-trusty-beta-release"}
    : ${E2E_DOWN:="false"}
    : ${E2E_NETWORK:="e2e-gce-trusty-beta-release"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_RELEASE_SKIP_TESTS[@]:+${GCE_RELEASE_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          ${TRUSTY_BETA_SKIP_TESTS[@]:+${TRUSTY_BETA_SKIP_TESTS[@]}} \
          )"}
    : ${KUBE_GCE_INSTANCE_PREFIX="e2e-gce"}
    : ${PROJECT:="k8s-e2e-gce-trusty-beta"}
    : ${KUBE_GCE_MINION_PROJECT:="${TRUSTY_IMAGE_PROJECT}"}
    : ${KUBE_GCE_MINION_IMAGE:="$(get_latest_trusty_image ${JOB_NAME})"}
    : ${KUBE_OS_DISTRIBUTION:="trusty"}
    : ${ENABLE_CLUSTER_REGISTRY:=false}
    : ${JENKINS_PUBLISHED_VERSION:="release/stable-1.1"}
    ;;

  # Runs slow tests on GCE with Trusy-beta as base image for minions,
  # sequentially.
  kubernetes-e2e-gce-trusty-beta-slow)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-trusty-beta-slow"}
    : ${E2E_NETWORK:="e2e-gce-trusty-beta-slow"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.focus=$(join_regex_no_empty \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          ) --ginkgo.skip=$(join_regex_allow_empty \
          ${TRUSTY_BETA_SKIP_TESTS[@]:+${TRUSTY_BETA_SKIP_TESTS[@]}} \
          )"}
    : ${KUBE_GCE_INSTANCE_PREFIX="e2e-trusty-beta-slow"}
    : ${PROJECT:="k8s-e2e-gce-trusty-beta-slow"}
    : ${KUBE_GCE_MINION_PROJECT:="${TRUSTY_IMAGE_PROJECT}"}
    : ${KUBE_GCE_MINION_IMAGE:="$(get_latest_trusty_image ${JOB_NAME})"}
    : ${KUBE_OS_DISTRIBUTION:="trusty"}
    : ${ENABLE_CLUSTER_REGISTRY:=false}
    : ${JENKINS_PUBLISHED_VERSION:="release/stable-1.1"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    ;;

  # Runs non-flaky tests on GCE on the release candidate branch,
  # sequentially. As a reminder, if you need to change the skip list
  # or flaky test list on the release branch, you'll need to propose a
  # pull request directly to the release branch itself.
  kubernetes-e2e-gce-release-1.1)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-release-1.1"}
    : ${E2E_DOWN:="false"}
    : ${E2E_NETWORK:="gce-e2e-1-1"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          )"}
    : ${JENKINS_PUBLISHED_VERSION:="ci/latest-1.1"}
    : ${KUBE_GCE_INSTANCE_PREFIX="e2e-gce-1-1"}
    : ${KUBE_GCS_STAGING_PATH_SUFFIX:="release-1.1"}
    : ${PROJECT:="k8s-jkns-e2e-gce-release"}
    : ${ENABLE_DEPLOYMENTS:=true}
    ;;

  # Runs "disruptive" tests on GCE on the release candidate branch,
  # sequentially, against the latest 1.1 ci release.
  # The autoscaling test is not disruptive, however there is no better suite
  # to run it.
  kubernetes-e2e-gce-disruptive-1.1)
    : ${E2E_CLUSTER_NAME:="jenkins-gce-e2e-disruptive-1.1"}
    : ${E2E_DOWN:="false"}
    : ${E2E_NETWORK:="gce-e2e-disruptive-1-1"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${REBOOT_SKIP_TESTS[@]:+${REBOOT_SKIP_TESTS[@]}}\
          ) --ginkgo.focus=$(join_regex_no_empty \
          ${DISRUPTIVE_TESTS[@]:+${DISRUPTIVE_TESTS[@]}} \
          "Autoscaling\sSuite.*via\sreplicationController" \
          "GCE\sL7\sLoadBalancer\sController"
          )"}
    : ${JENKINS_PUBLISHED_VERSION:="ci/latest-1.1"}
    : ${KUBE_GCE_INSTANCE_PREFIX="e2e-gce-disruptive-1-1"}
    : ${KUBE_GCS_STAGING_PATH_SUFFIX:="disruptive-1.1"}
    # TODO: move into its own project
    : ${PROJECT:="kubernetes-jenkins"}
    : ${ENABLE_DEPLOYMENTS:=true}
    : ${ENABLE_DAEMONSETS:=true}
    ;;

  kubernetes-e2e-gke-subnet)
    : ${E2E_CLUSTER_NAME:="jkns-gke-subnet"}
    # auto-subnet manually created - if deleted, it will need to be recreated
    # gcloud alpha compute networks create auto-subnet --mode auto
    : ${E2E_NETWORK:="auto-subnet"}
    : ${E2E_SET_CLUSTER_API_VERSION:=y}
    : ${JENKINS_USE_SERVER_VERSION:=y}
    : ${PROJECT:="k8s-jkns-e2e-gke-subnet"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GKE_DEFAULT_SKIP_TESTS[@]:+${GKE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          )"}
    ;;

  kubernetes-e2e-gke-prod)
    # Override GKE default to use prod Cloud SDK and prod endpoint
    CLOUDSDK_BUCKET=""
    GKE_API_ENDPOINT="https://container.googleapis.com/"
    : ${E2E_CLUSTER_NAME:="jkns-gke-e2e-prod"}
    : ${E2E_NETWORK:="e2e-gke-prod"}
    : ${E2E_SET_CLUSTER_API_VERSION:=y}
    : ${JENKINS_USE_SERVER_VERSION:=y}
    : ${PROJECT:="k8s-jkns-e2e-gke-prod"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GKE_DEFAULT_SKIP_TESTS[@]:+${GKE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          )"}
    ;;

  kubernetes-e2e-gke-staging)
    # Override GKE default to use rc Cloud SDK and prod endpoint
    CLOUDSDK_BUCKET="gs://cloud-sdk-build/testing/rc"
    GKE_API_ENDPOINT="https://staging-container.sandbox.googleapis.com/"
    : ${E2E_CLUSTER_NAME:="jkns-gke-e2e-staging"}
    : ${E2E_NETWORK:="e2e-gke-staging"}
    : ${E2E_SET_CLUSTER_API_VERSION:=y}
    : ${JENKINS_USE_SERVER_VERSION:=y}
    : ${PROJECT:="k8s-jkns-e2e-gke-staging"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GKE_DEFAULT_SKIP_TESTS[@]:+${GKE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          )"}
    ;;

  kubernetes-e2e-gke-test)
    # Override GKE default to use rc Cloud SDK
    CLOUDSDK_BUCKET="gs://cloud-sdk-build/testing/rc"
    : ${E2E_CLUSTER_NAME:="jkns-gke-e2e-test"}
    : ${E2E_NETWORK:="e2e-gke-test"}
    : ${E2E_SET_CLUSTER_API_VERSION:=y}
    : ${JENKINS_USE_SERVER_VERSION:=y}
    : ${PROJECT:="k8s-jkns-e2e-gke-test"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GKE_DEFAULT_SKIP_TESTS[@]:+${GKE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          )"}
    ;;

  kubernetes-e2e-gke-1.1)
    : ${E2E_CLUSTER_NAME:="gke-release-1-1"}
    : ${E2E_NETWORK:="gke-release-1-1"}
    : ${E2E_SET_CLUSTER_API_VERSION:=y}
    : ${JENKINS_PUBLISHED_VERSION:="ci/latest-1.1"}
    : ${PROJECT:="k8s-jkns-e2e-gke-release-1-1"}
    : ${FAIL_ON_GCP_RESOURCE_LEAK:="true"}
    # Note Daemon sets and Deployments are experimental features in 1.1,
    # and are turned off in GKE.
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GKE_DEFAULT_SKIP_TESTS[@]:+${GKE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          )"}
    ;;

  kubernetes-e2e-aws-release-1.1)
    : ${E2E_PUBLISH_GREEN_VERSION:=true}
    : ${E2E_CLUSTER_NAME:="e2e-aws-1-1"}
    : ${E2E_NETWORK:="e2e-aws-1-1"}
    : ${E2E_ZONE:="us-west-2a"}
    : ${ZONE:="us-west-2a"}
    : ${JENKINS_PUBLISHED_VERSION:="ci/latest-1.1"}
    : ${GINKGO_TEST_ARGS:="--ginkgo.skip=$(join_regex_allow_empty \
          ${GCE_DEFAULT_SKIP_TESTS[@]:+${GCE_DEFAULT_SKIP_TESTS[@]}} \
          ${GCE_FLAKY_TESTS[@]:+${GCE_FLAKY_TESTS[@]}} \
          ${GCE_SLOW_TESTS[@]:+${GCE_SLOW_TESTS[@]}} \
          ${AWS_REQUIRED_SKIP_TESTS[@]:+${AWS_REQUIRED_SKIP_TESTS[@]}} \
          )"}
    : ${AWS_CONFIG_FILE:='/var/lib/jenkins/.aws/credentials'}
    : ${AWS_SSH_KEY:='/var/lib/jenkins/.ssh/kube_aws_rsa'}
    : ${KUBE_SSH_USER:='ubuntu'}
    # Needed to be able to create PD from the e2e test
    : ${AWS_SHARED_CREDENTIALS_FILE:='/var/lib/jenkins/.aws/credentials'}
    ;;

  # kubernetes-upgrade-gke-1.1-master
  #
  # Test upgrades from the latest release-1.1 build to the latest master build.
  #
  # Configurations for step2, step3, step5, and step7 live in master.

  kubernetes-upgrade-gke-1.1-master-step1-deploy)
    configure_upgrade_step 'ci/latest-1.1' 'configured-in-master' 'upgrade-gke-1-1-master' 'kubernetes-jenkins-gke-upgrade'
    # Starting in v1.2, NUM_MINIONS/NUM_NODES defaults to 3, so we have to deploy 3 here.
    NUM_MINIONS=3
    ;;

  kubernetes-upgrade-gke-1.1-master-step4-e2e-old)
    configure_upgrade_step 'ci/latest-1.1' 'configured-in-master' 'upgrade-gke-1-1-master' 'kubernetes-jenkins-gke-upgrade'
    # Starting in v1.2, NUM_MINIONS/NUM_NODES defaults to 3, so we have to deploy 3 here.
    NUM_MINIONS=3
    ;;

  kubernetes-upgrade-gke-1.1-master-step6-e2e-old)
    configure_upgrade_step 'ci/latest-1.1' 'configured-in-master' 'upgrade-gke-1-1-master' 'kubernetes-jenkins-gke-upgrade'
    # Starting in v1.2, NUM_MINIONS/NUM_NODES defaults to 3, so we have to deploy 3 here.
    NUM_MINIONS=3
    ;;

  # kubernetes-upgrade-gce-1.1-master
  #
  # Test upgrades from the latest release-1.1 build to the latest master build.
  #
  # Configurations for step2, step3, step5, and step7 live in master.

  kubernetes-upgrade-gce-1.1-master-step1-deploy)
    configure_upgrade_step 'ci/latest-1.1' 'configured-in-master' 'upgrade-gce-1-1-master' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-1.1-master-step4-e2e-old)
    configure_upgrade_step 'ci/latest-1.1' 'configured-in-master' 'upgrade-gce-1-1-master' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-1.1-master-step6-e2e-old)
    configure_upgrade_step 'ci/latest-1.1' 'configured-in-master' 'upgrade-gce-1-1-master' 'k8s-jkns-gce-upgrade'
    ;;

  # kubernetes-upgrade-gke-1.0-current-release
  #
  # Test upgrades from the latest release-1.0 build to the latest current
  # release build.
  #
  # Configurations for step1, step4, and step6 live in the release-1.0 branch.

  kubernetes-upgrade-gke-1.0-current-release-step2-kubectl-e2e-new)
    configure_upgrade_step 'configured-in-release-1.0' 'ci/latest-1.1' 'upgrade-gke-1-0-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-1.0-current-release-step3-upgrade-master)
    configure_upgrade_step 'configured-in-release-1.0' 'ci/latest-1.1' 'upgrade-gke-1-0-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-1.0-current-release-step5-upgrade-cluster)
    configure_upgrade_step 'configured-in-release-1.0' 'ci/latest-1.1' 'upgrade-gke-1-0-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-1.0-current-release-step7-e2e-new)
    configure_upgrade_step 'configured-in-release-1.0' 'ci/latest-1.1' 'upgrade-gke-1-0-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  # kubernetes-upgrade-gce-stable-current-release
  #
  # Test upgrades from the stable build to the latest current release build.

  kubernetes-upgrade-gce-stable-current-release-step1-deploy)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-stable-current-release-step2-kubectl-e2e-new)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-stable-current-release-step3-upgrade-master)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-stable-current-release-step4-e2e-old)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-stable-current-release-step5-upgrade-cluster)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-stable-current-release-step6-e2e-old)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  kubernetes-upgrade-gce-stable-current-release-step7-e2e-new)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gce-st-cur-rel' 'k8s-jkns-gce-upgrade'
    ;;

  # kubernetes-upgrade-gke-stable-current-release
  #
  # Test upgrades from the stable build to the latest current release build.

  kubernetes-upgrade-gke-stable-current-release-step1-deploy)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-stable-current-release-step2-kubectl-e2e-new)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-stable-current-release-step3-upgrade-master)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-stable-current-release-step4-e2e-old)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-stable-current-release-step5-upgrade-cluster)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-stable-current-release-step6-e2e-old)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;

  kubernetes-upgrade-gke-stable-current-release-step7-e2e-new)
    configure_upgrade_step 'release/stable-1.1' 'ci/latest-1.1' 'upgrade-gke-stable-current-release' 'kubernetes-jenkins-gke-upgrade'
    ;;
esac

# AWS variables
export KUBE_AWS_INSTANCE_PREFIX=${E2E_CLUSTER_NAME}
export KUBE_AWS_ZONE=${E2E_ZONE}
export AWS_CONFIG_FILE=${AWS_CONFIG_FILE:-}
export AWS_SSH_KEY=${AWS_SSH_KEY:-}
export KUBE_SSH_USER=${KUBE_SSH_USER:-}
export AWS_SHARED_CREDENTIALS_FILE=${AWS_SHARED_CREDENTIALS_FILE:-}

# GCE variables
export INSTANCE_PREFIX=${E2E_CLUSTER_NAME}
export KUBE_GCE_ZONE=${E2E_ZONE}
export KUBE_GCE_NETWORK=${E2E_NETWORK}
export KUBE_GCE_INSTANCE_PREFIX=${KUBE_GCE_INSTANCE_PREFIX:-}
export KUBE_GCS_STAGING_PATH_SUFFIX=${KUBE_GCS_STAGING_PATH_SUFFIX:-}
export KUBE_GCE_MINION_PROJECT=${KUBE_GCE_MINION_PROJECT:-}
export KUBE_GCE_MINION_IMAGE=${KUBE_GCE_MINION_IMAGE:-}
export KUBE_OS_DISTRIBUTION=${KUBE_OS_DISTRIBUTION:-}
export GCE_SERVICE_ACCOUNT=$(gcloud auth list 2> /dev/null | grep active | cut -f3 -d' ')

# GKE variables
export CLUSTER_NAME=${E2E_CLUSTER_NAME}
export ZONE=${E2E_ZONE}
export KUBE_GKE_NETWORK=${E2E_NETWORK}
export E2E_SET_CLUSTER_API_VERSION=${E2E_SET_CLUSTER_API_VERSION:-}
export CMD_GROUP=${CMD_GROUP:-}
export MACHINE_TYPE=${MINION_SIZE:-}  # GKE scripts use MACHINE_TYPE for the node vm size

if [[ ! -z "${GKE_API_ENDPOINT:-}" ]]; then
  export CLOUDSDK_API_ENDPOINT_OVERRIDES_CONTAINER=${GKE_API_ENDPOINT}
fi

# Shared cluster variables
export E2E_MIN_STARTUP_PODS=${E2E_MIN_STARTUP_PODS:-}
export KUBE_ENABLE_CLUSTER_MONITORING=${ENABLE_CLUSTER_MONITORING:-}
export KUBE_ENABLE_CLUSTER_REGISTRY=${ENABLE_CLUSTER_REGISTRY:-}
export KUBE_ENABLE_HORIZONTAL_POD_AUTOSCALER=${ENABLE_HORIZONTAL_POD_AUTOSCALER:-}
export KUBE_ENABLE_DEPLOYMENTS=${ENABLE_DEPLOYMENTS:-}
export KUBE_ENABLE_EXPERIMENTAL_API=${ENABLE_EXPERIMENTAL_API:-}
export MASTER_SIZE=${MASTER_SIZE:-}
export MINION_SIZE=${MINION_SIZE:-}
export MINION_DISK_SIZE=${MINION_DISK_SIZE:-}
export NUM_MINIONS=${NUM_MINIONS:-}
export TEST_CLUSTER_LOG_LEVEL=${TEST_CLUSTER_LOG_LEVEL:-}
export TEST_CLUSTER_RESYNC_PERIOD=${TEST_CLUSTER_RESYNC_PERIOD:-}
export PROJECT=${PROJECT:-}
export JENKINS_EXPLICIT_VERSION=${JENKINS_EXPLICIT_VERSION:-}
export JENKINS_PUBLISHED_VERSION=${JENKINS_PUBLISHED_VERSION:-'ci/latest-1.1'}

export KUBE_ADMISSION_CONTROL=${ADMISSION_CONTROL:-}

export KUBERNETES_PROVIDER=${KUBERNETES_PROVIDER}
export PATH=${PATH}:/usr/local/go/bin
export KUBE_SKIP_UPDATE=y
export KUBE_SKIP_CONFIRMATIONS=y

# E2E Control Variables
export E2E_OPT="${E2E_OPT:-}"
export E2E_UP="${E2E_UP:-true}"
export E2E_TEST="${E2E_TEST:-true}"
export E2E_DOWN="${E2E_DOWN:-true}"
export E2E_CLEAN_START="${E2E_CLEAN_START:-}"
# Used by hack/ginkgo-e2e.sh to enable ginkgo's parallel test runner.
export GINKGO_PARALLEL=${GINKGO_PARALLEL:-}
export GINKGO_TEST_ARGS="${GINKGO_TEST_ARGS:-}"

# Use e2e-runner.sh from the master branch.
source <(curl -fsS --retry 3 "https://raw.githubusercontent.com/kubernetes/kubernetes/master/hack/jenkins/e2e-runner.sh")
