package midplat

import (
	"encoding/json"
	"strings"
	"testing"
)

// install-by-url 契约：可选字段（appName/fileSize）为空时必须从 JSON 省略，
// 必填字段（downloadUrl/md5/packageName/version）即使为空也必须保留键，否则中台校验报错。
func TestInstallByURLApp_OmitsEmptyOptionalFields(t *testing.T) {
	// 仅填必填字段 → appName / fileSize 应被省略。
	b, err := json.Marshal(InstallByURLApp{
		DownloadURL: "https://example.com/apk/demo.apk",
		MD5:         "0123456789abcdef0123456789abcdef",
		PackageName: "com.example.demo",
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "appName") {
		t.Errorf("appName 为空应省略，实际: %s", s)
	}
	if strings.Contains(s, "fileSize") {
		t.Errorf("fileSize 为空应省略，实际: %s", s)
	}
	for _, want := range []string{
		`"downloadUrl":"https://example.com/apk/demo.apk"`,
		`"md5":"0123456789abcdef0123456789abcdef"`,
		`"packageName":"com.example.demo"`,
		`"version":"1.0.0"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("必填字段缺失 %s，实际: %s", want, s)
		}
	}

	// 填满可选字段 → appName / fileSize 必须保留。
	b2, _ := json.Marshal(InstallByURLApp{AppName: "Demo App", DownloadURL: "https://x", FileSize: "12MB"})
	s2 := string(b2)
	if !strings.Contains(s2, `"appName":"Demo App"`) {
		t.Errorf("appName 非空应保留，实际: %s", s2)
	}
	if !strings.Contains(s2, `"fileSize":"12MB"`) {
		t.Errorf("fileSize 非空应保留，实际: %s", s2)
	}
}

// 请求体顶层契约：cpIds / apps 键名必须与中台一致。
func TestInstallByURLRequest_JSONShape(t *testing.T) {
	b, err := json.Marshal(InstallByURLRequest{
		CpIDs: []string{"cp-aaa-001", "cp-bbb-002"},
		Apps: []InstallByURLApp{{
			DownloadURL: "https://example.com/apk/demo.apk",
			MD5:         "0123456789abcdef0123456789abcdef",
			PackageName: "com.example.demo",
			Version:     "1.0.0",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"cpIds":["cp-aaa-001","cp-bbb-002"]`) {
		t.Errorf("cpIds 形状不符，实际: %s", s)
	}
	if !strings.Contains(s, `"apps":[`) {
		t.Errorf("apps 形状不符，实际: %s", s)
	}
}
