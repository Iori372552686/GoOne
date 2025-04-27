#!/bin/sh

#ulimit -c unlimited
source /etc/profile

SCRIPT_PATH=`pwd`
#SERVER_PATH=`echo ${SCRIPT_PATH%/*}`
SERVER_PATH=$SCRIPT_PATH
SERVER_NAME=`echo ${SERVER_PATH##*/}`

# BASE_PATH=`echo ${SERVER_PATH%/*}`

SERVER_BIN_PATH="${SERVER_PATH}/"
SERVER_PARAM="/data/GoOne/commconf/server_conf.yaml"
SERVER_PARAM_OTHER="${SERVER_PATH}/${SERVER_NAME}_conf2.json"
echo ${SERVER_BIN_PATH}
echo ${SERVER_PARAM}

is_running()
{
    #proc_num=$(ps -ef | grep -w "bin/${SERVER_NAME}" | grep -v grep | wc -l)
    proc_num=$(ps -C  "${SERVER_NAME}" | sed  -e '1d' | wc -l)
    if [ ${proc_num} -gt 0 ];then
        echo "Server ${SERVER_NAME} has already running!"
        return 0
    else
        return 1
    fi
}

start()
{
    is_running
    if [ $? -eq 1 ]; then
        daemonize -e ./err.log -c ./ ${SERVER_BIN_PATH}${SERVER_NAME} -svr_conf=${SERVER_PARAM}
        if [ $? -eq 0 ];then
            ps -C "$SERVER_NAME" -o "pid=" > ${SERVER_NAME}.pid
            echo "Start server ${SERVER_NAME} OK"
        else
            echo "Start server ${SERVER_NAME} FAILED"
        fi
    else
        echo "Start server ${SERVER_NAME} FAILED"
    fi
}

start2()
{
    is_running
    if [ $? -eq 1 ]; then
        daemonize -e ./err.log -c ./ ${SERVER_BIN_PATH}${SERVER_NAME} -svr_conf=${SERVER_PARAM}  -pay_conf=${SERVER_PARAM_OTHER}
        if [ $? -eq 0 ];then
            ps -C "$SERVER_NAME" -o "pid=" > ${SERVER_NAME}.pid
            echo "Start server ${SERVER_NAME} OK"
        else
            echo "Start server ${SERVER_NAME} FAILED"
        fi
    else
        echo "Start server ${SERVER_NAME} FAILED"
    fi
}


stop()
{
    i=3
    stop_flag=0
    while [ $i -gt 0 ]
    do
        killall ${SERVER_NAME}
        sleep 1

        is_running
        if [ $? -eq 1 ]; then
            stop_flag=1
            break
        fi

        ((i=$i-1))
    done
    if [ ${stop_flag} -eq 0 ] ; then
        killall -9 ${SERVER_NAME}
        is_running
        if [ $? -eq 0 ]; then
            stop_flag=1
        fi

    fi

    if [ $stop_flag -eq 1 ];then
        rm ${SERVER_NAME}.pid
        echo "Stop server ${SERVER_NAME} OK"
    else
        echo "Stop server ${SERVER_NAME} FAILED"
    fi

    return 0
}

#clean()
#{
#    str=`grep key ${SERVER_PARAM} | grep shm | awk -F':' '{print $2}'`
#    for key in $str; do
#        ipcrm -M $key
#    done
#}

reload()
{
    is_running
    if [ $? -eq 0 ]; then
        echo "server ${SERVER_NAME} is not running"
    else
        kill -s SIGUSR1 ${SERVER_NAME}
        #${SERVER_BIN_PATH}/${SERVER_NAME} reload
    fi
}

usage()
{
    echo "Usage: $0 [start|stop|restart|check]"
}

if [ $# -lt 1 ];then
    usage
    exit
fi

if [ "$1" = "start" ];then
    start
elif [ "$1" = "start2" ];then
    start2
elif [ "$1" = "stop" ];then
    stop
elif [ "$1" = "restart" ];then
    stop
    start
elif [ "$1" = "restart2" ];then
    stop
    start2
elif [ "$1" = "check" ];then
    is_running
    exit $?
elif [ "$1" = "reload" ];then
    reload
else
    usage
fi
