# Install GoLang
* [下载页面](https://golang.org/dl/)
* [下载地址（1.13.1）](https://dl.google.com/go/go1.13.1.windows-amd64.msi)


# Install RabbitMQ
## Dependency: Erlang
* [下载页面](https://www.erlang.org/downloads)
* [下载地址（22.1）](http://erlang.org/download/otp_win64_22.1.exe)
## RabbitMQ
* [下载页面](https://www.rabbitmq.com/download.html)
* [下载地址（3.8.0）](https://github.com/rabbitmq/rabbitmq-server/releases/download/v3.8.0/rabbitmq-server-3.8.0.exe)
## Plugins: rabbitmq_management
* 开启
```
cd ${RabbitMQ_Dir}/sbin
rabbitmq-plugins.bat enable rabbitmq_management
```
* 访问地址：http://localhost:15672
* 账号/密码：guest/guest


# Install ZooKeeper
## Dependency: JRE
* 下载安装JRE（可能需要手动设置JAVA_HOME环境变量）
## ZooKeeper
* 下载
  * [下载页面](https://zookeeper.apache.org/releases.html) （注意是下载bin.tar.gz，而不是tar.gz）
* 解压
* 配置文件
```
cd ${ZOOKEEPER_HOME}/conf
rename zoo_sample.cfg zoo.cfg
edit zoo.cfg  # change dataDir
```
* 环境变量
  * ZOOKEEPER_HOME = ${ZOOKEEPER_HOME}
  * PATH += ;%ZOOKEEPER_HOME%\bin
* 运行bin/zkServer
* GUI查询工具
  * [zkui](https://github.com/echoma/zkui)
  
  
  # Install Redis (Win10)
* 启动WSL。在管理员权限的PowerShell中运行：
```
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Windows-Subsystem-Linux
```
* 在Windows Store中安装Ubuntu
* 登录Ubuntu，更改镜像到aliyun。参照：[aliyun mirror](https://developer.aliyun.com/mirror/ubuntu)
```
cd /etc/apt
sudo cp sources.list sources.list.backup
sudo vi sources.list

:%s/archive.ubuntu.com/mirrors.aliyun.com/gc
:%s/http:/https:/gc
```
* 安装redis-server
```
sudo apt-get update
sudo apt-get upgrade
sudo apt-get install redis-server
sudo service redis-server start
```