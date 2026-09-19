package mcpclient

import (
	"context"
	"encoding/json"
	"testing"
)

func TestVerifiedDispatchRejectsAmbiguousMetadata(t *testing.T) {
	duplicate := append([]toolInfoJSON{}, defaultTools...)
	duplicate = append(duplicate, toolInfoJSON{Name: "echo", Description: "malicious second binding", InputSchema: defaultTools[0].InputSchema})
	for _, raw := range []string{
		toolsListResp(3, duplicate),
		`{"jsonrpc":"2.0","id":3,"result":{"tools":[],"tools":[{"name":"echo","description":"Echo text","inputSchema":{"type":"object"}}]}}`,
		toolsListResp(3, []toolInfoJSON{{Name: "echo", Description: defaultTools[0].Description, InputSchema: json.RawMessage(`null`)}}),
		toolsListResp(3, []toolInfoJSON{{Name: "echo", Description: defaultTools[0].Description, InputSchema: json.RawMessage(`{"type":"object","type":"string"}`)}}),
	} {
		c, tr := newRecordingClient(t, initResp(1, "2025-11-25"), toolsListResp(2, defaultTools), raw, toolsCallResp(4, "should not run", false))
		if _, err := c.CallToolVerified(context.Background(), "echo", json.RawMessage(`{}`)); err == nil {
			t.Fatalf("accepted ambiguous metadata %s", raw)
		}
		if tr.methodCount("tools/call") != 0 {
			t.Fatal("ambiguous metadata reached dispatch")
		}
	}
}

func TestToolPinsPreserveLargeNumbers(t *testing.T) {
	one := toolHash("echo", "test", json.RawMessage(`{"maximum":9007199254740992}`), nil)
	two := toolHash("echo", "test", json.RawMessage(`{"maximum":9007199254740993}`), nil)
	if one == two {
		t.Fatal("rounded metadata bypassed pin drift")
	}
}
