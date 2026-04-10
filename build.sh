#!/bin/bash
#add by  Iori  2020.10.22

PRONAME="$1"					# 参数1
PRO2="$2"					    # 参数2
project_root_dir=$(pwd)
##export GOPATH=${project_root_dir}/gopath

connsvr() {
  echo "building connsvr !"
  cd ${project_root_dir}/cmd/connsvr
  go build -o ${project_root_dir}/build/connsvr
}

mainsvr() {
  echo "building mainsvr !"
  cd ${project_root_dir}/cmd/mainsvr
  go build -o ${project_root_dir}/build/mainsvr
}

dbsvr() {
  echo "building dbsvr !"
  cd ${project_root_dir}/src/dbsvr
  go build -o ${project_root_dir}/build/dbsvr
}

mysqlsvr() {
  echo "building mysqlsvr !"
  cd ${project_root_dir}/cmd/mysqlsvr
  go build -o ${project_root_dir}/build/mysqlsvr
}

gmconnsvr() {
  echo "building gmconnsvr !"
  cd ${project_root_dir}/src/gmconnsvr
  go build -o ${project_root_dir}/build/gmconnsvr
}

rcmdsvr() {
   echo "building rcmdsvr !"
  cd ${project_root_dir}/src/rcmdsvr
  go build -o ${project_root_dir}/build/rcmdsvr
}

infosvr() {
  echo "building infosvr !"
  cd ${project_root_dir}/cmd/infosvr
  go build -o ${project_root_dir}/build/infosvr
}

roomcentersvr() {
  echo "building roomcentersvr !"
  cd ${project_root_dir}/cmd/roomcentersvr
  go build -o ${project_root_dir}/build/roomcentersvr
}

gamesvr() {
  echo "building gamesvr !"
  cd ${project_root_dir}/src/gamesvr
  go build -o ${project_root_dir}/build/gamesvr
}

opvpsvr() {
  echo "building opvpsvr !"
  cd ${project_root_dir}/src/opvpsvr
  go build -o ${project_root_dir}/build/opvpsvr
}

mailsvr() {
  echo "building mailsvr !"
  cd ${project_root_dir}/src/mailsvr
  go build -o ${project_root_dir}/build/mailsvr
}

chatsvr() {
  echo "building chatsvr !"
  cd ${project_root_dir}/src/chatsvr
  go build -o ${project_root_dir}/build/chatsvr
}

friendsvr() {
  echo "building friendsvr !"
  cd ${project_root_dir}/src/friendsvr
  go build -o ${project_root_dir}/build/friendsvr
}

websvr() {
  echo "building websvr !"
  cd ${project_root_dir}/cmd/web_svr
  go build -o ${project_root_dir}/build/websvr
}

ranksvr() {
  echo "building ranksvr !"
  cd ${project_root_dir}/src/ranksvr
  go build -o ${project_root_dir}/build/ranksvr
}

guildsvr() {
  echo "building guildsvr !"
  cd ${project_root_dir}/src/guildsvr
  go build -o ${project_root_dir}/build/guildsvr
}

case "$PRONAME" in
conn)
    connsvr
    ;;

main)
    mainsvr
    ;;

mysql)
    mysqlsvr
    ;;

db)
    dbsvr
    ;;

gmconn)
    gmconnsvr
    ;;

rcmd)
    rcmdsvr
    ;;

info)
    infosvr
    ;;

roomcenter)
    roomcentersvr
    ;;

game)
    gamesvr
    ;;

opvp)
    opvpsvr
    ;;

mail)
    mailsvr
    ;;

chat)
    chatsvr
    ;;

friend)
    friendsvr
    ;;

web)
    websvr
    ;;

rank)
    ranksvr
    ;;

guild)
    guildsvr
    ;;
*)
    connsvr
    mainsvr
    mysqlsvr
    infosvr
    roomcentersvr
    websvr
esac



