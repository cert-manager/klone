#!/bin/bash

set -e

go build -o ./dist/klone_test .

cd ./example && ../dist/klone_test sync
