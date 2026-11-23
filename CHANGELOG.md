# 更新日志

本文件记录 AssetHub 各 **git tag** 对应的用户可见变更。  
约定：打正式版本 tag（如 `v1.2.0`）时，在本文档**最上方**追加一节，再推送 tag。  
推送 `v*` tag 后，GitHub Actions 会推送 Docker 镜像，并按本节内容创建 / 更新对应 [GitHub Release](https://github.com/plushaddict/asset-hub/releases)。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

镜像：[`plushaddict/assethub`](https://hub.docker.com/r/plushaddict/assethub)（tag 与 git tag 对齐）。

---

## [v1.2.0] - 2026-11-23

### 新增

- 素材 **显示名**（`displayName`，可中文）与 **资源名**（`assetName`，OSS / 业务查询键）分离；新建时可由显示名拼音预填资源名，冲突自动加 `_2`、`_3`…；列表关键字同时匹配显示名与资源名。
- 设置页 **公共** Tab：本账号 OSS 链接可 **直登原对象** 或 **按规则重传**（可选重传后删原）；删除素材 / 文件是否同步删 OSS 由 `deleteRemoveOss` 控制，不再弹窗征求。
- 素材列表支持 **卡片 / 列表** 视图切换；整卡可点进详情；偏好记入本机 `localStorage`（`assethub.listViewMode`）。

### 变更

- 格式文件填远程 URL：识别为本账号 Bucket 时按公共策略分流；外链仍下载后入库。
- 管理端列表 / 详情 / 新建主标题改为显示名；删除确认文案指向「设置 → 公共」。
- README 同步命名规则、公共设置与接口字段说明。

---

## [v1.1.0] - 2026-11-23

### 新增

- Docker 部署可用环境变量覆盖登录账号 / 密码 / session secret：`ASSETHUB_USERNAME`、`ASSETHUB_PASSWORD`、`ASSETHUB_SECRET`（`SECRET` 可选），不必再挂载自定义 config。
- 文档与 Compose 示例（含群晖）同步说明上述环境变量。

---

## [v1.0.0] - 2026-11-23

首个正式版。

### 新增

- 添加 MIT `LICENSE`。
- 格式文件支持填远程 URL，由服务端拉取后上传 OSS（`remote_fetch`）。
- 群晖部署示例：`deploy/docker-compose.synology.yml`。

### 变更

- 管理端列表 / 详情配合远程拉取与相关交互调整。
- Docker 部署文档整理。

---

## [v0.0.4] - 2026-11-23

### 新增

- 全站固定底栏展示作者 `blackcat` 与构建版本号。
- 公开接口 `GET /api/version`。
- Docker / GitHub Actions 通过 ldflags 注入 git tag 作为版本。

---

## [v0.0.3] - 2026-11-23

### 新增

- 管理端新建素材时可在同一弹窗多选上传多种格式。
- `asset_file` 支持可选「说明」（管理端可填可改；业务 API 不返回）。
- `deploy/DOCKER.md` Docker 部署说明。

### 变更

- yaml 仅保留进程配置；OSS 只由设置页维护，不再从配置文件种子写入。

---

## [v0.0.2] - 2026-11-21

与 [v0.0.1](#v001---2026-11-21) 指向同一提交（初始化里程碑再打 tag）。

---

## [v0.0.1] - 2026-11-21

### 新增

- 初始化 AssetHub：分类 / 标签 / 素材管理，多格式文件挂接。
- 阿里云 OSS 上传与公网链接；设置页配置 OSS、试连 Bucket。
- 管理端登录、列表、详情、设置页（内嵌 HTML）。
- SQLite 存储；单二进制 / Docker 部署；GitHub Actions 推送 Docker Hub。
