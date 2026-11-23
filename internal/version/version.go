package version

// Version 由构建 -ldflags -X assethub/internal/version.Version=... 注入。
// 默认与当前发布版本对齐；发新版时请同步改此处，或始终用 ldflags 覆盖。
var Version = "v0.0.4"

const Author = "blackcat"
