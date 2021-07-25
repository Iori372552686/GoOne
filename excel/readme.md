# 使用
* 在gopath/src/project.me/g1/xlstrans中运行build<br />
  会在此目录中生成xlstrans
* xls_list.txt中填上需被转化的xlsx的文件路径列表（以当前目录为父目录）
* 运行run_me
  * 会在data下生成数据文件，
  * 同时在..\gopath\src\project.me\g1\gamedata\中生成对应的go文件

# 机制
* 管理各个xls的go文件，会根据模版文件_conf_mgr_template.go来生成<br />
  其中{STRUCT_NAME}和{DATA_FILE_NAME}会变替换
* 运行xlstrans，会在proto,data,go三个目录中，生成对应的文件。
