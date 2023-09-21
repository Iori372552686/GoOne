
# GoOne
GoOne 是一款基于Go语言实现的Reactor模型的微服务式分布式游戏架构，继承了很多C++游戏架构的思想，结合Go语言的轻便与开发效率,与主流中间件的完美搭配，适用于中小型游戏，mmo等游戏.


## GoOne Framework：
![image](https://user-images.githubusercontent.com/27808711/126938946-797aa10a-d552-444b-ab66-1ea62d760b60.png)

### 目录结构

```
├─build				编译服务源码所生成的bin执行文件
├─common			基础组件,公共代码,常量定义等
├─deploy			ansible脚本目录,用于自动化
├─excel				xls配置目录,配置转换工具与脚本
├─gamedata			gameconf,xls配置编译后的文件目录
├─lib                           本地lib库源码文件
├─protobuf			        pb protocol文件
├─example			使用示例
├─src			  具体的业务项目源码
   ├─connsvr	          网关服务项目源码
   ├─infosvr		  日志服务项目源码  
   ├─mainsvr	          游戏主逻辑服务源码  
   ├─mysqlsvr	          mysql服务源码  
      
└─main.sh                       主控制台脚本
```

* [[GoOne架构---点击查看详细说明文档]](/doc/G1服务器技术架构文档.docx)
* 项目还在持续优化中,欢迎加入一起创作

# Environment Setup
* [linux setup](/doc/setup_linux.md)
* [windows setup](/doc/setup_win.md)



## 快速开始
#### Linux服务器
> ./main.sh dep  dev_xxname init    --  init dir
>
> ./main.sh dep  dev_xxname push    --  push bin&conf
>
> ./main.sh dep  dev_xxname start     -- start
>
> ./main.sh dep  dev_xxname restart    -- restart





# 如何编译部署？

## Install ansible
```
yum install ansible.noarch
```
### 查看ansible脚本cmd
```
root@iori GoMini]# ./main.sh
Usage Cmd:{build|allbuild|xls|ptc|dep}
```

## host
```
#GoMini/deploy/inithost host.txt
[local]
127.0.0.1 ansible_ssh_user=root ansible_ssh_pass=pwd ansible_sudo_pass=Iori@123
#pwd 你的root密码

#GoMini/deploy/hosts/host_dev.txt
add
[dev_local]
127.0.0.1 ansible_ssh_user=root ansible_ssh_pass=123456 ansible_sudo_pass=123456

#GoMini/deploy/playbook_dev/dev_local.yml
#GoMini/deploy/playbook_dev/dev_local.vars
```

## init
```
#GoMini/deploy/inithost
ansible-playbook -i host.txt inithost.yml 
```

## protoc
```
#GoMini/lib/deps/protoc/protoc-3.11.4-linux-x86_64/bin
cp protoc protoc-gen-go /usr/local/bin
```


## 编译部署
```
#GoMini   --主目录 main.sh 控制台脚本
./main.sh allbuild    转xls_conf + 编译pb协议 + build go svr_src
./main.sh build    build go svr_src , 不带服务名则默认全部编译，带服务名单个编译
./main.sh xls    转xls_conf 
./main.sh ptc    编译pb协议文件
./main.sh dep  dev_name   发布某个服务到dev上，输入可以查看详细指令
```

### K8s&Docker部署

```
Todo 后续
```