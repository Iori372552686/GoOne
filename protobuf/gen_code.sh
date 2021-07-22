#!/usr/bin/env bash

#protoc_dir=../deps/protoc/protoc-3.11.4-linux-x86_64/bin
# for mac dev
#if [ `uname` = "Darwin" ]; then
#  protoc_dir=../deps/protoc/protoc-3.11.4-osx-x86_64/bin
#fi

protoc --go_out=./protocol  *.proto
