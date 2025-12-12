@echo off
echo Running PDF tests...
echo ===================

go test -v ./test/gopdf

echo.
echo Tests completed.