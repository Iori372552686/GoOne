set project_root_dir="../../../.."

go build -o %project_root_dir%/excel/xlstrans.exe main.go parse_struct.go xls_to_pb.go xls_to_data.go xls_to_go.go xls_to_const.go xls_to_system_unlock.go
pause