#!/bin/sh

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

wget -O checksums.txt https://github.com/cert-manager/klone/releases/download/latest/checksums.txt
wget -O checksums.txt.cosign.bundle https://github.com/cert-manager/klone/releases/download/latest/checksums.txt.cosign.bundle

cosign verify-blob checksums.txt \
    --bundle checksums.txt.cosign.bundle \
    --certificate-identity=https://github.com/cert-manager/klone/.github/workflows/release.yml@refs/tags/v0.0.1-alpha.0 \
    --certificate-oidc-issuer=https://token.actions.githubusercontent.com
