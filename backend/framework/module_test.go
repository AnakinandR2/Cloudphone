package framework_test

import (
	"testing"

	"manager-backend/framework"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type stubModule struct {
	name        string
	initCalled  bool
	startCalled bool
}

func (m *stubModule) Name() string                                             { return m.name }
func (m *stubModule) Init(db *gorm.DB) error                                   { m.initCalled = true; return nil }
func (m *stubModule) RegisterRoutes(r *gin.RouterGroup, mw ...gin.HandlerFunc) {}
func (m *stubModule) OnStart() error                                           { m.startCalled = true; return nil }
func (m *stubModule) OnStop() error                                            { return nil }

func TestModuleRegistry(t *testing.T) {
	reg := framework.NewModuleRegistry()
	assert.Empty(t, reg.GetAll())

	m1 := &stubModule{name: "m1"}
	m2 := &stubModule{name: "m2"}
	reg.Register(m1)
	reg.Register(m2)

	all := reg.GetAll()
	assert.Len(t, all, 2)
	assert.Equal(t, "m1", all[0].Name())
	assert.Equal(t, "m2", all[1].Name())
}

func TestModuleInit(t *testing.T) {
	m := &stubModule{name: "test"}
	m.Init(nil)
	assert.True(t, m.initCalled)
}

func TestModuleOnStart(t *testing.T) {
	m := &stubModule{name: "test"}
	m.OnStart()
	assert.True(t, m.startCalled)
}
