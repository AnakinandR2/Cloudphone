package framework_test

import (
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// pkgInfo 是 `go list -json` 输出中我们关心的字段。
type pkgInfo struct {
	ImportPath string
	Imports    []string
}

// TestModuleBoundaries 是 Modulith 的架构自检（类比 Spring Modulith 的 verify()）：
// 它解析整个模块的依赖图，断言以下边界规则。Go 的 internal 机制已在编译期强制其中
// 大部分，本测试提供可读的失败信息并覆盖编译期之外的分层约束。
//
//  1. 平台层 framework/** 不得依赖任何业务模块 modules/**。
//  2. 任何包不得 import 其他业务模块的 internal 实现（只能经其公开包交互）。
//  3. 业务模块 modules/** 不得 import 应用根（main 包，import path 为 "manager-backend"）。
func TestModuleBoundaries(t *testing.T) {
	const root = "manager-backend"

	out, err := exec.Command("go", "list", "-json", root+"/...").Output()
	if err != nil {
		t.Fatalf("go list 失败: %v", err)
	}

	dec := json.NewDecoder(strings.NewReader(string(out)))
	var violations []string
	for {
		var p pkgInfo
		if err := dec.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("解析 go list 输出失败: %v", err)
		}

		fromMod := moduleOf(p.ImportPath)
		fromFramework := strings.HasPrefix(p.ImportPath, root+"/framework")
		fromModule := strings.HasPrefix(p.ImportPath, root+"/modules/")

		for _, imp := range p.Imports {
			// 规则 1
			if fromFramework && strings.HasPrefix(imp, root+"/modules/") {
				violations = append(violations, p.ImportPath+" → "+imp+"（平台层不得依赖业务模块）")
			}
			// 规则 2
			if toMod := internalModuleOf(imp, root); toMod != "" && toMod != fromMod {
				violations = append(violations, p.ImportPath+" → "+imp+"（跨模块访问 internal，应经公开包）")
			}
			// 规则 3
			if fromModule && imp == root {
				violations = append(violations, p.ImportPath+" → "+imp+"（业务模块不得依赖应用根）")
			}
		}
	}

	if len(violations) > 0 {
		t.Errorf("检测到 %d 处模块边界违规:\n  %s", len(violations), strings.Join(violations, "\n  "))
	}
}

// moduleOf 从 import path 提取业务模块名，例如
// "manager-backend/modules/staff/internal" → "user"；非业务模块返回 ""。
func moduleOf(importPath string) string {
	const marker = "/modules/"
	i := strings.Index(importPath, marker)
	if i < 0 {
		return ""
	}
	rest := importPath[i+len(marker):]
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		return rest[:j]
	}
	return rest
}

// internalModuleOf 若 import path 指向某业务模块的 internal 实现，返回其模块名，否则 ""。
func internalModuleOf(importPath, root string) string {
	if !strings.HasPrefix(importPath, root+"/modules/") {
		return ""
	}
	if !strings.Contains(importPath, "/internal") {
		return ""
	}
	return moduleOf(importPath)
}
