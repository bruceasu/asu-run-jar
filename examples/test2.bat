@echo off
setlocal enabledelayedexpansion
set "CWD=%CD%"
rmdir /s /q test 2>nul
mkdir test\src\com\example\spi
mkdir test\srcA\com\example\spi\impl
mkdir test\srcB\com\example\spi\impl
mkdir test\lib
mkdir test\classes
mkdir test\classesA
mkdir test\classesB
cd /d %~dp0test

REM д Hello.java
echo package com.example; > src\com\example\Hello.java
echo import com.example.spi.MyService; >> src\com\example\Hello.java
echo import java.util.ServiceLoader; >> src\com\example\Hello.java
echo public class Hello { >> src\com\example\Hello.java
echo     public static void main(String[] args) { >> src\com\example\Hello.java
echo         System.out.println("== ServiceLoader Test =="); >> src\com\example\Hello.java
echo         ServiceLoader^<MyService^> loader = ServiceLoader.load(MyService.class); >> src\com\example\Hello.java
echo         for (MyService s : loader) { >> src\com\example\Hello.java
echo             s.sayHello(); >> src\com\example\Hello.java
echo         } >> src\com\example\Hello.java
echo     } >> src\com\example\Hello.java
echo } >> src\com\example\Hello.java

REM д�ӿ�
echo package com.example.spi; > src\com\example\spi\MyService.java
echo public interface MyService { >> src\com\example\spi\MyService.java
echo     void sayHello(); >> src\com\example\spi\MyService.java
echo } >> src\com\example\spi\MyService.java

REM дʵ�� A
echo package com.example.spi.impl; > srcA\com\example\spi\impl\A.java
echo import com.example.spi.MyService; >> srcA\com\example\spi\impl\A.java
echo public class A implements MyService { >> srcA\com\example\spi\impl\A.java
echo     public void sayHello() { System.out.println("Hello from Service A"); } >> srcA\com\example\spi\impl\A.java
echo } >> srcA\com\example\spi\impl\A.java

REM дʵ�� B
echo package com.example.spi.impl; > srcB\com\example\spi\impl\B.java
echo import com.example.spi.MyService; >> srcB\com\example\spi\impl\B.java
echo public class B implements MyService { >> srcB\com\example\spi\impl\B.java
echo     public void sayHello() { System.out.println("Hello from Service B"); } >> srcB\com\example\spi\impl\B.java
echo } >> srcB\com\example\spi\impl\B.java

REM ����ӿں� Hello
javac -d classes src\com\example\spi\MyService.java src\com\example\Hello.java

REM ����ʵ�� A������� a.jar
javac -cp classes -d classesA srcA\com\example\spi\impl\A.java
mkdir classesA\META-INF\services
echo com.example.spi.impl.A > metaA\META-INF\services\com.example.spi.MyService
jar cf lib\serviceA.jar -C classesA .

REM ����ʵ�� B������� b.jar
javac -cp classes -d classesB srcB\com\example\spi\impl\B.java
mkdir classesB\META-INF\services
echo com.example.spi.impl.B > metaB\META-INF\services\com.example.spi.MyService
jar cf lib\serviceB.jar -C classesB .

echo.
echo === ׼����ɣ���ʼʹ�� go ��� (fat-jar) ===
ajava build -main com.example.Hello -jarname app.jar -static -src src -out bin

echo.
echo === ���н�� ===
java -jar bin\app.jar

endlocal
cd %CWD%
pause
