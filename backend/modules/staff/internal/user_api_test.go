package staff_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserCRUD(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })
	admin := createTestUser(t, "admin", "admin123", true)
	token := getToken(t, admin.ID, admin.Username)
	r := setupRouter()

	body, _ := json.Marshal(staff.StaffCreate{Username: "newuser", Password: "pwd123", IsActive: true})
	req, _ := http.NewRequest("POST", "/api/v1/staff/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var createResp framework.Response
	json.Unmarshal(w.Body.Bytes(), &createResp)
	assert.Equal(t, 0, createResp.Code)
	userData := createResp.Data.(map[string]interface{})
	userID := int(userData["id"].(float64))

	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/staff/%d", userID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	boolTrue := true
	updateBody, _ := json.Marshal(staff.StaffUpdate{Username: "updateduser", IsActive: &boolTrue, IsSuperuser: &boolTrue})
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/api/v1/staff/update/%d", userID), bytes.NewBuffer(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var updateResp framework.Response
	json.Unmarshal(w.Body.Bytes(), &updateResp)
	assert.Equal(t, 0, updateResp.Code)
	assert.Equal(t, "updateduser", updateResp.Data.(map[string]interface{})["username"])

	req, _ = http.NewRequest("DELETE", fmt.Sprintf("/api/v1/staff/delete/%d", userID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	req, _ = http.NewRequest("GET", fmt.Sprintf("/api/v1/staff/%d", userID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUserListPagination(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })
	admin := createTestUser(t, "admin", "admin123", true)
	token := getToken(t, admin.ID, admin.Username)
	r := setupRouter()

	for i := 0; i < 5; i++ {
		createTestUser(t, fmt.Sprintf("page_%d", i), "p", false)
	}

	req, _ := http.NewRequest("GET", "/api/v1/staff/list?page=1&size=3", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp framework.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(6), data["total"])
	assert.Len(t, data["list"].([]interface{}), 3)
}

func TestNonSuperuserForbidden(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })
	u := createTestUser(t, "normaluser", "pass", false)
	token := getToken(t, u.ID, u.Username)
	r := setupRouter()

	req, _ := http.NewRequest("GET", "/api/v1/staff/list", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
