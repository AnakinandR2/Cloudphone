package billing

import "testing"

func TestInstanceMetaProvider_NilReturnsEmpty(t *testing.T) {
	t.Cleanup(func() { SetInstanceMetaProvider(nil) })
	SetInstanceMetaProvider(nil)
	got := lookupInstanceMeta([]string{"cp-1"})
	if len(got) != 0 {
		t.Errorf("nil provider should yield empty map, got %v", got)
	}
}

func TestInstanceMetaProvider_RegisteredEnriches(t *testing.T) {
	t.Cleanup(func() { SetInstanceMetaProvider(nil) })
	SetInstanceMetaProvider(func(cpIDs []string) map[string]InstanceMeta {
		out := map[string]InstanceMeta{}
		for _, id := range cpIDs {
			if id == "cp-1" {
				out[id] = InstanceMeta{Name: "手机A", Status: "RUNNING"}
			}
		}
		return out
	})
	got := lookupInstanceMeta([]string{"cp-1", "cp-x"})
	if got["cp-1"].Name != "手机A" || got["cp-1"].Status != "RUNNING" {
		t.Errorf("cp-1 = %+v, want 手机A/RUNNING", got["cp-1"])
	}
	if _, ok := got["cp-x"]; ok {
		t.Errorf("cp-x should be absent")
	}
}

func TestInstanceMetaProvider_EmptyInputSkipsProvider(t *testing.T) {
	t.Cleanup(func() { SetInstanceMetaProvider(nil) })
	called := false
	SetInstanceMetaProvider(func(cpIDs []string) map[string]InstanceMeta {
		called = true
		return nil
	})
	lookupInstanceMeta(nil)
	if called {
		t.Errorf("provider should not be called for empty cpIDs")
	}
}
