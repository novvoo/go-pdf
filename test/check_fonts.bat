@echo off
echo === 检查系统字体 ===
echo.

echo 检查 Windows 字体目录:
dir /b C:\Windows\Fonts\*YaHei*.ttf 2>nul
dir /b C:\Windows\Fonts\*SimSun*.ttf 2>nul
dir /b C:\Windows\Fonts\*SimHei*.ttf 2>nul
echo.

echo 运行字体测试程序...
go run font_test.go
echo.

echo 运行换行符测试程序...
go run newline_test.go
echo.

echo === 测试完成 ===
echo 请查看生成的图片:
echo - font_test.png (字体测试)
echo - newline_test.png (换行符测试)
pause
