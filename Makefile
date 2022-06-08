# SPDX-FileCopyrightText: 2022-present Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

RUNTIME_VERSION := $(shell go run github.com/atomix/runtime/cmd/atomix-runtime-version@master)

.PHONY: build

build:
	RUNTIME_VERSION=$(RUNTIME_VERSION) goreleaser release --snapshot --rm-dist

reuse-tool: # @HELP install reuse if not present
	command -v reuse || python3 -m pip install reuse

license: reuse-tool # @HELP run license checks
	reuse lint
