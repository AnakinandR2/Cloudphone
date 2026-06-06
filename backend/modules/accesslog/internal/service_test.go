package accesslog

import (
	"fmt"
	"testing"
	"time"

	"manager-backend/framework"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedAccessLogs(t *testing.T, count int) {
	t.Helper()
	items := make([]AccessLog, count)
	for i := range items {
		items[i] = AccessLog{
			UserID:     uint(i%3 + 1),
			Username:   fmt.Sprintf("user%d", i%3+1),
			Method:     []string{"GET", "POST", "PUT", "DELETE"}[i%4],
			Path:       fmt.Sprintf("/api/v1/test/%d", i),
			StatusCode: []int{200, 201, 400, 500}[i%4],
			LatencyMs:  10 + i,
			ClientIP:   "127.0.0.1",
			CreatedAt:  time.Now().Add(-time.Duration(count-i) * time.Minute),
		}
	}
	framework.DB.CreateInBatches(items, 50)
}

func TestAccessLogGetList(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 25)

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(25), total)
	assert.Len(t, list, 10)
}

func TestAccessLogGetListPagination(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 25)

	list, _, err := AccessLogService.GetList(ListQuery{Page: 3, Size: 10})
	require.NoError(t, err)
	assert.Len(t, list, 5)
}

func TestAccessLogGetListFilterUsername(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 12)

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 50, Username: "user1"})
	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	for _, item := range list {
		assert.Contains(t, item.Username, "user1")
	}
}

func TestAccessLogGetListFilterMethod(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 12)

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 50, Method: "POST"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	for _, item := range list {
		assert.Equal(t, "POST", item.Method)
	}
}

func TestAccessLogGetListFilterPath(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 12)

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 50, Path: "/api/v1/test/1"})
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
	for _, item := range list {
		assert.Contains(t, item.Path, "/api/v1/test/1")
	}
}

func TestAccessLogGetListFilterStatusGroup(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 12)

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 50, StatusGroup: "4xx"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	for _, item := range list {
		assert.GreaterOrEqual(t, item.StatusCode, 400)
		assert.Less(t, item.StatusCode, 500)
	}
}

func TestAccessLogGetListFilterTimeRange(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 20)

	start := time.Now().Add(-10 * time.Minute).Format("2006-01-02 15:04:05")
	end := time.Now().Format("2006-01-02 15:04:05")

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 50, StartTime: start, EndTime: end})
	require.NoError(t, err)
	assert.Greater(t, total, int64(0))
	assert.GreaterOrEqual(t, int(total), len(list))
}

func TestAccessLogGetListSort(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	seedAccessLogs(t, 5)

	list, _, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 10, Order: "latency_ms", Sort: "ascending"})
	require.NoError(t, err)
	require.Greater(t, len(list), 1)
	assert.LessOrEqual(t, list[0].LatencyMs, list[1].LatencyMs)
}

func TestAccessLogGetListFilterScope(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })
	now := time.Now()
	require.NoError(t, logRepo.createBatch([]AccessLog{
		{Scope: "staff", Username: "admin", Method: "GET", Path: "/u", CreatedAt: now},
		{Scope: "user", Username: "13800138000", Method: "GET", Path: "/c1", CreatedAt: now},
		{Scope: "user", Username: "13800138001", Method: "GET", Path: "/c2", CreatedAt: now},
		{Scope: "anonymous", Method: "POST", Path: "/login", CreatedAt: now},
	}))

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 50, Scope: "user"})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	for _, item := range list {
		assert.Equal(t, "user", item.Scope)
	}
}

func TestAccessLogGetListEmpty(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })

	list, total, err := AccessLogService.GetList(ListQuery{Page: 1, Size: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, list)
}

func TestAccessLogGetByID(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("access_logs") })

	entry := AccessLog{
		UserID:          1,
		Username:        "admin",
		Method:          "GET",
		Path:            "/api/v1/test",
		StatusCode:      200,
		LatencyMs:       42,
		ClientIP:        "127.0.0.1",
		RequestHeaders:  `{"Content-Type":"application/json"}`,
		RequestBody:     `{"key":"value"}`,
		ResponseHeaders: `{"Content-Type":"application/json"}`,
		ResponseBody:    `{"code":0,"message":"成功"}`,
		CreatedAt:       time.Now(),
	}
	framework.DB.Create(&entry)

	found, err := AccessLogService.GetByID(int(entry.ID))
	require.NoError(t, err)
	assert.Equal(t, entry.ID, found.ID)
	assert.Equal(t, "GET", found.Method)
	assert.Equal(t, `{"key":"value"}`, found.RequestBody)
	assert.NotEmpty(t, found.RequestHeaders)
	assert.NotEmpty(t, found.ResponseHeaders)
}

func TestAccessLogGetByIDNotFound(t *testing.T) {
	_, err := AccessLogService.GetByID(999999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}
