package staff

// Permission 单个权限定义
type Permission struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// PermissionGroup 按模块分组的权限
type PermissionGroup struct {
	Module      string       `json:"module"`
	ModuleKey   string       `json:"module_key"`
	Permissions []Permission `json:"permissions"`
}

// PermissionGroups 系统预定义权限列表（代码维护，不存数据库）
var PermissionGroups = []PermissionGroup{
	{
		Module: "仪表盘", ModuleKey: "dashboard",
		Permissions: []Permission{
			{Key: "dashboard:view", Label: "查看仪表盘"},
		},
	},
	{
		Module: "管理员管理", ModuleKey: "staff",
		Permissions: []Permission{
			{Key: "staff:view", Label: "查看用户"},
			{Key: "staff:create", Label: "创建用户"},
			{Key: "staff:edit", Label: "编辑用户"},
			{Key: "staff:delete", Label: "删除用户"},
		},
	},
	{
		Module: "角色管理", ModuleKey: "role",
		Permissions: []Permission{
			{Key: "role:view", Label: "查看角色"},
			{Key: "role:create", Label: "创建角色"},
			{Key: "role:edit", Label: "编辑角色"},
			{Key: "role:delete", Label: "删除角色"},
		},
	},
	{
		Module: "接口访问日志", ModuleKey: "access_log",
		Permissions: []Permission{
			{Key: "access_log:view", Label: "查看访问日志"},
		},
	},
	{
		Module: "示例管理", ModuleKey: "example",
		Permissions: []Permission{
			{Key: "example:view", Label: "查看示例"},
			{Key: "example:create", Label: "创建示例"},
			{Key: "example:edit", Label: "编辑示例"},
			{Key: "example:delete", Label: "删除示例"},
		},
	},
	{
		Module: "用户管理", ModuleKey: "user",
		Permissions: []Permission{
			{Key: "user:view", Label: "查看前台用户"},
			{Key: "user:manage", Label: "管理前台用户（启用/禁用）"},
		},
	},
	{
		Module: "云手机资源", ModuleKey: "cloudphone",
		Permissions: []Permission{
			{Key: "cloudphone:view", Label: "查看云手机资源（规格/云主机/镜像/应用）"},
		},
	},
	{
		Module: "代理池管理", ModuleKey: "proxy",
		Permissions: []Permission{
			{Key: "proxy:view", Label: "查看代理池"},
			{Key: "proxy:manage", Label: "删除/管理代理"},
		},
	},
	{
		Module: "实例管理", ModuleKey: "phone",
		Permissions: []Permission{
			{Key: "phone:view", Label: "查看云手机实例"},
			{Key: "phone:manage", Label: "删除/管理云手机实例"},
		},
	},
	{
		Module: "应用管理", ModuleKey: "app",
		Permissions: []Permission{
			{Key: "app:view", Label: "查看用户上传的应用"},
			{Key: "app:manage", Label: "删除用户上传的应用"},
		},
	},
	{
		Module: "计费管理", ModuleKey: "billing",
		Permissions: []Permission{
			{Key: "billing:view", Label: "查看计费（账户/订单/流水）"},
			{Key: "billing:manage", Label: "管理计费（调整余额/资源、配置定价与试用）"},
		},
	},
	{
		Module: "自动化脚本", ModuleKey: "script",
		Permissions: []Permission{
			{Key: "script:view", Label: "查看脚本商店/用户脚本"},
			{Key: "script:manage", Label: "管理商店脚本/下架用户脚本"},
		},
	},
	{
		Module: "合作商管理", ModuleKey: "partner",
		Permissions: []Permission{
			{Key: "partner:view", Label: "查看代理IP合作商/点击明细"},
			{Key: "partner:manage", Label: "管理代理IP合作商（增删改/上传）"},
		},
	},
	// scaffold:permission-groups
}

// validPermissions 预计算的有效权限集合
var validPermissions map[string]bool

func init() {
	validPermissions = make(map[string]bool)
	for _, g := range PermissionGroups {
		for _, p := range g.Permissions {
			validPermissions[p.Key] = true
		}
	}
}

// IsValidPermission 检查权限标识是否为系统预定义
func IsValidPermission(key string) bool {
	return validPermissions[key]
}
