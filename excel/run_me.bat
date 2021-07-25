rd /s /q xls
svn co https://192.168.4.34/svn/ow/trunk/xls

xlstrans.exe
protoc --go_out=./go/gen -I proto proto/*.proto

set gamedata_dir=..\gopath\src\project.me\g1\gamedata
mkdir %gamedata_dir%
xcopy /y go\gen\*  %gamedata_dir%\

pause
