# AssetHub Docker 部署教程

轻量图片素材管理系统：按分类与标签组织素材，上传到阿里云 OSS，并生成可复制、可管理的公网访问链接，方便在其它应用和文档中直接引用图片。

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

| 项 | 说明 |
| --- | --- |
| 镜像 | `blackcatzoo/assethub`（Docker Hub） |
| 架构 | `linux/amd64` |
| 数据 | SQLite：`/data/assethub.db`（**镜像内无业务库**，新挂载即空库） |
| 默认账号 | `admin` / `123456`（可改，见第 5 节） |

OSS、业务 token、分类等：**登录后在设置页配置**，部署时不必挂 `config.yaml`。

---

## 1. 前置条件

- Docker（建议 24+）与 Compose v2
- 能拉 Docker Hub 镜像、能访问阿里云 OSS
- 备好 OSS：`endpoint` / `bucket` / AccessKey / `url_prefix`

---

## 2. 最快启动（docker run）

```bash
docker pull blackcatzoo/assethub:latest

docker run -d \
  --name assethub \
  --restart unless-stopped \
  -p 8080:8080 \
  -v assethub-data:/data \
  blackcatzoo/assethub:latest
```

打开 `http://<服务器IP>:8080`，用默认账号登录，再进设置页配 OSS / 分类。

```bash
docker logs -f assethub
docker stop assethub && docker rm assethub   # 删容器不删卷；数据仍在 assethub-data
```

---

## 3. Docker Compose

### 3.1 命名卷（默认，推荐）

在任意目录写 `docker-compose.yml`：

```yaml
services:
  assethub:
    image: blackcatzoo/assethub:latest
    ports:
      - "8080:8080"
    volumes:
      - assethub-data:/data
    restart: unless-stopped

volumes:
  assethub-data:    # 留空即可：由 Compose 自动创建命名卷，无需再填内容
```

```bash
docker compose up -d
```

顶层 `volumes.assethub-data:` 为空映射是正常写法，表示让 Docker 托管该卷。

### 3.2 自定义宿主机目录

想把库文件落到指定路径（便于备份、挂盘）时，改用**绑定挂载**，并去掉顶层 `volumes:`：

```yaml
services:
  assethub:
    image: blackcatzoo/assethub:latest
    ports:
      - "8080:8080"
    volumes:
      # 宿主机目录 : 容器内 /data
      - /opt/assethub/data:/data
      # 或相对路径：- ./data:/data
    restart: unless-stopped
```

先建目录：

```bash
mkdir -p /opt/assethub/data
# 确保 Docker 对该目录可写（按系统调整权限）
```

| 写法 | 含义 |
| --- | --- |
| `assethub-data:/data` | Docker 命名卷（自动管理） |
| `/opt/assethub/data:/data` | 绝对路径绑定 |
| `./data:/data` | 相对 compose 文件所在目录 |

`docker run` 同理：把 `-v assethub-data:/data` 换成 `-v /opt/assethub/data:/data`。

### 3.3 从源码本地构建

```bash
cd asset-hub
docker compose up --build -d
```

使用仓库内 `docker-compose.yml` + `deploy/Dockerfile`。

---

## 4. 首次使用（页面配置）

1. 打开 `http://<主机>:8080`，`admin` / `123456` 登录  
2. **设置 → OSS**：填 endpoint / bucket / AK / SK / url_prefix 并保存（会试连）  
3. **设置 → 分类**：至少建一个，填 `ossPrefix`（如 `icon/`、`cover/`）  
4. （可选）设置业务 API 的 `business_token`  
5. 首页引导条消失后即可建素材、上传文件  

业务查询示例：

```http
GET /api/business/asset?assetName=book_cover_001
Header: X-Asset-Token: <business_token>
```

> OSS / 业务 token **只**走设置页；登录账号与 session `secret` 只来自 yaml，页面改不了。

---

## 5. 改登录密码（可选）

