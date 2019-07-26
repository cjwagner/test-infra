#!/usr/bin/env bash
# Copyright 2019 The Kubernetes Authors.
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

# Requires go, docker, and kubectl.

set -o errexit
set -o nounset
set -o pipefail

# Prep and check args.
job=${1:-""}
config=${CONFIG_PATH:-""}
job_config=${JOB_CONFIG_PATH:-""}
out_dir=${OUT_DIR:-/tmp/prowjob-out}

echo job=${job}
echo CONFIG_PATH=${config}
echo JOB_CONFIG_PATH=${job_config}
echo OUT_DIR=${out_dir} "(May be different when reusing an existing kind cluster.)"

if [[ -z ${job} ]]
then
  echo "Must specify a job name as the first argument."
  exit 2
fi
if [[ -z ${config} ]]
then
  echo "Must specify config.yaml location via CONFIG_PATH env var."
  exit 2
fi
if [[ -n ${job_config} ]]
then
  job_config="--job-config-path=${job_config}"
fi

# Install kind and set up cluster if not already done.
if [[ -z $(which kind) ]]
then
  GO111MODULE="on" go get sigs.k8s.io/kind@v0.4.0 && kind create cluster
fi
found="false"
for clust in $(kind get clusters)
do
  if [[ ${clust} == "mkpod" ]]
  then
    found="true"
  fi
done
if [[ ${found} == "false" ]]
then
  # Create config file
  cat <<EOF >> ${PWD}/kind-config.yaml
kind: Cluster
apiVersion: kind.sigs.k8s.io/v1alpha3
nodes:
  - extraMounts:
      - containerPath: /output
        hostPath: ${out_dir}
EOF

  kind create cluster --name=mkpod --config=${PWD}/kind-config.yaml --wait=5m
  rm ${PWD}/kind-config.yaml
fi
export KUBECONFIG="$(kind get kubeconfig-path --name="mkpod")"

# Install mkpj and mkpod if not already done.
if [[ -z $(which mkpj) ]]
then
  go get k8s.io/test-infra/prow/cmd/mkpj
fi
if [[ -z $(which mkpod) ]]
then
  go get k8s.io/test-infra/prow/cmd/mkpod
fi

# Generate PJ and Pod.
mkpj --config-path=${config} ${job_config} --job=${job} > ${PWD}/pj.yaml
mkpod --build-id=snowflake --prow-job=${PWD}/pj.yaml --local --out-dir=/output/${job} > ${PWD}/pod.yaml

# Deploy pod and watch.
pod=$(kubectl apply -f ${PWD}/pod.yaml | cut -d ' ' -f 1)
kubectl get ${pod} -w
