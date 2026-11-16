@echo off
setlocal enabledelayedexpansion
set "CWD=%CD%"

REM �������
md test
cd test
rmdir /s /q bin src lib META-INF 2>nul
mkdir src\com\example
mkdir lib

REM ================== ׼�� Hello.java ==================
echo package com.example; > src\com\example\Hello.java
echo public class Hello { >> src\com\example\Hello.java
echo     public static void main(String[] args) { >> src\com\example\Hello.java
echo         System.out.println("Hello World"); >> src\com\example\Hello.java
echo         new depa.A().say(); >> src\com\example\Hello.java
echo         new depb.B().say(); >> src\com\example\Hello.java
echo     } >> src\com\example\Hello.java
echo } >> src\com\example\Hello.java

REM ================== ģ������ A (a.jar) ==================
mkdir depa
echo package depa; > depa\A.java
echo public class A { >> depa\A.java
echo     public void say() { System.out.println("Hello from A"); } >> depa\A.java
echo } >> depa\A.java
javac depa\A.java
jar cf lib\a.jar depa\A.class
rmdir /s /q depa

REM ================== ģ������ B (b.jar) ==================
mkdir depb
echo package depb; > depb\B.java
echo public class B { >> depb\B.java
echo     public void say() { System.out.println("Hello from B"); } >> depb\B.java
echo } >> depb\B.java
javac depb\B.java
jar cf b.jar depb\B.class
echo b.jar > dep.lib
rmdir /s /q depb

REM ================== 1. ��ͨģʽ ==================
echo.
echo === ��ͨģʽ ===
ajava build -main com.example.Hello -jarname app.jar

echo ��� JAR ����:
jar tf bin\app.jar | findstr META-INF/dependencies.lib
if errorlevel 1 echo �����嵥δ���ɣ�
echo ����:
jar xf bin\app.jar META-INF\dependencies.lib
type META-INF\dependencies.lib
del META-INF\dependencies.lib

REM ================== 2. ��̬ģʽ ==================
echo.
echo === ��̬ģʽ ===
rmdir /s /q bin
mkdir bin
ajava build -main com.example.Hello -jarname app.jar -static

echo ��� JAR ����:
jar tf bin\app.jar | findstr META-INF/dependencies-static.lib
if errorlevel 1 echo ��̬�����嵥δ���ɣ�
echo ����:
jar xf bin\app.jar META-INF\dependencies-static.lib
type META-INF\dependencies-static.lib
del META-INF\dependencies-static.lib

REM ================== 3. �������� ==================
echo.
echo === �������� ===
rmdir /s /q bin
mkdir bin
ajava build -main com.example.Hello -jarname app.jar -explib

echo ��鵼���� lib:
dir bin\lib

cd %CWD%
echo.
echo === ������� ===
endlocal
pause
