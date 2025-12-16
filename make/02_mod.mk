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

include make/test-unit.mk
include make/test-e2e.mk

.PHONY: print-goreleaser-version
print-goreleaser-version:
	@echo "goreleaser_version=$(GORELEASER_VERSION)"

.PHONY: build
build: $(bin_dir)/klone

sources := **/*.go go.mod go.sum
$(bin_dir)/klone: $(sources) .goreleaser.yaml | $(NEEDS_GORELEASER) $(bin_dir)
	$(GORELEASER) build --single-target --snapshot --clean --output $@
