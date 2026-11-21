# AssetHub

轻量图片素材资产管理系统：按**分类**组织素材（每类自带 OSS 对象前缀），支持**标签**筛选；一个素材可挂多种格式文件（png / svg / webp / ico …），上传到阿里云 OSS 并生成可复制的访问链接，同时对外提供查询 API 给业务方使用。

## 技术栈

- 后端：Go 1.22+ / Gin
- 存储：SQLite（`modernc.org/sqlite`，纯 Go 无 CGO，交叉编译零痛苦）
- 前端：内嵌 HTML + AlpineJS + TailwindCSS（CDN），`go:embed` 打进二进制，无 Node、无前端构建
- 对象存储：阿里云 OSS（`aliyun-oss-go-sdk`）

## 目录结构

```
asset-hub/
├── main.go                     程序入口、路由、go:embed
├── config.yaml                 首次部署种子（账号 / OSS / 端口）
├── internal/
│   ├── config/config.go        配置加载与默认值
│   ├── model/model.go          Asset / Category / Tag / AssetFile
│   ├── repo/                   SQLite 数据层（建表、CRUD）
│   ├── service/                OSS、设置、分类、标签、素材业务
│   └── handler/handler.go      HTTP 接口
└── static/                     登录、列表、详情、设置页（内嵌）
```

## 快速开始

1. 修改 `config.yaml`：填 OSS 的 `endpoint / bucket / access_key_id / access_key_secret / url_prefix`，以及后台登录账号（可选 `business_token`）。
2. 安装依赖并运行：

```bash
go mod tidy
go run .
```

3. 浏览器打开 http://127.0.0.1:8080 ，用配置里的账号密码登录。
4. 打开 **设置**（`/settings`）：
   - **OSS**：核对或保存连接（保存时会试连 Bucket）；密钥在接口响应中脱敏。
   - **分类**：至少新建一个分类并填写 `ossPrefix`（如 `icon/`、`cover/`）。
5. 首页未完成配置时会显示**引导条**（缺 OSS → 去 OSS 页；缺分类 → 去分类页）。配置就绪后：新建素材（必选分类，可选标签）→ 进详情上传各格式 → 复制链接或走业务 API。

> 首次启动：若库中尚无 `system_settings` 行，会用 yaml 里的 `oss` 与 `server.business_token` 写入种子；之后以页面设置为准，改 yaml 不会自动覆盖已有库配置。启动日志会打印 `ossReady=`（仅表示 OSS 客户端构造成功，**不**等价于 Bucket 试连通过）。

## 编译打包（跨平台）

```bash
# 当前平台
go build -o assethub .

# macOS arm64（M 系列）
GOOS=darwin GOARCH=arm64 go build -o assethub-darwin-arm64 .

# Linux amd64（云服务器）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o assethub-linux-amd64 .

# Windows amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o assethub.exe .
```

> `config.yaml` 要和二进制放在同一目录；数据库 `assethub.db` 在运行目录自动生成。

## Docker

镜像仓库：`blackcatzoo/assethub`（Docker Hub）。页面打进二进制（`ASSETHUB_EMBED_STATIC=1`）；SQLite 默认写在卷 `/data/assethub.db`，**镜像内不含业务库**，新卷即空库。

本地构建并启动：

```bash
docker compose up --build -d
# 浏览器 http://127.0.0.1:8080  默认账号见 config.docker.yaml / 挂载的 config.yaml
```

清空数据库（删命名卷后重启即空库）：

```bash
docker compose down -v
docker compose up -d
```

可选：挂载宿主机配置覆盖容器内默认配置（在 `docker-compose.yml` 取消注释）：

```yaml
volumes:
  - assethub-data:/data
  - ./config.yaml:/app/config.yaml:ro
```

### 发布到 Docker Hub（与 TabLink 相同流程）

仓库需配置 GitHub Actions Secrets：

| Secret | 说明 |
| --- | --- |
| `DOCKERHUB_USERNAME` | 如 `blackcatzoo` |
| `DOCKERHUB_TOKEN` | Docker Hub Access Token |

推送 `v*` tag 触发 `.github/workflows/docker-publish.yml`，构建 `linux/amd64` 并推送：

- `blackcatzoo/assethub:<version>`（去掉 `v` 前缀，如 `1.0.0`）
- `blackcatzoo/assethub:latest`

```bash
git tag v1.0.0
git push origin v1.0.0
```

拉取运行：

```bash
docker pull blackcatzoo/assethub:latest
docker run -d --name assethub -p 8080:8080 -v assethub-data:/data blackcatzoo/assethub:latest
```

## 配置说明（config.yaml）

```yaml
server:
  port: 8080
  username: admin
  password: "123456"
  secret: "session 加密密钥，务必改成随机字符串"
  session_max_age: 86400
  business_token: ""        # 业务查询 token；空则不校验。也可在设置页维护

database:
  path: "assethub.db"

oss:
  endpoint: "oss-cn-beijing.aliyuncs.com"   # 不要带 https://
  bucket: "your-bucket"
  access_key_id: "..."
  access_key_secret: "..."
  private: true             # true=私有读，返回带时效签名 URL；false=公共读
  signed_expire: 3600
  url_prefix: "https://your-bucket.oss-cn-beijing.aliyuncs.com/"
```

**已废弃**：`path_rule` / 系统级 `icon_prefix`、`cover_prefix`。对象路径前缀仅在**分类**的 `ossPrefix` 上配置；旧 yaml 若仍含 `path_rule` 会被忽略。

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
| GET | `/api/asset/list` | 分页：`keyword`, `categoryId`, `tagIds`（逗号分隔，**AND**）, `page`, `size` |
| GET | `/api/asset/:id` | 素材详情 + `files` + `tags` |
| PUT | `/api/asset/:id` | 修改 `{assetName, categoryId, remark, sort}`；若带 `tagIds` 或 `tagNames` 则**全量替换**标签（空数组即清空） |
| DELETE | `/api/asset/:id` | 删除素材（删 OSS 对象 + 逻辑删库） |
| POST | `/api/asset/:id/files` | 上传，`multipart/form-data`，字段 `file` |
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