# AssetHub

轻量图片素材管理系统：按分类与标签组织素材，上传到阿里云 OSS，并生成可复制、可管理的公网访问链接，方便在其它应用和文档中直接引用图片。

源码：[github.com/plushaddict/asset-hub](https://github.com/plushaddict/asset-hub) · 镜像：[`plushaddict/assethub`](https://hub.docker.com/r/plushaddict/assethub)

按**分类**组织素材（每类自带 OSS 对象前缀），支持**标签**筛选；一个素材可挂多种格式文件（png / svg / webp / ico …）。列表支持按创建/更新时间、名称、排序字段排序；格式文件可本地上传或填远程 URL 由服务端拉取。

## 为什么需要 AssetHub

### 直接上传阿里云 OSS 的痛点

- **难找、难管**：文件散落在控制台/客户端，只能靠对象路径翻找，没有「素材名 / 分类 / 标签」这类业务视角。
- **链接不好用**：公网 URL 要自己拼或从控制台复制，文档和应用里一堆死链；换路径、换文件后，引用处要逐个改。
- **格式分散**：同一图标的 png / svg / webp 往往是多个对象，缺少「一个素材挂多种格式」的关联。
- **权限与协作成本高**：给业务同学开控制台权限过大；不给权限又只能反复找人代传、代拷链接。
- **易乱目录**：手工命名/前缀不统一，Bucket 越用越乱，后期清理和复用成本高。

### AssetHub 的优点

- **素材级管理**：分类（对应 OSS 前缀）+ 标签筛选，按业务语义组织，而不是只盯着对象 Key。
- **一键可用的公网链接**：上传后直接生成可复制链接，适合贴进其它应用、文档、设计稿。
- **多格式一体**：同一素材可挂多种后缀，主图与说明可管，引用时按需选格式。
- **轻量可部署**：单二进制 / Docker，SQLite，内嵌管理页，不依赖复杂前端构建。
- **运营友好**：设置页配 OSS、试连 Bucket；业务方拿链接即可用，不必碰控制台。

一句话：**OSS 管存储，AssetHub 管「可引用、可维护的素材链接」**——把「传到哪」变成「用在哪都能稳定找到」。

## 技术栈

- 后端：Go 1.22+ / Gin
- 存储：SQLite（`modernc.org/sqlite`，纯 Go 无 CGO，交叉编译零痛苦）
- 前端：内嵌 HTML + AlpineJS + TailwindCSS（CDN），`go:embed` 打进二进制，无 Node、无前端构建
- 对象存储：阿里云 OSS（`aliyun-oss-go-sdk`）

## 目录结构

```
asset-hub/
├── main.go                     程序入口、路由、go:embed
├── config.yaml                 进程配置（账号 / 端口 / 库路径）
├── internal/
│   ├── config/config.go        配置加载与默认值
│   ├── model/model.go          Asset / Category / Tag / AssetFile
│   ├── repo/                   SQLite 数据层（建表、CRUD）
│   ├── service/                OSS、设置、分类、标签、素材业务
│   └── handler/handler.go      HTTP 接口
└── static/                     登录、列表、详情、设置页（内嵌）
```

## 快速开始

1. （可选）修改 `config.yaml` 里的后台账号 / 密码 / session `secret` / 端口。
2. 安装依赖并运行：

```bash
go mod tidy
go run .
```

3. 浏览器打开 http://127.0.0.1:8080 ，用配置里的账号密码登录（默认见 `config.yaml`）。
4. 打开 **设置**（`/settings`）：
   - **OSS**：填写并保存连接（保存时会试连 Bucket）；密钥在接口响应中脱敏。
   - **分类**：至少新建一个分类并填写 `ossPrefix`（如 `icon/`、`cover/`）。
   - （可选）配置业务 API 的 `business_token`。
5. 首页未完成配置时会显示**引导条**（缺 OSS → 去 OSS 页；缺分类 → 去分类页）。配置就绪后：新建素材（必选分类，可选标签，**可在同一弹窗顺带上传多种格式**）→ 亦可进详情继续添加/管理格式（格式文件可填「说明」便于区分类型）→ 复制链接或走业务 API。

> OSS / 业务 token **只**存在库中，由设置页维护，不写进 yaml。空库启动时 OSS 未就绪，须先在页面保存配置。

## 编译打包（跨平台）

```bash
# 当前平台
go build -o assethub .

# 写入版本号（发布时用 git tag）
go build -ldflags="-X assethub/internal/version.Version=$(git describe --tags --always)" -o assethub .

# macOS arm64（M 系列）
GOOS=darwin GOARCH=arm64 go build -o assethub-darwin-arm64 .

# Linux amd64（云服务器）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o assethub-linux-amd64 .

# Windows amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o assethub.exe .
```

> `config.yaml` 要和二进制放在同一目录；数据库 `assethub.db` 在运行目录自动生成。

## Docker

镜像仓库：`plushaddict/assethub`（Docker Hub）。页面打进二进制（`ASSETHUB_EMBED_STATIC=1`）；SQLite 默认写在卷 `/data/assethub.db`，**镜像内不含业务库**，新卷即空库。

**完整部署教程（拉取运行 / Compose / 群晖 / 自定义配置 / 备份升级 / 反代 / 发版）：见 [deploy/DOCKER.md](deploy/DOCKER.md)。**

本地构建并启动：

```bash
docker compose up --build -d
# 浏览器 http://127.0.0.1:8080  默认 admin / 123456；OSS 等登录后在设置页配置
```

拉取已发布镜像：

```bash
docker pull plushaddict/assethub:latest
docker run -d --name assethub --restart unless-stopped \
  -p 8080:8080 -v assethub-data:/data plushaddict/assethub:latest
```

群晖 / 仅拉取镜像：用 [`deploy/docker-compose.synology.yml`](deploy/docker-compose.synology.yml)（含 `pull_policy: always`），勿直接用根目录带 `build:` 的 `docker-compose.yml`。

清空数据库（删命名卷后重启即空库）：

```bash
docker compose down -v
docker compose up -d
```

发布到 Docker Hub：仓库配置 Secrets `DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN` 后，推送 `v*` tag 即可（详见 [deploy/DOCKER.md](deploy/DOCKER.md) 第九节）。

## 配置说明（config.yaml）

yaml **仅**进程级项（登录账号、端口、库路径）。OSS / 业务 token 在设置页配置。

```yaml
server:
  port: 8080
  username: admin
  password: "123456"
  secret: "session 加密密钥，务必改成随机字符串"
  session_max_age: 86400

database:
  path: "assethub.db"
```

**已废弃**：yaml 中的 `oss`、`business_token`、`path_rule` / 系统级前缀。对象路径前缀仅在**分类**的 `ossPrefix` 上配置。

### 环境变量覆盖（可选）

Docker 或其它部署可用环境变量覆盖登录相关项（**非空才生效**，优先于 yaml）：

| 变量 | 覆盖 | 说明 |
| --- | --- | --- |
| `ASSETHUB_USERNAME` | `server.username` | 可选 |
| `ASSETHUB_PASSWORD` | `server.password` | 可选；生产建议设 |
| `ASSETHUB_SECRET` | `server.secret`（Cookie 签名） | **非必须**；公网/多实例再设，见 [deploy/DOCKER.md](deploy/DOCKER.md) 第 5 节 |

本地 `go run` 同样生效。只改密码：`docker run -e ASSETHUB_PASSWORD='强密码' ...`。
## OSS 对象命名规则

```
{分类.ossPrefix}{assetName}.{suffix}
```

示例：分类前缀 `icon/`、素材名 `logo_book` → `icon/logo_book.png`。素材改名不会移动已上传对象（只影响新上传）；需改路径请删文件后重传。

## 接口一览

### 页面

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/login` | 登录页 |
| GET | `/` | 素材列表（分类/标签筛选、配置引导条） |
| GET | `/asset/:id` | 素材详情（含所有格式文件） |
| GET | `/settings` | 系统设置（OSS、分类、标签） |

### 管理接口（需登录 Cookie）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/login` | 登录，body：`{username,password}` |
| POST | `/api/logout` | 退出 |
| GET | `/api/settings` | OSS 与业务 token 脱敏视图 + `ossReady` / `categoryCount` / `setupComplete` |
| PUT | `/api/settings` | 保存 OSS；body 含 `clearBusinessToken` 可清空业务 token；保存前 `GetBucketInfo` 试连 |
| GET | `/api/setup/status` | 引导条状态：`ossReady`, `ossError`, `categoryCount`, `setupComplete` |
| GET | `/api/categories` | 分类列表 |
| POST | `/api/categories` | 新建 `{name, ossPrefix, sort}` |
| PUT | `/api/categories/:id` | 更新分类 |
| DELETE | `/api/categories/:id` | 删除（分类下仍有素材时拒绝） |
| GET | `/api/tags` | 标签列表 |
| POST | `/api/tags` | 新建 `{name}` |
| PUT | `/api/tags/:id` | 更新标签 |
| DELETE | `/api/tags/:id` | 删除标签 |
| POST | `/api/asset` | 新建 `{assetName, categoryId, tagIds?, tagNames?, remark}` |
| GET | `/api/asset/list` | 分页：`keyword`, `categoryId`, `tagIds`（逗号分隔，**AND**）, `page`, `size`, `sortBy`（`createTime`/`updateTime`/`assetName`/`sort`，默认 `createTime`）, `sortOrder`（`asc`/`desc`，默认 `desc`） |
| GET | `/api/asset/:id` | 素材详情 + `files` + `tags` |
| PUT | `/api/asset/:id` | 修改 `{assetName, categoryId, remark, sort}`；若带 `tagIds` 或 `tagNames` 则**全量替换**标签（空数组即清空） |
| DELETE | `/api/asset/:id` | 删除素材（删 OSS 对象 + 逻辑删库） |
| POST | `/api/asset/:id/files` | 上传，`multipart/form-data`：`file` 与 `sourceUrl` 互斥（二选一）；可选 `remark`（说明，≤200 字）。`sourceUrl` 为远程 http(s) 地址，由服务端拉取后入库 |
| PUT | `/api/asset/:id/files/:fid` | 更新 `{ remark }`（说明，≤200 字） |
| PUT | `/api/asset/:id/files/:fid/primary` | 设为主图 |
| DELETE | `/api/asset/:id/files/:fid` | 删除单个格式文件 |

### 业务查询接口（给 ComiGo 等业务）

```http
GET /api/business/asset?assetName=book_cover_001
GET /api/business/asset?assetId=12
Header: X-Asset-Token: <business_token>   # 或 query token=；未配置 business_token 时不校验
```

响应示例：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "assetId": 12,
    "assetName": "book_cover_001",
    "categoryId": 3,
    "categoryName": "书籍封面",
    "tags": [
      { "id": 1, "name": "comigo" }
    ],
    "remark": "书籍id:1001",
    "fileList": [
      { "id": 1, "suffix": "png", "width": 800, "height": 1200, "url": "https://...", "isPrimary": 1 },
      { "id": 2, "suffix": "webp", "width": 800, "height": 1200, "url": "https://...", "isPrimary": 0 }
    ]
  }
}
```

列表/详情中的素材字段包括：`categoryId`、`categoryName`、`tags[]`、`primaryUrl`、`fileCount` 等（已移除 `assetType`）。

## 设计要点

- **一个素材 = 一条 asset + N 条 asset_file**；同素材下后缀唯一，重复上传同后缀会被拦截。
- **分类**决定 OSS 前缀；**标签**多对多，列表筛选用 `tagIds` 做 AND。
- 首个上传文件自动为主图；删主图后提升剩余第一个。
- 删除素材时先删 OSS 再逻辑删库；`del_flag` 软删可追溯。
- 私有 Bucket 不落库长期 URL，读取时实时签名。
- OSS 未就绪时进程仍启动；上传/保存设置会提示先配置 OSS。

## RAM 权限建议

对象读写（按实际分类前缀缩小 `Resource`）：

```json
{
  "Version": "1",
  "Statement": [{
    "Effect": "Allow",
    "Action": ["oss:PutObject", "oss:DeleteObject", "oss:GetObject"],
    "Resource": ["acs:oss:*:*:your-bucket/icon/*", "acs:oss:*:*:your-bucket/cover/*"]
  }]
}
```

在**设置页保存 OSS** 时还会调用 `GetBucketInfo` 试连（当前 SDK 无 HeadBucket）。若保存总是失败而对象上传正常，请为 RAM 用户增加 **`oss:GetBucketInfo`**（可对 `acs:oss:*:*:your-bucket` 或账号下 bucket 列表授权）。
