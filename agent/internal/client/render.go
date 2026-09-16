package client

// 生成 soga 的 routes.toml 并写入文件。地址解析和测速在 assign 阶段已经做完,
// 这里只做归并 + 生成文本,不碰网络。详见 需求文档.md 4.3 第 5 点。
//
// 手写文本而不用 toml 库序列化,是为了保住"一行一个规则"的排版 ——
// 一个平台动辄几十条域名,挤成一行没法人工核对。
//
// 转义走 strconv.Quote:它对 BEL/VT 会写成 \a \v,TOML 不认这两个转义 —— 域名规则里不会出现。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// render 按 render.type 分发到具体渲染器,生成配置并原子写入 path。
// 目前只有 soga;以后加渲染器就在这里多一个 case。
func render(t, path string, a assignment) error {
	switch t {
	case "soga":
		out, err := renderSoga(a)
		if err != nil {
			return err
		}
		return writeFile(path, out)
	}
	return fmt.Errorf("不支持的 render.type %q", t)
}

// renderSoga 按节点归并平台(一个节点一个 [[routes]] 块),块内每个平台前插一行
// "# 平台名" 注释便于人工核对;末尾一条 rules=["*"] 的 direct 兜底。
func renderSoga(a assignment) ([]byte, error) {
	names := make([]string, 0, len(a.Platforms))
	for name := range a.Platforms {
		names = append(names, name)
	}
	sort.Strings(names)

	// 平台按它落在哪个节点归并;两层 map 遍历无序,排序才能保证输出稳定
	byAlias := make(map[string][]string, len(a.Platforms))
	for _, name := range names {
		alias := a.Platforms[name].Alias
		byAlias[alias] = append(byAlias[alias], name)
	}
	aliases := make([]string, 0, len(byAlias))
	for alias := range byAlias {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	var b strings.Builder
	b.WriteString("enable=true\n")

	for _, alias := range aliases {
		node, ok := a.Nodes[alias]
		if !ok {
			return nil, fmt.Errorf("平台引用的节点 %q 不在探测结果里", alias)
		}
		fmt.Fprintf(&b, "\n# 路由 %s\n[[routes]]\nrules=[\n", alias)
		for _, name := range byAlias[alias] {
			// 不直接写 name:名字里若带控制字符,会把这一行注释撕开
			fmt.Fprintf(&b, "  # %s\n", func(s string) string { q := strconv.Quote(s); return q[1 : len(q)-1] }(name))
			for _, rule := range a.Platforms[name].Rules {
				fmt.Fprintf(&b, "  %s,\n", strconv.Quote(rule))
			}
		}
		b.WriteString("]\n\n[[routes.Outs]]\n")
		// 出口写哪些键由节点类型决定,各协议字段集不同,不能共用一套
		switch node.Type {
		case "socks5":
			// soga 只认 "socks",写成 "socks5" 会报 unknown out type
			fmt.Fprintf(&b, "type=\"socks\"\nserver=%s\nport=%d\n", strconv.Quote(node.Host), node.Port)
			if node.Value1 != "" || node.Value2 != "" {
				fmt.Fprintf(&b, "username=%s\npassword=%s\n", strconv.Quote(node.Value1), strconv.Quote(node.Value2))
			}
		case "http":
			fmt.Fprintf(&b, "type=\"http\"\nserver=%s\nport=%d\n", strconv.Quote(node.Host), node.Port)
			if node.Value1 != "" || node.Value2 != "" {
				fmt.Fprintf(&b, "username=%s\npassword=%s\n", strconv.Quote(node.Value1), strconv.Quote(node.Value2))
			}
		default:
			// 本版认不出这个类型的出口字段集:该平台域名退回本地直连,不阻断整份配置
			b.WriteString("type=\"direct\"\n")
		}
	}

	// 兜底:没命中上面任何一条路由的流量走本地出站
	b.WriteString("\n[[routes]]\nrules=[\"*\"]\n\n[[routes.Outs]]\ntype=\"direct\"\n")
	return []byte(b.String()), nil
}

// writeFile 原地覆盖写入。不要改成"临时文件 + rename"——soga 用 inotify 监听这个文件,
// watch 绑在 inode 上,rename 换掉 inode 会让内核摘掉 watch,之后怎么改都感知不到。
// 沿用原文件权限,新建时 0644。
func writeFile(path string, b []byte) error {
	mode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	// 目标目录可能还不存在(如 /etc/soga)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, mode)
}
