package role

import (
	g1_protocol "github.com/Iori372552686/g1_common/protocol"
)

func (r *Role) ActTaskGet(taskId, argv1, argv2, argv3 int32) *g1_protocol.PbTask {
	if r.PbRole.Actvity_Info == nil {
		return nil
	}

	for _, v := range r.PbRole.Actvity_Info.TaskMap {
		if v.TaskId == taskId && v.Argv1 == argv1 && v.Argv2 == argv2 && v.Argv3 == argv3 {
			return v
		}
	}

	return nil
}

func (r *Role) ActTaskGetById(taskId, taskType int32) *g1_protocol.PbTask {
	if r.PbRole.Actvity_Info == nil {
		return nil
	}

	for _, v := range r.PbRole.Actvity_Info.TaskMap {
		if v.TaskId == taskId && v.TaskType == taskType {
			return v
		}
	}

	return nil
}

func (r *Role) ActTaskDelete(taskId int32) bool {
	if r.PbRole.Actvity_Info == nil {
		return false
	}

	delete(r.PbRole.Actvity_Info.TaskMap, taskId)
	r.MarkActvityTaskDirty(taskId, true)
	return true
}

/*
// 多个任务打包上报
func (r *Role) ActTaskReportList(tasks []*g1_protocol.PbTask) int32 {
	updateTaskCnt := int32(0)
	for _, v := range tasks {
		updateTaskCnt += r.ActTaskDoReport(v.TaskId, v.Argv1, v.Argv2, v.Argv3, v.Progress)
	}
	// 有变动则推送给客户端
	if updateTaskCnt > 0 {
		_ = r.SyncDataToClient(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO)
	}
	return updateTaskCnt
}

// 单个任务上报
func (r *Role) ActTaskReport(taskType, argv1, argv2, argv3 int32, progress int32) int32 {
	updateTaskCnt := r.ActTaskDoReport(taskType, argv1, argv2, argv3, progress)
	// 有变动则推送给客户端
	if updateTaskCnt > 0 {
		_ = r.SyncDataToClient(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO)
	}

	return updateTaskCnt
}


func (r *Role) ActTaskDoReport(taskType, argv1, argv2, argv3 int32, progress int32) int32 {
	r.Debugf("Acttask|task report, {id:%v, v1=%v, v2=%v, v3=%v, progress=%v}",
		taskType, argv1, argv2, argv3, progress)

	conf := gamedata.TaskTypeConfMgr.GetOne(taskType)
	if conf == nil {
		return 0
	}

	ret := 0
	for _, tval := range gamedata.TaskConfigConfMgr.GetAll() {
		//不是相关类型任务
		if tval.TaskTargetType != taskType {
			continue
		}

		// 因为所有相关的task都在login时候添加进来了，所以没有找到的taskid都是不相关的task，不用管
		task := r.ActTaskGet(tval.TaskId, argv1, argv2, argv3)
		if task == nil {
			continue
		}

		//记录方式
		if tval.GetRecord() == 2 {
			//todo  check 是否已经解锁
			continue
		}

		//计数方式
		if conf.CountType == 1 {
			task.Progress += progress
		} else if conf.CountType == 2 {
			task.Progress = progress
		}

		//完成条件判断
		if task.HasAward == int32(g1_protocol.ActvityTaskAwardType_A_NULL_TYPE) {
			if tval.Compare == int32(g1_protocol.ActvityTaskCompareType_EQUAL_TYPE) {
				if task.Progress == tval.Target {
					task.HasAward = int32(g1_protocol.ActvityTaskAwardType_UNCLAIMED_TYPE)
				}
			} else if tval.Compare == int32(g1_protocol.ActvityTaskCompareType_BIG_AND_EQUAL_TYPE) {
				if task.Progress >= tval.Target {
					task.HasAward = int32(g1_protocol.ActvityTaskAwardType_UNCLAIMED_TYPE)
				}
			} else if tval.Compare == int32(g1_protocol.ActvityTaskCompareType_SMALL_AND_EQUAL_TYPE) {
				if task.Progress <= tval.Target {
					task.HasAward = int32(g1_protocol.ActvityTaskAwardType_UNCLAIMED_TYPE)
				}
			}
		}

		//check 自动奖励
		if task.HasAward == int32(g1_protocol.ActvityTaskAwardType_UNCLAIMED_TYPE) &&
			tval.GetReceive() == int32(g1_protocol.ActvityTaskReceiveType_AUTO_RECEIVE_TYPE) {
			if 0 == r.ActvityTaskGetReward(tval.TaskId) {
				task.HasAward = int32(g1_protocol.ActvityTaskAwardType_AUTO_TYPE)
			}
		}

		r.Debugf("Acttask|update task, {id:%v, v1=%v, v2=%v, v3=%v, progress=%v}", task.TaskId, task.Argv1, task.Argv2, task.Argv3, task.Progress)
		ret += 1
	}

	r.MainTaskCheckFinish()
	return int32(ret)
}

// 获取奖励
func (r *Role) ActvityTaskGetReward(taskId int32) int {
	if taskId == 0 {
		return int(g1_protocol.ErrorCode_ERR_ARGV)
	}

	conf := gamedata.TaskConfigConfMgr.GetOne(taskId)
	if conf == nil {
		return int(g1_protocol.ErrorCode_ERR_CONF)
	}

	reward := misc.SplitItemFromArray(conf.Reward)
	r.ItemsAdd(reward, &Reason{g1_protocol.Reason_REASON_ACTVITY_TASK_REWARD, taskId})
	return 0
}

// login时更新一次
func (r *Role) ActTaskOnLogin() {
	r.ActTaskAllUpdate()
}

// 创建时初始化
func (r *Role) ActTaskOnCreate() {
	r.ActTaskAllUpdate()
}

// 检测更新任务
func (r *Role) ActTaskAllUpdate() {
	// 活动任务
	act_info := r.PbRole.Actvity_Info
	if act_info == nil {
		act_info = &g1_protocol.RoleActvityTaskInfo{}
	}

	for _, v := range gamedata.TaskConfigConfMgr.GetAll() {
		tag := v.GetUpdateTag() //更新标记
		task := r.ActTaskGetById(v.TaskId, v.TaskType)
		if task == nil && tag == int32(g1_protocol.ActvityTaskUpdateType_AT_NULL_TYPE) {
			task = &g1_protocol.PbTask{TaskId: v.TaskId, TaskType: v.TaskType, Argv1: v.TaskArgv_1, Argv2: v.TaskArgv_2, Argv3: v.TaskArgv_3}
			act_info.TaskList = append(act_info.TaskList, task)
			r.Debugf("add Acttask| %v %v %v %v", v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		} else if task != nil && tag == int32(g1_protocol.ActvityTaskUpdateType_AT_UPDATE_TYPE) {
			task.TaskType = v.TaskType
			task.Argv1 = v.TaskArgv_1
			task.Argv2 = v.TaskArgv_2
			task.Argv3 = v.TaskArgv_3
			r.Debugf("update Acttask| %v %v %v %v", v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		} else if task != nil && tag == int32(g1_protocol.ActvityTaskUpdateType_AT_DELETE_TYPE) {
			r.ActTaskDelete(v.TaskId)
			r.Debugf("delete  Acttask| %d %v %v %v", v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		}
	}
}

// 每月重置刷新
func (r *Role) ActTaskEveryMonthReset(day, hour int32) {
	act_info := r.PbRole.Actvity_Info
	if act_info == nil {
		return
	}

	for _, v := range gamedata.TaskConfigConfMgr.GetAll() {
		tag := v.GetReset_() //重置标记
		if tag == nil {
			continue
		}
		if len(tag) < 3 {
			continue
		}
		if tag[0] != 4 {
			continue
		}
		if tag[1] != day {
			continue
		}
		if tag[2] != hour {
			continue
		}

		task := r.ActTaskGet(v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		if task != nil {
			task.HasAward = int32(g1_protocol.ActvityTaskAwardType_A_NULL_TYPE)
			task.Progress = 0
			r.Debugf("EveryMonthReset  Acttask| %d %v Progress=%v", v.TaskId, task.HasAward, task.Progress)
		}
	}
}

// 每周重置刷新
func (r *Role) ActTaskEveryWeekReset(sameweek bool, weekday, hour int32) {
	act_info := r.PbRole.Actvity_Info
	if act_info == nil {
		return
	}
	if weekday == 0 {
		weekday = 7
	}
	if !sameweek {
		weekday += 7
	}

	for _, v := range gamedata.TaskConfigConfMgr.GetAll() {
		tag := v.GetReset_() //重置标记
		if tag == nil {
			continue
		}
		if len(tag) < 3 {
			continue
		}
		if tag[0] != 3 {
			continue
		}
		if tag[1] > weekday {
			continue
		}
		if tag[1] == weekday && tag[2] > hour {
			continue
		}

		task := r.ActTaskGet(v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		if task != nil {
			task.HasAward = int32(g1_protocol.ActvityTaskAwardType_A_NULL_TYPE)
			task.Progress = 0
			r.Debugf("EveryWeekReset  Acttask| %d %v Progress=%v", v.TaskId, task.HasAward, task.Progress)
		}
	}
}

// 立刻刷新需要重置的任务
func (r *Role) ActTaskRightNowReset() {
	act_info := r.PbRole.Actvity_Info
	if act_info == nil {
		return
	}

	for _, v := range gamedata.TaskConfigConfMgr.GetAll() {
		tag := v.GetReset_() //重置标记
		if tag == nil {
			continue
		}
		if tag[0] != 5 {
			continue
		}

		task := r.ActTaskGet(v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		if task != nil {
			task.HasAward = int32(g1_protocol.ActvityTaskAwardType_A_NULL_TYPE)
			task.Progress = 0
			r.Debugf("RightNowReset  Acttask| %d %v Progress=%v", v.TaskId, task.HasAward, task.Progress)
		}
	}
}

// 每日xx点重置刷新
func (r *Role) ActTaskEveryDaySomeClockReset(sameday bool, hour int32) {
	act_info := r.PbRole.Actvity_Info
	if act_info == nil {
		return
	}
	if !sameday {
		hour += 24
	}

	for _, v := range gamedata.TaskConfigConfMgr.GetAll() {
		tag := v.GetReset_() //重置标记
		if tag == nil {
			continue
		}
		if len(tag) < 2 {
			continue
		}
		if tag[0] != 2 {
			continue
		}
		if tag[1] > hour {
			continue
		}

		task := r.ActTaskGet(v.TaskId, v.TaskArgv_1, v.TaskArgv_2, v.TaskArgv_3)
		if task != nil {
			task.HasAward = int32(g1_protocol.ActvityTaskAwardType_A_NULL_TYPE)
			task.Progress = 0
			r.Debugf("EveryDayReset  Acttask| %d %d %v Progress=%v", hour, v.TaskId, task.HasAward, task.Progress)
		}
	}
}

//CMD_MAIN_ACTIVITY_TASK_SINGLE_UPDATE_NOTIFY
func (r *Role) SendActvityTaskSingleUpdateToSelf(c cmd_handler.IContext, info *g1_protocol.PbTask, flag int32) {
	rsp := &g1_protocol.ActvityTaskSingleUpdate{}
	rsp.UpdateType = flag
	rsp.TaskInfo = info
	router.SendPbMsgByBusIdSimple(c.OriSrcBusId(), c.Uid(), uint32(g1_protocol.CMD_MAIN_ACTIVITY_TASK_SINGLE_UPDATE_NOTIFY), rsp)
}
*/

