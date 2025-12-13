#!/usr/bin/env bash

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

set -eu -o pipefail

klone_binary=$(realpath $1)

# 1. Test that the sync command works for the example directory

pushd ./example
"$klone_binary" sync
popd

# 2. Create a temp directory and test that the init, add, and sync commands work

temp_dir=$(mktemp -d)
trap '{ rm -rf "$temp_dir"; echo "> Deleted temp dir $temp_dir"; }' EXIT

pushd "$temp_dir"

"$klone_binary" init

mkdir -p a/b/e

touch a/SHOULD_NOT_BE_DELETED
touch a/b/SHOULD_BE_DELETED
touch a/b/e/SHOULD_BE_DELETED

"$klone_binary" add a/b c/d https://github.com/cert-manager/community.git logo main

"$klone_binary" add a/b e https://github.com/cert-manager/community.git logo main 9f0ea0341816665feadcdcfb7744f4245604ab28

"$klone_binary" sync

if [ ! -f a/SHOULD_NOT_BE_DELETED ]; then
	echo "Test failed: a/SHOULD_NOT_BE_DELETED not found"
	exit 1
fi

if  [ -f a/b/SHOULD_BE_DELETED ]; then
	echo "Test failed: a/b/SHOULD_BE_DELETED was found"
	exit 1
fi

if  [ -f a/b/e/SHOULD_BE_DELETED ]; then
	echo "Test failed: a/b/e/SHOULD_BE_DELETED was found"
	exit 1
fi

echo "> Test succeeded"

echo "> klone.yaml"
cat klone.yaml

echo "> Directory structure"
tree -a

popd
