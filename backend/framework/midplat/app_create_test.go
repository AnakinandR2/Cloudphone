package midplat

import (
	"encoding/json"
	"strings"
	"testing"
)

// 秒传命中时 UploadID=0 必须从 JSON 省略，否则中台按编号 0 找上传记录 → 「上传记录不存在或文件路径为空」。
func TestCreateFromUploadedFileRequest_OmitsZeroUploadID(t *testing.T) {
	// 秒传：uploadId=0 → 省略，靠 md5 定位。
	b, err := json.Marshal(CreateFromUploadedFileRequest{AppName: "微信", MD5: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "uploadId") {
		t.Errorf("秒传请求不应包含 uploadId，实际: %s", b)
	}
	if !strings.Contains(string(b), `"md5":"abc"`) {
		t.Errorf("秒传请求应保留 md5，实际: %s", b)
	}

	// 普通上传：uploadId 非 0 → 必须保留。
	b2, _ := json.Marshal(CreateFromUploadedFileRequest{UploadID: 12345, AppName: "微信"})
	if !strings.Contains(string(b2), `"uploadId":12345`) {
		t.Errorf("普通上传请求应包含 uploadId，实际: %s", b2)
	}
}
