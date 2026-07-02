package apkparse

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// O3 回归：readZipEntry 对声明解压大小超上限的条目直接拒绝（不解压），防压缩炸弹打爆内存。
func TestReadZipEntry_RejectsOversizedEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("big.bin")
	require.NoError(t, err)
	_, err = w.Write(make([]byte, 200*1024)) // 200 KiB 条目
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err)

	// max 小于条目声明大小 → 拒绝。
	_, ok := readZipEntry(zr, "big.bin", 64*1024)
	assert.False(t, ok, "声明解压大小超上限的条目应被拒绝（防压缩炸弹）")

	// max 足够 → 正常读取。
	data, ok := readZipEntry(zr, "big.bin", 1<<20)
	require.True(t, ok)
	assert.Len(t, data, 200*1024)
}