// ============================================================
// 活动任务段组件（增量档）
// ============================================================

type ActivityTaskComponent struct {
	sectionBase

	actTaskUpserts int32Set
	actTaskDeletes int32Set
}

func NewActivityTaskComponent() *ActivityTaskComponent { return &ActivityTaskComponent{} }

func (c *ActivityTaskComponent) Name() string { return "activity_task" }

func (c *ActivityTaskComponent) Flag() g1_protocol.ERoleSectionFlag {
	return g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO
}

func (c *ActivityTaskComponent) OnInit(r *Role) error { c.bind(r); return nil }
func (c *ActivityTaskComponent) OnDestroy()            {}

// InitField 保持原 RoleInitField 语义：活动任务段不做 nil 兜底。
func (c *ActivityTaskComponent) InitField(uid uint64) {}

// Touch 活动任务段走增量档：标记 patch + 持久化脏（与原 TouchActvityTaskInfo 一致）。
func (c *ActivityTaskComponent) Touch(reason string) {
	c.role.markPatchSection(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO)
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, reason)
}

func (c *ActivityTaskComponent) MarkDirty(taskID int32, deleted bool) {
	c.role.markPatchSection(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO)
	if deleted {
		delInt32Value(&c.actTaskUpserts, taskID)
		setInt32Value(&c.actTaskDeletes, taskID)
	} else {
		delInt32Value(&c.actTaskDeletes, taskID)
		setInt32Value(&c.actTaskUpserts, taskID)
	}
	c.role.markPersistSectionDirty(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, "activity_task")
}

