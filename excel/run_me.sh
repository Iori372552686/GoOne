#!/bin/sh
#set -x

rm -rf ./xls
svn co https://192.168.4.34/svn/ow/trunk/xls
./xlstrans

protoc --go_out=./go/gen -I proto proto/*.proto

gamedata_dir=../gopath/src/project.me/g1/gamedata
mkdir -p ${gamedata_dir}
cp go/gen/*.go  ${gamedata_dir}/


