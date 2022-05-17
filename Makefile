# SPDX-FileCopyrightText: 2022-present Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

GOLANG_CROSS_VERSION := v1.18.1

RUNTIME_VERSION := $(shell go run github.com/atomix/runtime/cmd/atomix-runtime-version@master)

.PHONY: build
build: build-controller build-proxy

build-controller: build-controller-bin build-controller-docker

build-controller-bin:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/controller/bin.yaml --snapshot --rm-dist

build-controller-docker:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/controller/docker.yaml --snapshot --rm-dist

build-proxy: build-proxy-bin build-proxy-docker

build-proxy-bin:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/proxy/bin.yaml --snapshot --rm-dist

build-proxy-docker:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/proxy/docker.yaml --snapshot --rm-dist

.PHONY: release
release: release-controller release-proxy

release-controller: release-controller-bin release-controller-docker

release-controller-bin:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/controller/bin.yaml --rm-dist

release-controller-docker:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/controller/docker.yaml --rm-dist

release-proxy: release-proxy-bin release-proxy-docker

release-proxy-bin:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/proxy/bin.yaml --rm-dist

release-proxy-docker:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-e RUNTIME_VERSION=$(RUNTIME_VERSION) \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/build \
		-w /build \
		goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release -f ./build/proxy/docker.yaml --rm-dist

reuse-tool: # @HELP install reuse if not present
	command -v reuse || python3 -m pip install reuse

license: reuse-tool # @HELP run license checks
	reuse lint
