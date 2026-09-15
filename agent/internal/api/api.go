package api

// 封装对 Server 的 REST 调用 + "Token: <token>" 鉴权头,并统一解开响应壳 {code,msg,data}。
// 对应 server 端 GET /api/agent/node、POST /api/agent/report、GET /api/agent/unlocked。
//
// 鉴权方式:Agent 的两种模式(node / client)统一用 "Token:" 请求头,
// 不用 Authorization: Bearer,也不用 ?token= 查询参数。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	base  string
	token string
	hc    *http.Client
}

func New(base, token string) *Client {
	return &Client{
		base:  base,
		token: token,
		hc:    &http.Client{Timeout: 30 * time.Second},
	}
}

// WithHTTPClient 注入自定义 http.Client(便于测试用 RoundTripper 打桩)。
func (c *Client) WithHTTPClient(hc *http.Client) *Client {
	c.hc = hc
	return c
}

// envelope 对应 server 的统一响应壳。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// NodeInfo 对应 GET /api/agent/node 的 data(本节点连接信息)。
type NodeInfo struct {
	Alias  string `json:"alias"`
	Type   string `json:"type"` // socks5 | http
	Port   int    `json:"port"`
	Value1 string `json:"value1"` // 账号
	Value2 string `json:"value2"` // 密码
	Value3 string `json:"value3"`
	Value4 string `json:"value4"`
	Value5 string `json:"value5"`
	Value6 string `json:"value6"`
}

// ReportItem 对应 POST /api/agent/report 的单条检测结果。
type ReportItem struct {
	Name   string `json:"name"` // 平台名,与 platforms.name 匹配
	Status int    `json:"status"`
	Region string `json:"region,omitempty"`
	Info   string `json:"info,omitempty"`
	Err    string `json:"err,omitempty"`
}

// Status 启用状态:1=启用 / 0=禁用。
const (
	StatusDisabled = 0
	StatusEnabled  = 1
)

// UnlockedNode 下发的单个节点:连接信息 + 启用状态。
type UnlockedNode struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Value1   string `json:"value1"`
	Value2   string `json:"value2"`
	Value3   string `json:"value3"`
	Value4   string `json:"value4"`
	Value5   string `json:"value5"`
	Value6   string `json:"value6"`
	UploadAt string `json:"upload_at,omitempty"`
	Status   *int   `json:"status,omitempty"`
}

type UnlockedPlatform struct {
	Aliases []string `json:"alias"`
	Rules   []string `json:"rules"`
	Status  *int     `json:"status,omitempty"`
}

// UnlockedData 对应 GET /api/agent/unlocked 的 data,保持 Server 下发的原始形状。
type UnlockedData struct {
	Nodes     map[string]UnlockedNode     `json:"node"`
	Platforms map[string]UnlockedPlatform `json:"platform"`
}

func (c *Client) do(method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return err
	}
	// Agent(node / client 两种模式)统一用 "Token: <token>" 头
	req.Header.Set("Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s %s: bad response (http %d): %s", method, path, resp.StatusCode, string(raw))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: http %d: %s", method, path, resp.StatusCode, env.Msg)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("%s %s: decode data: %w", method, path, err)
		}
	}
	return nil
}

// Node 拉本节点连接信息(server 模式)。
func (c *Client) Node() (*NodeInfo, error) {
	var ni NodeInfo
	if err := c.do(http.MethodGet, "/api/agent/node", nil, &ni); err != nil {
		return nil, err
	}
	return &ni, nil
}

// Report 上报全平台检测结果(server 模式)。
func (c *Client) Report(items []ReportItem) error {
	return c.do(http.MethodPost, "/api/agent/report", map[string]any{"results": items}, nil)
}

// Unlocked 拉下发数据(client 模式)
func (c *Client) Unlocked() (*UnlockedData, error) {
	var ud UnlockedData
	if err := c.do(http.MethodGet, "/api/agent/unlocked", nil, &ud); err != nil {
		return nil, err
	}
	return &ud, nil
}
