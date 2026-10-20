module github.com/ratel-online/server

go 1.17

// 为了兼容 Go 1.17 部署环境，以下依赖以本地副本形式引入：
// 上游 go.mod 声明的 go 版本高于 1.17（Milky=1.22、cast=1.19、uno=1.18），
// Go 1.21+ 工具链会强制主模块版本 ≥ 依赖版本，因此 vendor 到 third_party 并统一降为 go 1.17。
// 源码本身未使用 1.18+ 语法，功能与上游一致。
replace (
	github.com/Szzrain/Milky-go-sdk => ./third_party/milky-go-sdk
	github.com/feel-easy/uno => ./third_party/uno
	github.com/spf13/cast => ./third_party/cast
)

require (
	github.com/Szzrain/Milky-go-sdk v1.0.1
	github.com/feel-easy/uno v0.0.0-20220721061415-e6a3189cfd70
	github.com/gorilla/websocket v1.5.3
	github.com/ratel-online/core v0.0.0-20250225062905-81b6faff6d25
	github.com/spf13/cast v1.6.0
)

require (
	github.com/awesome-cap/im v0.0.0-20210720090440-7556eb92965d // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.9 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/modern-go/concurrent v0.0.0-20180228061459-e0a39a4cb421 // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c // indirect
)
