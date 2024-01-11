#!/bin/bash

set -e

go build -o ./dist/klone_test .
klone_binary=$(realpath ./dist/klone_test)

pushd ./example
"$klone_binary" sync
popd

temp_dir=$(mktemp -d)
trap '{ rm -rf "$temp_dir"; echo "> Deleted temp dir $temp_dir"; }' EXIT
pushd "$temp_dir"
"$klone_binary" init
popd
