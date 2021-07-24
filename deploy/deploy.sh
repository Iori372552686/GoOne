#!/bin/sh
#set -x

ENV=$1
OPTION=$2

usage()
{
  echo "Usage: $0 <env> <init|push|start|stop|restart> [role names]..."
  echo "Example: $0 dev1 push mainsvr connsvr  // push file of mainsvr and connsvr"
  echo "         $0 dev1 init  // init all server, no role indicated means all roles"
}

if [[ $# < 2 ]]; then
  usage
  exit 1
fi

#所有的role
ALL_TARGET_ROLES=("commconf" "gamedata" "connsvr" "mainsvr" "dbsvr" "gmconnsvr" "rcmdsvr"
"infosvr" "mysqlsvr" "gamesvr" "gamesvrlua" "opvpsvr" "mailsvr" "friendsvr" "chatsvr"
"wbsvr" "ranksvr" "guildsvr")

#如果命令行没有指明role，则默认为所有role
target_role=()
if [[ $# < 3 ]]; then
    target_role=(`echo ${ALL_TARGET_ROLES[*]}`)
else
    for ((i=3;i<=$#;i++));
    do
       target_role[${#target_role[@]}]=${!i}
    done
fi

#计算tags
target=""
for i in "${!target_role[@]}";  
do
    #最前面不用添加逗号
    if [[ $i != 0 ]]; then
        target="$target,"
    fi
    target="$target${target_role[$i]}_$OPTION"
done

#如果没有tags，则在ansible后面不添加--tags标签
tags="--tags $target"
if [[ $target == "" ]]; then
  tags=""
fi

#由于在子目录运行playbook会有目录结构问题，所以建个临时文件
rand=$RANDOM
TMP=.tmp${rand}.myl
cp playbook_dev/${ENV}.yml ${TMP}
#执行playbook
ansible-playbook -i hosts/host_dev.txt ${TMP} $tags
rm ${TMP}

