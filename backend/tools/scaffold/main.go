// Command scaffold 生成一个符合本项目 DDD / Modulith 规范的新业务模块（含单元测试）。
//
// 用法：
//
//	go run ./tools/scaffold -name product            # 生成 modules/product（实体 Product，表 products）
//	go run ./tools/scaffold -name order -table orders -title Order
//
// 生成的模块为「后台（staff）受保护的 CRUD」骨架，与 modules/example 同构：
// model / repository（接口+GORM 实现）/ service（领域错误+白名单排序）/ api / module / 公开门面，
// 以及 main_test + service_test。建表与初始数据用 framework.RegisterSetup 登记（无版本号迁移）。
//
// 生成后按提示补三处：main.go 与 apptest/main_test.go 的 blank import、
// 以及 modules/staff/internal/permissions.go 中的权限项（用于授予非超管）。
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

type data struct {
	Name  string // 包名 / 路由前缀 / 权限前缀，小写单词，如 product
	Title string // 实体名 / 服务前缀，PascalCase，如 Product
	Table string // 数据表名，如 products
	Kind  string // staff（后台 RBAC）| user（前台按属主隔离）
}

func main() {
	name := flag.String("name", "", "模块名（小写单词，如 product）")
	title := flag.String("title", "", "实体名 PascalCase（默认由 name 推导）")
	table := flag.String("table", "", "数据表名（默认 name+\"s\"）")
	root := flag.String("root", ".", "项目根目录（含 go.mod）")
	kind := flag.String("kind", "staff", "模块风格：staff（后台 RBAC）| user（前台按属主隔离登录）")
	skipWire := flag.Bool("skip-wire", false, "不自动改写 main.go/apptest/permissions.go，仅生成模块文件")
	flag.Parse()

	if *name == "" || !regexp.MustCompile(`^[a-z][a-z0-9]*$`).MatchString(*name) {
		fmt.Fprintln(os.Stderr, "错误：-name 必填且须为小写字母/数字单词，如 -name product")
		os.Exit(2)
	}
	if *kind != "staff" && *kind != "user" {
		fmt.Fprintln(os.Stderr, "错误：-kind 只能是 staff 或 user")
		os.Exit(2)
	}

	d := data{Name: *name, Title: *title, Table: *table, Kind: *kind}
	if d.Title == "" {
		d.Title = strings.ToUpper((*name)[:1]) + (*name)[1:]
	}
	if d.Table == "" {
		d.Table = *name + "s"
	}

	moduleDir := filepath.Join(*root, "modules", d.Name)
	if _, err := os.Stat(moduleDir); err == nil {
		fmt.Fprintf(os.Stderr, "错误：%s 已存在，拒绝覆盖\n", moduleDir)
		os.Exit(1)
	}

	// 按风格选择模板：staff = 后台 RBAC CRUD；user = 前台按属主隔离的 per-owner CRUD。
	gen := map[string]string{
		"{{.Name}}.go":          facadeTmpl,
		"internal/main_test.go": mainTestTmpl,
	}
	if d.Kind == "user" {
		gen["internal/model.go"] = modelTmplUser
		gen["internal/repository.go"] = repoTmplUser
		gen["internal/service.go"] = serviceTmplUser
		gen["internal/api.go"] = apiTmplUser
		gen["internal/module.go"] = moduleTmplUser
		gen["internal/service_test.go"] = serviceTestTmplUser
	} else {
		gen["internal/model.go"] = modelTmpl
		gen["internal/repository.go"] = repoTmpl
		gen["internal/service.go"] = serviceTmpl
		gen["internal/api.go"] = apiTmpl
		gen["internal/module.go"] = moduleTmplStaff
		gen["internal/service_test.go"] = serviceTestTmpl
	}

	for rel, tmpl := range gen {
		out := filepath.Join(moduleDir, strings.ReplaceAll(rel, "{{.Name}}", d.Name))
		if err := render(out, tmpl, d); err != nil {
			fmt.Fprintf(os.Stderr, "生成 %s 失败：%v\n", out, err)
			os.Exit(1)
		}
		fmt.Println("created", out)
	}

	fmt.Printf("\n模块 %q 已生成（实体 %s，表 %s）。\n", d.Name, d.Title, d.Table)

	if *skipWire {
		fmt.Printf(`
未自动改写（-skip-wire）。请手动：
  1. main.go 与 apptest/main_test.go 增加 _ "manager-backend/modules/%s"
  2. modules/staff/internal/permissions.go 登记 %s:view/create/edit/delete
`, d.Name, d.Name)
	} else if err := autoWire(*root, d); err != nil {
		fmt.Fprintf(os.Stderr, "\n自动改写失败：%v\n请手动登记 blank import 与权限。\n", err)
		os.Exit(1)
	}

	// 格式化生成与改写的文件。
	run(*root, "gofmt", "-w",
		filepath.Join("modules", d.Name),
		"main.go",
		filepath.Join("apptest", "main_test.go"),
		filepath.Join("modules", "staff", "internal", "permissions.go"),
	)

	fmt.Printf("\n完成。建议执行： go build ./... && go test ./modules/%s/... ./framework/\n", d.Name)
}

// autoWire 在锚点处插入 blank import 与权限组（幂等：已存在则跳过）。
func autoWire(root string, d data) error {
	importLine := fmt.Sprintf("\t_ \"manager-backend/modules/%s\"", d.Name)
	importDedup := fmt.Sprintf("\"manager-backend/modules/%s\"", d.Name)
	for _, p := range []string{"main.go", filepath.Join("apptest", "main_test.go")} {
		if err := insertBeforeAnchor(filepath.Join(root, p), "// scaffold:module-imports", importLine, importDedup); err != nil {
			return err
		}
		fmt.Println("wired", p)
	}

	// 前台模块无 RBAC，不登记权限。
	if d.Kind != "staff" {
		return nil
	}

	permBlock := fmt.Sprintf(`	{
		Module: "%s", ModuleKey: "%s",
		Permissions: []Permission{
			{Key: "%s:view", Label: "查看%s"},
			{Key: "%s:create", Label: "创建%s"},
			{Key: "%s:edit", Label: "编辑%s"},
			{Key: "%s:delete", Label: "删除%s"},
		},
	},`, d.Title, d.Name, d.Name, d.Title, d.Name, d.Title, d.Name, d.Title, d.Name, d.Title)
	permPath := filepath.Join(root, "modules", "staff", "internal", "permissions.go")
	if err := insertBeforeAnchor(permPath, "// scaffold:permission-groups", permBlock, fmt.Sprintf("ModuleKey: \"%s\"", d.Name)); err != nil {
		return err
	}
	fmt.Println("wired modules/staff/internal/permissions.go")
	return nil
}

// insertBeforeAnchor 在含 anchor 的行之前插入 text；若 dedup 子串已存在则跳过；找不到锚点报错。
func insertBeforeAnchor(path, anchor, text, dedup string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(b)
	if strings.Contains(content, dedup) {
		return nil // 已登记，幂等跳过
	}
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, anchor) {
			out := append([]string{}, lines[:i]...)
			out = append(out, text)
			out = append(out, lines[i:]...)
			return os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o644)
		}
	}
	return fmt.Errorf("在 %s 未找到锚点 %q", path, anchor)
}

func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "%s 执行失败：%v\n%s\n", name, err, out)
	}
}

func render(path, tmpl string, d data) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	t, err := template.New(path).Funcs(template.FuncMap{"bq": func() string { return "`" }}).Parse(tmpl)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, d)
}