仅当需要改**用户名 / 密码 / session secret** 时挂载配置。日常部署可跳过。  
**不要**在 yaml 里写 OSS / business_token。

`config.yaml` 示例（`database.path` 必须为 `/data/assethub.db`）：

```yaml
server:
  port: 8080
  username: admin
  password: "改成强密码"
  secret: "改成至少32位随机字符串"
  session_max_age: 86400

database:
  path: "/data/assethub.db"
```

```bash
docker run -d \
  --name assethub \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /opt/assethub/data:/data \
  -v /opt/assethub/config.yaml:/app/config.yaml:ro \
  blackcatzoo/assethub:latest
```

Compose：

```yaml
    volumes:
      - /opt/assethub/data:/data
      - /opt/assethub/config.yaml:/app/config.yaml:ro
```

---

## 6. 备份与清空

| 位置 | 内容 |
| --- | --- |
| `/data/assethub.db` | 分类、标签、素材元数据 |
| 阿里云 OSS | 图片文件本身 |

**命名卷备份：**

```bash
docker run --rm -v assethub-data:/data -v "$(pwd)":/backup alpine \
  cp /data/assethub.db /backup/assethub.db.bak
```

**自定义目录：** 直接备份宿主机上的 `assethub.db`（重要操作前建议先 `docker compose stop`）。

**清空库（不可恢复）：**

```bash
# 命名卷
docker compose down -v    # 或 docker volume rm assethub-data
docker compose up -d

# 自定义目录：停容器后删掉宿主机目录内 db 文件再启动
```

清空库**不会**删 OSS 上的对象。

---

## 7. 升级镜像

```bash
docker pull blackcatzoo/assethub:latest

docker stop assethub && docker rm assethub
docker run -d \
  --name assethub \
  --restart unless-stopped \
  -p 8080:8080 \
  -v assethub-data:/data \          # 或继续用你的自定义目录
  blackcatzoo/assethub:latest
```

Compose：

```bash
docker compose pull
docker compose up -d
```

务必继续挂载**同一数据路径**；若曾挂 `config.yaml`，升级时一并带上。

---

## 8. 反向代理（可选）

```nginx
server {
    listen 443 ssl;
    server_name assethub.example.com;
    # ssl_certificate     ...;
    # ssl_certificate_key ...;

    client_max_body_size 32m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

可只对本机开放 8080，由 Nginx 对外提供 HTTPS。

---

## 9. 发布镜像（维护者）

仓库 Secrets：`DOCKERHUB_USERNAME`、`DOCKERHUB_TOKEN`（Access Token，非登录密码）。

```bash
git tag v0.0.2
git push origin v0.0.2
```

产物：`blackcatzoo/assethub:0.0.2`、`blackcatzoo/assethub:latest`（`linux/amd64`）。  
工作流：`.github/workflows/docker-publish.yml`。

---

## 10. 常见问题

| 现象 | 处理 |
| --- | --- |
| Actions：`Username and password required` | 未配 Docker Hub Secrets |
| 页面打不开 | `docker logs assethub`；查端口与安全组 |
| `volumes.assethub-data:` 要不要填？ | 不用；空着即自动命名卷。要自定义路径见 **3.2** |
| 想改默认登录密码 | 挂载自定义 config（第 5 节）并重建容器 |
| 上传 / 保存 OSS 失败 | 查 AK；保存设置需 `oss:GetBucketInfo` |
| 换机 / 升级后数据没了 | 未挂同一命名卷或同一宿主机目录 |
| 架构不兼容 | 官方镜像为 `linux/amd64`；ARM 需模拟或自建多架构 |

---

## 相关文件

| 文件 | 作用 |
| --- | --- |
| `deploy/Dockerfile` | 多阶段构建（Go → Alpine） |
| `config.docker.yaml` | 镜像内默认进程配置（账号 / 库路径） |
| `docker-compose.yml` | 仓库内编排示例（命名卷） |
| `.github/workflows/docker-publish.yml` | 推送 `v*` tag 自动发布 |
