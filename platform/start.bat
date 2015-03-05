:: 该脚本是开启所有服务
@echo off


:: 1.启动web服务平台
start bee run platform

echo "begin"
:: 2.开启其他服务
cd ./static/hls/server/
start start.cmd