@echo off
chcp 65001 >nul 2>&1
title V-CoPanel System Reset Engine
taskkill /F /IM bridge.exe /T >nul 2>&1
taskkill /F /IM mysqld.exe /T >nul 2>&1
taskkill /F /IM redis-server.exe /T >nul 2>&1
taskkill /F /IM mailpit.exe /T >nul 2>&1
taskkill /F /IM php.exe /T >nul 2>&1
taskkill /F /IM node.exe /T >nul 2>&1

powershell -NoProfile -Command "Get-Process nginx -ErrorAction SilentlyContinue | Where-Object {$_.Path -like '*workspace\shared-services*'} | Stop-Process -Force"

timeout /t 3 /nobreak >nul

cd /d "%~dp0.."
if exist "workspace\shared-services" rmdir /s /q "workspace\shared-services"
if exist "workspace\runtimes" rmdir /s /q "workspace\runtimes"
if exist "workspace\core" rmdir /s /q "workspace\core"
if exist "workspace\mariadb" rmdir /s /q "workspace\mariadb"
if exist "workspace\redis" rmdir /s /q "workspace\redis"
if exist "workspace\mailpit" rmdir /s /q "workspace\mailpit"
if exist "workspace\phpmyadmin" rmdir /s /q "workspace\phpmyadmin"
if exist "workspace\db-admin.json" del /f /q "workspace\db-admin.json"
if exist "workspace\vcredist_installed.flag" del /f /q "workspace\vcredist_installed.flag"

timeout /t 1 /nobreak >nul
start "" "start.bat"
exit
