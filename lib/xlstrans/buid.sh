#!/bin/sh

set -x

export GOPATH="${GOPATH}:${project_root_dir}/gopath"
go build -o ../../../../excel/xlstrans main.go parse_struct.go xls_to_pb.go xls_to_data.go xls_to_go.go xls_to_const.go xls_to_system_unlock.go

