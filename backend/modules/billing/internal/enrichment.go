package billing

// 实例展示信息富化：billing 需要在「续费列表 / 费用日志」展示坐在席位上的实例名称/状态，
// 但 Modulith 不允许 billing 依赖 phone。用函数指针 provider 反转依赖：phone 装配时注册，
// billing 只持有一个函数指针，无编译期依赖。

// InstanceMeta 实例展示信息（由 phone 提供）。
type InstanceMeta struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// instanceMetaProvider 由 phone 在装配时通过 SetInstanceMetaProvider 注册；nil 表示无富信息。
var instanceMetaProvider func(cpIDs []string) map[string]InstanceMeta

// SetInstanceMetaProvider 注册实例展示信息提供者（phone 模块调用）。传 nil 可清除（测试用）。
func SetInstanceMetaProvider(fn func(cpIDs []string) map[string]InstanceMeta) {
	instanceMetaProvider = fn
}

// lookupInstanceMeta 批量取实例展示信息；无 provider 或无输入时返回空 map。
func lookupInstanceMeta(cpIDs []string) map[string]InstanceMeta {
	if instanceMetaProvider == nil || len(cpIDs) == 0 {
		return map[string]InstanceMeta{}
	}
	return instanceMetaProvider(cpIDs)
}
