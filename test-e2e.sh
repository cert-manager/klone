#!/bin/bash

set -e

go build -o ./dist/klone_test .
klone_binary=$(realpath ./dist/klone_test)

# Test that the sync command works for the example directory
pushd ./example
"$klone_binary" sync
popd

# Create a temp directory and test that the init, add, and sync commands work
temp_dir=$(mktemp -d)
trap '{ rm -rf "$temp_dir"; echo "> Deleted temp dir $temp_dir"; }' EXIT
pushd "$temp_dir"
"$klone_binary" init
mkdir -p a/b
touch a/SHOULD_NOT_BE_DELETED
touch a/b/SHOULD_BE_DELETED
"$klone_binary" add a/b c/d https://github.com/cert-manager/community.git main logo
"$klone_binary" sync
tree -a
popd
