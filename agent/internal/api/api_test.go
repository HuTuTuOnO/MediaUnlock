package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// rtFunc 让我们用函数充当 http.RoundTripper,免去起监听端口(沙箱不允许 bind)。
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestGetNode(t *testing.T) {
	var gotAuth, gotPath string
	c := New("https://panel", "node-tok").WithHTTPClient(&http.Client{
		Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			gotAuth = r.Header.Get("Token")
			gotPath = r.URL.Path
			return jsonResp(200, `{"code":200,"msg":"success","data":{"alias":"jp1","type":"socks5","port":1080,"value1":"u","value2":"p"}}`), nil
		}),
	})
	ni, err := c.Node()
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "node-tok" {
		t.Fatalf("Token header = %q, want %q", gotAuth, "node-tok")
	}
	if gotPath != "/api/agent/node" {
		t.Fatalf("path = %q", gotPath)
	}
	if ni.Type != "socks5" || ni.Port != 1080 || ni.Value1 != "u" || ni.Value2 != "p" {
		t.Fatalf("unexpected node: %+v", ni)
	}
}

// Server 全量下发:node 与 platform 各自带 status,Agent 必须能解析出来才能判断跳过。
func TestUnlockedParsesStatus(t *testing.T) {
	body := `{"code":200,"msg":"success","data":{
		"node":{"jp1":{"host":"h","port":1080,"type":"socks5","status":0}},
		"platform":{"Netflix":{"alias":["jp1"],"rules":["domain:netflix.com"],"status":1}}
	}}`
	c := New("https://panel", "tok").WithHTTPClient(&http.Client{
		Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResp(200, body), nil
		}),
	})
	ups, err := c.Unlocked()
	if err != nil {
		t.Fatalf("Unlocked: %v", err)
	}
	up, ok := ups.Platforms["Netflix"]
	if !ok || len(ups.Platforms) != 1 {
		t.Fatalf("platforms = %+v", ups.Platforms)
	}
	if up.Status == nil || *up.Status != StatusEnabled {
		t.Fatalf("platform status = %v, want %d", up.Status, StatusEnabled)
	}
	if len(up.Aliases) != 1 || up.Aliases[0] != "jp1" {
		t.Fatalf("aliases = %+v", up.Aliases)
	}
	n, ok := ups.Nodes["jp1"]
	if !ok || len(ups.Nodes) != 1 {
		t.Fatalf("nodes = %+v", ups.Nodes)
	}
	// 节点的 alias 就是 map 的 key(下发体里没有 alias 字段,也不该有)
	if n.Status == nil || *n.Status != StatusDisabled {
		t.Fatalf("node status = %v, want %d", n.Status, StatusDisabled)
	}
}

func TestReportSendsResults(t *testing.T) {
	var body map[string]any
	c := New("https://panel", "tok").WithHTTPClient(&http.Client{
		Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			json.Unmarshal(raw, &body)
			return jsonResp(200, `{"code":200,"msg":"success","data":{"accepted":1,"dropped":0}}`), nil
		}),
	})
	if err := c.Report([]ReportItem{{Name: "Netflix", Status: 1, Region: "JP"}}); err != nil {
		t.Fatal(err)
	}
	results, ok := body["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("results not sent: %+v", body)
	}
}

func TestGetUnlocked(t *testing.T) {
	var gotAuth, gotPath string
	c := New("https://panel", "tok").WithHTTPClient(&http.Client{
		Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			gotAuth = r.Header.Get("Token")
			gotPath = r.URL.Path
			return jsonResp(200, `{"code":200,"msg":"success","data":{"node":{"jp1":{"type":"socks5","host":"1.2.3.4","port":1080}},"platform":{"Netflix":{"alias":["jp1"],"rules":["domain:netflix.com"]}}}}`), nil
		}),
	})
	out, err := c.Unlocked()
	if err != nil {
		t.Fatal(err)
	}
	// client 模式也走 "Token:" 头,不再拼 ?token=
	if gotAuth != "tok" {
		t.Fatalf("Token header = %q, want %q", gotAuth, "tok")
	}
	if gotPath != "/api/agent/unlocked" {
		t.Fatalf("path = %q, want /api/agent/unlocked", gotPath)
	}
	// Unlocked 只解码:原样保留 server 下发的两张 map,不做重组(不改 key、不摊平 alias)
	up, ok := out.Platforms["Netflix"]
	if !ok || len(out.Platforms) != 1 || len(up.Aliases) != 1 || up.Aliases[0] != "jp1" {
		t.Fatalf("unexpected platforms: %+v", out.Platforms)
	}
	if n, ok := out.Nodes["jp1"]; !ok || len(out.Nodes) != 1 || n.Host != "1.2.3.4" || n.Port != 1080 {
		t.Fatalf("unexpected nodes: %+v", out.Nodes)
	}
}

func TestErrorStatusSurfacesMsg(t *testing.T) {
	c := New("https://panel", "bad").WithHTTPClient(&http.Client{
		Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResp(401, `{"code":401,"msg":"invalid node token"}`), nil
		}),
	})
	_, err := c.Node()
	if err == nil || !strings.Contains(err.Error(), "invalid node token") {
		t.Fatalf("want error carrying msg, got %v", err)
	}
}
