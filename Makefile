# Copyright 2023 The cert-manager Authors.
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

# For details on some of these "prelude" settings, see:
# https://clarkgrubb.com/makefile-style-guide
MAKEFLAGS += --warn-undefined-variables --no-builtin-rules
SHELL := /usr/bin/env bash
.SHELLFLAGS := -uo pipefail -c
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:
.SUFFIXES:
FORCE:

bin_dir := _bin

sources := $(shell find ./cmd ./pkg -name "*.go")

goreleaser_version := v2.6.1
goreleaser := $(bin_dir)/goreleaser-$(goreleaser_version)/goreleaser

.PHONY: build
build: $(bin_dir)/klone

.PHONY: test
test:
	go test ./...

.PHONY: test-e2e
test-e2e: $(bin_dir)/klone
	./test-e2e.sh $<

$(bin_dir)/klone: $(sources) .goreleaser.yaml | $(goreleaser) $(bin_dir)
	$(goreleaser) build --single-target --snapshot --clean --output $@

$(goreleaser): | $(bin_dir)/goreleaser-$(goreleaser_version)
	GOBIN=$(shell pwd)/$| go install github.com/goreleaser/goreleaser/v2@$(goreleaser_version)

.PHONY: print-goreleaser-version
print-goreleaser-version:
	@echo "goreleaser_version=$(goreleaser_version)"

$(bin_dir) $(bin_dir)/goreleaser-$(goreleaser_version):
	mkdir -p $@
