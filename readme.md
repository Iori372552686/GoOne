
## GoOne Framework：
![image](https://user-images.githubusercontent.com/27808711/126938946-797aa10a-d552-444b-ab66-1ea62d760b60.png)

* [[GoOne架构---详细说明文档]](/doc/G1服务器技术架构文档.docx)


# Environment Setup
* [linux setup](setup_linux.md)
* [windows setup](setup_win.md)

# Install protoc
* 当前项目中，已添加protoc与protoc-gen-go，<br />
  如需更新可参照下面的Appendix：protoc


# Debug Configuration

* goland run config
```
run kind: directory
directory: $svr_dir
working directory: $svr_dir
```
  
* program arguments
```
 -svr_conf=<dir>/<sever_name>_ide.conf       # 旧格式
 -svr_conf=<dir>/<sever_name>_conf_ide.json  # 新格式
 -stderrthreshold=INFO -v=1 # glog
 ```


# Appendix: Proto
* protoc
  * 当前项目目录中，已添加至$PROJECT_ROOT/deps。如需更新，可按以下操作：
  * [下载页面](https://github.com/protocolbuffers/protobuf/releases)
  * [linux下载地址（3.11.4）](https://github.com/protocolbuffers/protobuf/releases/download/v3.11.4/protoc-3.11.4-linux-x86_64.zip)
  * [win下载地址（3.11.4）](https://github.com/protocolbuffers/protobuf/releases/download/v3.11.4/protoc-3.11.4-win64.zip)
  * 解压到$PROJECT_ROOT/deps/protoc中的合适位置，更改$PROJECT_ROOT/protocol/gen_code.(sh|bat)中的protoc的位置
* protoc-gen-go
  * 把GOPATH设置为$PROJECT_ROOT/gopath
  * (可选)当前项目目录中，protoc-gen-go代码已添加至$PROJECT_ROOT/gopath。如需更新，可运行：<br />
    ```go get -d github.com/golang/protobuf/protoc-gen-go```
  * 运行：```go install github.com/golang/protobuf/protoc-gen-go```
  * 确认在$PROJECT_ROOT/gopath/bin中生成了protoc-gen-go
  * 将$PROJECT_ROOT/gopath/bin加到PATH环境变量中

# Appendix : All Golang modules
* 此处列出所有依赖的模块，目前已经加入到工程中

```sh
# protobuf
go get github.com/golang/protobuf/protoc-gen-go  # used by protoc
go get github.com/golang/protobuf/proto
go get github.com/jhump/protoreflect
go get google.golang.org/genproto/protobuf

# databases
go get github.com/gomodule/redigo/redis  # replaced with radix
go get github.com/chasex/redis-go-cluster  # replaced with radix
go get -v github.com/mediocregopher/radix  # substitute redigo and redis-go-cluster
go get github.com/go-sql-driver/mysql

# communication
go get github.com/streadway/amqp            # RabbitMQ
go get github.com/samuel/go-zookeeper/zk    # ZooKeeper

# misc
go get github.com/iancoleman/strcase
go get github.com/tealeg/xlsx
go get github.com/golang/glog


```