func (c *ActivityTaskComponent) BuildPatch(dst *g1_protocol.ScSyncUserDataV2) bool {
	patch := &g1_protocol.RoleActvityTaskPatch{}
	for _, taskID := range sortedInt32Values(c.actTaskUpserts) {
		if c.role.PbRole.Actvity_Info == nil || c.role.PbRole.Actvity_Info.TaskMap == nil {
			continue
		}
		if task := c.role.PbRole.Actvity_Info.TaskMap[taskID]; task != nil {
			patch.UpsertTasks = append(patch.UpsertTasks, task)
		}
	}
	patch.DeleteTaskIds = sortedInt32Values(c.actTaskDeletes)
	if len(patch.UpsertTasks) == 0 && len(patch.DeleteTaskIds) == 0 {
		return false
	}
	dst.ActvityTaskPatch = patch
	return true
}

func (c *ActivityTaskComponent) ClearDirty() {
	c.actTaskUpserts = nil
	c.actTaskDeletes = nil
}

func activityTaskSection() roleSection {
	return messageSection(g1_protocol.ERoleSectionFlag_ACTVITY_TASK_INFO, "activity_task",
		func(i *g1_protocol.RoleInfo) *g1_protocol.RoleActvityTaskInfo { return i.Actvity_Info },
		func(i *g1_protocol.RoleInfo, m *g1_protocol.RoleActvityTaskInfo) { i.Actvity_Info = m },
		func() *g1_protocol.RoleActvityTaskInfo { return new(g1_protocol.RoleActvityTaskInfo) })
}
