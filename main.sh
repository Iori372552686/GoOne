#!/bin/bash
#add by  Iori  2020.9.1

ROOT_DIR=$(pwd)           # 启动目录
PROUSER="root"             # 启动程序用户
CMD="$1"					#执行的cmd命令  参数1
DEV_NAME="$2"				#部署的设备index  参数2
DEV_Proc_Cmd="$3"			#部署的设备操作cmd   参数3
DEV_Proc_NAME="$4"			#部署的设备操作对应服务名  参数4

#all proc
start() {
	bulid
	sleep 1
	deploy
	sleep 2
	echo "---->Proc  All Build Done ^_^ !"
}

#excel build code_src
build() {
	echo "---->Proc  building code_src !"
	cd $ROOT_DIR && ./build.sh $DEV_NAME
	echo "---->Proc  build code_src done !"
} 

#excel build code & src & proto
allbuild() {
	excel
	sleep 1
	protocol
	sleep 1
	build
}

#excel build and import    
excel() {
	cd $ROOT_DIR/excel && ./run_me.sh
	echo "---->Proc excel build & import done !"
}

#protocol  build and  import     
protocol() {
	cd $ROOT_DIR/protocol && ./gen_code.sh
	echo "---->Proc protocol build & import done !"
}

#deploy    
deploy() {
    if [ $DEV_NAME ] ;then
		if [ $DEV_Proc_Cmd ] ;then
			if [ $DEV_Proc_Cmd == "init" ] ;then
				cd $ROOT_DIR/deploy && ./deploy.sh $DEV_NAME $DEV_Proc_Cmd
				echo "---->Deploy ##【$DEV_NAME】##  --> Init Done！ "
				
			elif [ $DEV_Proc_Cmd == "all" ];then
			  if [ $DEV_Proc_NAME ] ;then
				  cd $ROOT_DIR/deploy && ./deploy.sh $DEV_NAME push $DEV_Proc_NAME
				  sleep 1
				  ./deploy.sh $DEV_NAME restart $DEV_Proc_NAME
				  echo "---->Deploy ##【$DEV_NAME】##  --> All Done！ "
			  else
				  cd $ROOT_DIR/deploy && ./deploy.sh $DEV_NAME push
				  sleep 1
				  ./deploy.sh $DEV_NAME restart
				  echo "---->Deploy ##【$DEV_NAME】##  --> All Done！ "
				fi
			else
				cd $ROOT_DIR/deploy && ./deploy.sh $DEV_NAME $DEV_Proc_Cmd $DEV_Proc_NAME
				echo "---->Deploy 【$DEV_NAME】— [$DEV_Proc_Cmd]  Done！ "
				
			fi
		else
			echo "----> Deploy cmd [$DEV_Proc_Cmd]  error！Usage Cmd:{init|all|check|start|restart|push} "
		fi
	else
		echo "----> Deploy Dev_index [$DEV_NAME]  error！ plase input DEV_NAME afther Cmd"
    fi
}
 
# allow login   
kai() {
    #cd $ROOTDIR && /bin/sh gm.sh banlogin 0 3
    echo "------------->: [$PRONAME] Allow Login succeed!  ^_^ !"
}   
   
case "$CMD" in
all)
    start
    ;;
       
xls)
    excel
    ;;
       
ptc)
    protocol
    ;;
       
dep)
    deploy
    ;;
	
build)
    build
    ;;

allbuild)
    allbuild
    ;;
kai)
    kai
    ;;         
*)
    echo "Usage Cmd:{all|build|allbuild|xls|ptc|dep|kai}"
esac

