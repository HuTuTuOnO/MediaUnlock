package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"server/internal/config"
	"server/internal/models"
)

func setup(t *testing.T) (*gin.Engine, *gorm.DB, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	mkUser(t, db, "admin", "testpass")

	cfg := &config.Config{}
	cfg.JWT.Secret = "test-secret"
	cfg.JWT.ExpireHours = 1
	return New(db, cfg), db, cfg
}

func mkUser(t *testing.T, db *gorm.DB, name, pass string) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	db.Create(&models.User{Username: name, PasswordHash: string(hash)})
}

// decodeData 解开统一响应壳 { code, msg, data },把 data 反序列化到 v。
func decodeData(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v (body=%s)", err, w.Body.String())
	}
	if v != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, v); err != nil {
			t.Fatalf("decode data: %v (data=%s)", err, string(env.Data))
		}
	}
}

// tokenFor 登录并取回 JWT
func tokenFor(t *testing.T, r *gin.Engine, user, pass string) string {
	t.Helper()
	var resp struct {
		Token string `json:"token"`
	}
	decodeData(t, login(t, r, user, pass), &resp)
	if resp.Token == "" {
		t.Fatalf("login %s failed", user)
	}
	return resp.Token
}

// do 发一个带可选 JWT 的请求
func do(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doAgent 发 Agent 接口请求(/api/agent/*)。
// Agent 的 node / client 两种模式统一用 "Token: <token>" 请求头,
// 与管理端的 "Authorization: Bearer <JWT>" 区分开。
func doAgent(t *testing.T, r *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Token", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func login(t *testing.T, r *gin.Engine, user, pass string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLoginSuccess(t *testing.T) {
	r, _, _ := setup(t)
	w := login(t, r, "admin", "testpass")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	decodeData(t, w, &resp)
	if resp.Token == "" {
		t.Fatal("expected token in response")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	r, _, _ := setup(t)
	w := login(t, r, "admin", "wrong")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestMeWithToken(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	w := do(t, r, http.MethodGet, "/api/userinfo", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var me struct {
		UserID uint `json:"user_id"`
	}
	decodeData(t, w, &me)
	if me.UserID == 0 {
		t.Fatalf("want user_id, got %d", me.UserID)
	}
}

func TestMeWithoutToken(t *testing.T) {
	r, _, _ := setup(t)
	req := httptest.NewRequest(http.MethodGet, "/api/userinfo", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
