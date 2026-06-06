package staff_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"manager-backend/framework"
	"manager-backend/modules/staff/internal"

	"github.com/stretchr/testify/assert"
)

func TestLogin(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })
	createTestUser(t, "loginuser", "pass123", false)

	r := setupRouter()
	body, _ := json.Marshal(staff.LoginRequest{Account: "loginuser", Password: "pass123"})
	req, _ := http.NewRequest("POST", "/api/v1/staff/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp framework.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "loginuser", data["account"])
	assert.NotEmpty(t, data["token"])
}

func TestLoginWrongPassword(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })
	createTestUser(t, "wrongpwd", "correct", false)

	r := setupRouter()
	body, _ := json.Marshal(staff.LoginRequest{Account: "wrongpwd", Password: "wrong"})
	req, _ := http.NewRequest("POST", "/api/v1/staff/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp framework.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEqual(t, 0, resp.Code)
}

func TestGetCurrentUser(t *testing.T) {
	t.Cleanup(func() { framework.CleanTable("staff_roles", "staff") })
	u := createTestUser(t, "meuser", "pass", false)
	token := getToken(t, u.ID, u.Username)

	r := setupRouter()
	req, _ := http.NewRequest("GET", "/api/v1/staff/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp framework.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "meuser", data["username"])
}

func TestUnauthorizedAccess(t *testing.T) {
	r := setupRouter()
	req, _ := http.NewRequest("GET", "/api/v1/staff/auth/me", nil)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
