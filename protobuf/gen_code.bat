set protoc_dir=..\lib\deps\protoc\protoc-3.11.4-win64\bin
%protoc_dir%\protoc.exe --go_out=./protocol  *.proto
pause