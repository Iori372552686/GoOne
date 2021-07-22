package role

// 同步功能开放
func (r *Role) SyncOpenFuncData() {

}

func (r *Role) FuncIsOpen(openFuncId int32) bool {
	if r.PbRole.OpenFunInfo.IsAllOpen {
		return true
	}
	//return r.PbRole.OpenFunInfo.Data[openFuncId]
	for _, v := range r.PbRole.OpenFunInfo.Data {
		if v == openFuncId {
			return true
		}
	}
	return false
}
