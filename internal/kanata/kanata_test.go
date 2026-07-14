package kanata

import (
	"encoding/json"
	"testing"
)

func TestReloadMessage_jsonShape(t *testing.T) {
	m := ReloadMessage{Reload: ReloadRequest{Wait: true, TimeoutMs: 5000}}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)
	if got != `{"Reload":{"wait":true,"timeout_ms":5000}}` {
		t.Fatalf("unexpected json: %s", got)
	}
}
