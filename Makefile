# Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

REGISTRY         := eu.gcr.io/gardener-project
IMAGE_REPOSITORY := $(REGISTRY)/dnslb-controller-manager
IMAGE_TAG        := $(shell cat VERSION)

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start
start:
	go run cmd/dnslb-controller-manager/main.go --kubeconfig dev/kubeconfig-garden.yaml

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: revendor
revendor:
	@dep ensure -update

.PHONY: build
build:
	@build/build

.PHONY: build-local
build-local:
	@env LOCAL_BUILD=1 build/build

.PHONY: release
release: build build-local docker-images docker-login docker-push

.PHONY: docker-images
docker-images: build
	@if [[ ! -f bin/rel/dnslb-controller-manager ]]; then echo "No binary found. Please run 'make build'"; false; fi
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) -t $(IMAGE_REPOSITORY):latest -f Dockerfile --rm .

.PHONY: docker-login
docker-login: docker-images
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push: docker-login
	@if ! docker images $(IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@gcloud docker -- push $(IMAGE_REPOSITORY):$(IMAGE_TAG)
	@gcloud docker -- push $(IMAGE_REPOSITORY):latest

.PHONY: clean
clean:
	@rm -rf bin/
	@rm -f *linux-amd64
	@rm -f *darwin-amd64

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: verify
verify: check test

.PHONY: check
check:
	@.ci/check

.PHONY: test
test:
	@.ci/test

.PHONY: test-cov
test-cov:
	@env COVERAGE=1 .ci/test
	@echo "mode: set" > dnslb-controller-manager.coverprofile && find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> dnslb-controller-manager.coverprofile
	@go tool cover -html=dnslb-controller-manager.coverprofile -o=dnslb-controller-manager.coverage.html
	@rm dnslb-controller-manager.coverprofile

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f dnslb-controller-manager.coverage.html
