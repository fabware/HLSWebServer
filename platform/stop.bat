:: 该脚本关闭所有相关服务
@echo off

echo "关闭服务..."
:: 1. 关闭 web服务相关的进程
taskkill /f /im platform.exe
taskkill /f /im bee.exe

:: 2. 关闭pu（vircul_pu__video_1.0d.exe）
taskkill /f /im vircul_pu__video_1.0d.exe

:: 2. 关闭中转
taskkill /f /im datatransfer.exe

:: 3. 关闭vcu( 切片服务器)
taskkill /f /im vcu.exe


:: 4. 关闭遗留的ffmpeg
taskkill /f /im ffmpeg.exe

:: 5. 清理一些没有用cmd
taskkill /f /im cmd.exe
echo "关闭完成"