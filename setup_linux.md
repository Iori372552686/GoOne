# 部署机器安装
## Install golang from tar
* 如需安装与yum库中不同的版本，可使用tar安装, 建议安装1.13
```sh
# 文档：https://golang.org/doc/install
wget https://dl.google.com/go/go1.13.1.linux-amd64.tar.gz
sudo tar -C /usr/local -vxzf go1.13.1.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/golang.sh
# echo 'export GOROOT=/usr/local/go' >> /etc/profile.d/golang.sh
source /etc/profile.d/golang.sh
```


## Install RabbitMQ
```
sudo yum install rabbitmq-server -y
sudo systemctl start rabbitmq-server
sudo systemctl enable rabbitmq-server

# AdminSite, 
#   access point: http://<host>:15672
#   user/pass: guest/guest
sudo rabbitmq-plugins enable rabbitmq_management

firewall-cmd --zone=public --add-port=5672/tcp --permanent   # rabbitmq
firewall-cmd --zone=public --add-port=15672/tcp --permanent  # rabbitmq_management
firewall-cmd --reload
```


## Install ZooKeeper
```
yum install java-11-openjdk
wget https://downloads.apache.org/zookeeper/zookeeper-3.6.1/apache-zookeeper-3.6.1-bin.tar.gz
#也可以换其他不同版本的，需要bin包，不是源码包

#解压后先在conf文件夹创建zoo.cfg,可以直接赋值zoo_sample.cfg

#bin目录下./zkServer.sh启动
./zkServer.sh start

https://blog.csdn.net/StromNing/article/details/103953466?utm_medium=distribute.pc_aggpage_search_result.none-task-blog-2~all~first_rank_v2~rank_v25-8-103953466.nonecase&utm_term=zookeeper%20%E5%BC%80%E6%9C%BA%E8%87%AA%E5%90%AF
参考连接设置自启动
```

## Install Redis
```
yum install redis -y
vi /etc/redis.conf

systemctl start redis
systemctl enable redis

# 在redis.conf中添加下列配置，以提高安全性 <begin>
rename-command FLUSHALL ""
rename-command CONFIG   ""
rename-command EVAL     ""
requirepass <mypassword>
# 在redis.conf中添加下列配置，以提高安全性 <end>

firewall-cmd --zone=public --add-port=6379/tcp --permanent   # redis
firewall-cmd --reload
```


#如何编译部署？

## Install ansible
```
yum install ansible.noarch
```


##host
```
#gosvr/deploy/inithost host.txt
[local]
127.0.0.1 ansible_ssh_user=root ansible_ssh_pass=pwd ansible_sudo_pass=solgame
#pwd 你的root密码

#gosvr/deploy/hosts/host_dev.txt
add
[dev_local]
127.0.0.1 ansible_ssh_user=user00 ansible_ssh_pass=123456 ansible_sudo_pass=123456

#gosvr/deploy/playbook_dev/dev_local.yml
#gosvr/deploy/playbook_dev/dev_local.vars
```

##init
```
#gosvr/deploy/inithost
ansible-playbook -i host.txt inithost.yml 
```

##protoc
```
#gosvr/deps/protoc/protoc-3.11.4-linux-x86_64/bin
cp protoc protoc-gen-go /user/local/bin
#gosvr/gopath/src/project.me/g1/gamesvr
cp libtolua.so /user/lib64
```


##导表工具
```
#gosvr/gopath/src/project.me/xlstrans
./build.sh

```

##编译部署
```
#gosvr/excel 导表 
./run_me.sh
#gosvr/protocol 导协议
./gen_code.sh
#gosvr 编译
./build.sh
#gosvr/deploy 部署
d.sh dev_local init     #第一次部署 已经有新的部署任务的时候
d.sh dev_local push
d.sh dev_local start
```
