package apperr

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 构造器应正确设置 Kind 与消息，且 Error() 返回原始消息。
func TestConstructorsSetKindAndMsg(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		kind Kind
	}{
		{"BadRequest", BadRequest("x"), KindBadRequest},
		{"NotFound", NotFound("x"), KindNotFound},
		{"Conflict", Conflict("x"), KindConflict},
		{"Validation", Validation("x"), KindValidation},
		{"Unauthorized", Unauthorized("x"), KindUnauthorized},
		{"Forbidden", Forbidden("x"), KindForbidden},
		{"Internal", Internal("x"), KindInternal},
		{"PayloadTooLarge", PayloadTooLarge("x"), KindPayloadTooLarge},
		{"Upstream", Upstream("x"), KindUpstream},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.kind, c.err.Kind)
			assert.Equal(t, "x", c.err.Msg)
			assert.Equal(t, "x", c.err.Error())
		})
	}
}

func TestNew(t *testing.T) {
	e := New(KindConflict, "dup")
	assert.Equal(t, KindConflict, e.Kind)
	assert.Equal(t, "dup", e.Error())
}

// KindOf 应识别 apperr（含被 wrap 的），其它错误归为 Internal。
func TestKindOf(t *testing.T) {
	assert.Equal(t, KindNotFound, KindOf(NotFound("missing")))
	assert.Equal(t, KindInternal, KindOf(errors.New("plain")), "非 apperr → Internal")
	assert.Equal(t, KindInternal, KindOf(nil), "nil → Internal")

	wrapped := fmt.Errorf("上下文: %w", Forbidden("no"))
	assert.Equal(t, KindForbidden, KindOf(wrapped), "被 wrap 的 apperr 仍可识别")
}

// HTTPStatus 应把每个 Kind 映射到正确的 HTTP 状态码，未知 Kind 兜底 500。
func TestHTTPStatus(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, HTTPStatus(KindBadRequest))
	assert.Equal(t, http.StatusNotFound, HTTPStatus(KindNotFound))
	assert.Equal(t, http.StatusConflict, HTTPStatus(KindConflict))
	assert.Equal(t, http.StatusUnprocessableEntity, HTTPStatus(KindValidation))
	assert.Equal(t, http.StatusUnauthorized, HTTPStatus(KindUnauthorized))
	assert.Equal(t, http.StatusForbidden, HTTPStatus(KindForbidden))
	assert.Equal(t, http.StatusInternalServerError, HTTPStatus(KindInternal))
	assert.Equal(t, http.StatusServiceUnavailable, HTTPStatus(KindUnavailable))
	assert.Equal(t, http.StatusRequestEntityTooLarge, HTTPStatus(KindPayloadTooLarge))
	assert.Equal(t, http.StatusBadGateway, HTTPStatus(KindUpstream))
	assert.Equal(t, http.StatusInternalServerError, HTTPStatus(Kind(999)), "未知 Kind → 500")
}
