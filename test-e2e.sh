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
mkdir -p a/b/e
touch a/SHOULD_NOT_BE_DELETED
touch a/b/SHOULD_BE_DELETED
touch a/b/e/SHOULD_BE_DELETED
"$klone_binary" add a/b c/d https://github.com/cert-manager/community.git logo main
"$klone_binary" add a/b e https://github.com/cert-manager/community.git logo main 9f0ea0341816665feadcdcfb7744f4245604ab28
"$klone_binary" sync
if [ -f a/SHOULD_NOT_BE_DELETED ] && [ ! -f a/b/SHOULD_BE_DELETED ]; then
  echo "Test passed"
else
  echo "Test failed"
  exit 1
fi
cat klone.yaml
tree -a
popd
