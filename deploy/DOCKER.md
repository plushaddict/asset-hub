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
- **多格式一体**：同一素材可挂多种后缀与说明，引用时按需选格式。
- **轻量可部署**：单二进制 / Docker，SQLite，内嵌管理页，不依赖复杂前端构建。
- **运营友好**：设置页配 OSS、试连 Bucket；业务方拿链接即可用，不必碰控制台。

一句话：**OSS 管存储，AssetHub 管「可引用、可维护的素材链接」**——把「传到哪」变成「用在哪都能稳定找到」。

| 项 | 说明 |
| --- | --- |
| 源码 | [github.com/plushaddict/asset-hub](https://github.com/plushaddict/asset-hub) |
| 镜像 | `plushaddict/assethub`（[Docker Hub](https://hub.docker.com/r/plushaddict/assethub)） |
| 架构 | `linux/amd64` |
| 数据 | SQLite：`/data/assethub.db`（**镜像内无业务库**，新挂载即空库） |
| 默认账号 | `admin` / `123456`；可用 `-e ASSETHUB_*` 覆盖（见第 5 节） |

OSS、业务 token、分类等：**登录后在设置页配置**，部署时不必挂 `config.yaml`。

---

## 1. 前置条件

- Docker（建议 24+）与 Compose v2
- 能拉 Docker Hub 镜像、能访问阿里云 OSS
- 备好 OSS：`endpoint` / `bucket` / AccessKey / `url_prefix`

---

## 2. 最快启动（docker run）

```bash
docker pull plushaddict/assethub:latest

docker run -d \
  --name assethub \
  --restart unless-stopped \
  -p 8080:8080 \
  -v assethub-data:/data \
  -e ASSETHUB_USERNAME=admin \
  -e ASSETHUB_PASSWORD='改成强密码' \
  plushaddict/assethub:latest
```

上面 `-e` 可整段省略（仍为 `admin` / `123456`）。`ASSETHUB_SECRET` 非必须，公网/多实例再设，见第 5 节。

打开 `http://<服务器IP>:8080`，用默认账号登录，再进设置页配 OSS / 分类。

```bash
docker logs -f assethub
docker stop assethub && docker rm assethub   # 删容器不删卷；数据仍在 assethub-data
```

---

## 3. Docker Compose

### 3.1 命名卷（默认，推荐）

在任意目录写 `docker-compose.yml`（群晖可直接用仓库内 [`docker-compose.synology.yml`](docker-compose.synology.yml)）：

```yaml
services:
  assethub:
    image: plushaddict/assethub:latest
    # 每次 up / 群晖「构建」前拉取该 tag 最新 digest（本地有 latest 也不跳过）
    pull_policy: always
    ports:
      - "8080:8080"
    volumes:
      - assethub-data:/data
    # 可选：覆盖登录（等价于 docker run -e；整段 environment 可删，则仍为 admin / 123456）
    environment:
      ASSETHUB_USERNAME: admin
      ASSETHUB_PASSWORD: "改成强密码"
      # ASSETHUB_SECRET 非必须，见第 5 节「何时设 SECRET」
      # ASSETHUB_SECRET: "改成至少32位随机字符串"
    restart: unless-stopped

volumes:
  assethub-data:    # 留空即可：由 Compose 自动创建命名卷，无需再填内容
```

```bash
docker compose up -d
```

顶层 `volumes.assethub-data:` 为空映射是正常写法，表示让 Docker 托管该卷。

> **群晖：** Container Manager 的「构建」≈ `compose up`，**不是**单独 `pull`。没有 `pull_policy: always` 时，本地已有 `latest` 就不会再拉 Hub。仓库根目录 `docker-compose.yml` 带 `build:`，仅适合本机源码构建，不要直接丢给群晖。

### 3.2 自定义宿主机目录

想把库文件落到指定路径（便于备份、挂盘）时，改用**绑定挂载**，并去掉顶层 `volumes:`：

```yaml
services:
  assethub:
    image: plushaddict/assethub:latest
    pull_policy: always
    ports:
      - "8080:8080"
    volumes:
      # 宿主机目录 : 容器内 /data
      - /opt/assethub/data:/data
      # 或相对路径：- ./data:/data
    environment:
      ASSETHUB_USERNAME: admin
      ASSETHUB_PASSWORD: "改成强密码"
      # ASSETHUB_SECRET 非必须，见第 5 节
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
git clone https://github.com/plushaddict/asset-hub.git
cd asset-hub
docker compose up --build -d
```

使用仓库根目录 `docker-compose.yml` + `deploy/Dockerfile`。

---

## 4. 首次使用（页面配置）

1. 打开 `http://<主机>:8080`，`admin` / `123456` 登录  
2. **设置 → OSS**：填 endpoint / bucket / AK / SK / url_prefix 并保存（会试连）  
3. **设置 → 分类**：至少建一个，填 `ossPrefix`（如 `icon/`、`cover/`）  
4. （可选）设置业务 API 的 `business_token`  
5. 首页引导条消失后即可建素材：同一弹窗可顺带上传多种格式；详情页也可本地上传或填远程 `http(s)` URL 由服务端拉取  

业务查询示例：

```http
GET /api/business/asset?assetName=book_cover_001
Header: X-Asset-Token: <business_token>
```

> OSS / 业务 token **只**走设置页；登录账号与 session `secret` 可用环境变量或 yaml 配置，页面改不了。

---

## 5. 改登录账号（可选）

日常部署可不改默认账号。需要改登录时，**优先用环境变量**（不必挂自定义 yaml）；与 yaml 同时存在时，**非空环境变量优先**。

| 变量 | 覆盖 | 是否必须 |
| --- | --- | --- |
| `ASSETHUB_USERNAME` | `server.username` | 否；不设则用 yaml / 默认 `admin` |
| `ASSETHUB_PASSWORD` | `server.password` | 否；生产建议设强密码 |
| `ASSETHUB_SECRET` | `server.secret`（登录 Cookie 签名密钥） | **否**；见下方「何时设 SECRET」 |

空字符串与未设置同等，均不覆盖镜像内默认值。启动日志会列出被覆盖的键名，**不会**打印密码或 secret 明文。  
**注意：** 密码会出现在 `docker inspect` 与进程环境变量中；勿将含真实密钥的 compose 提交到公开仓库。

### 何时设 `ASSETHUB_SECRET`

`secret` 只用于**签名管理端登录 Cookie**，与登录密码无关，也**不是**改密码的必选项。

| 场景 | 建议 |
| --- | --- |
| 本机 / 内网试跑，只改密码 | **可不设**；继续用镜像内默认 secret 即可 |
| 公网或不可信网络暴露 | **建议设**一串至少 32 位随机字符，避免他人用默认 secret 伪造 Cookie |
| 多副本 / 滚动重启要保持已登录会话 | 各实例设**相同** `ASSETHUB_SECRET`；改 secret 会使已有登录全部失效，需重新登录 |

日常只改密码时，设 `ASSETHUB_PASSWORD`（可选再设 `ASSETHUB_USERNAME`）就够了。

### 5.1 docker run（推荐）

只改密码（最常见）：

```bash
docker run -d \
  --name assethub \
  --restart unless-stopped \
  -p 8080:8080 \
  -v assethub-data:/data \
  -e ASSETHUB_USERNAME=admin \
  -e ASSETHUB_PASSWORD='你的强密码' \
  plushaddict/assethub:latest
```

公网部署再追加一行（可选）：

```bash
  -e ASSETHUB_SECRET='至少32位随机字符串' \
```

也可只设密码：`-e ASSETHUB_PASSWORD='你的强密码'`。

### 5.2 Compose

第 3 节示例已含 `environment`。仓库内 `docker-compose.yml` / `deploy/docker-compose.synology.yml` 也有注释块，取消注释即可。只改密码时不必写 `ASSETHUB_SECRET`：

```yaml
    volumes:
      - assethub-data:/data
    environment:
      ASSETHUB_USERNAME: admin
      ASSETHUB_PASSWORD: "改成强密码"
      # 公网建议取消下一行注释：
      # ASSETHUB_SECRET: "改成至少32位随机字符串"
```

### 5.3 挂载 config.yaml（备选）

不想用环境变量时，可挂载自定义 `config.yaml`。**不要**在 yaml 里写 OSS / business_token。

`database.path` 必须为 `/data/assethub.db`：

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
  plushaddict/assethub:latest
```

若同时设置 `-e ASSETHUB_*` 与挂载 yaml，以**环境变量为准**。

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

Compose 已设 **`pull_policy: always`** 时：Hub 上对应 tag 有新构建后，执行 `docker compose up -d`，或群晖对该项目点 **构建**，会按 tag 再拉一次并重建容器。

仅「停止 / 启动」在部分 DSM 上**不会**重新 pull。

未设 `pull_policy` 时需手动：

```bash
docker compose pull
docker compose up -d
```

`docker run` 部署：

- 卷路径：继续用升级前的同一挂载（命名卷 `assethub-data` 或你的自定义宿主机目录）。
- 若曾用 `-e ASSETHUB_*` 改登录，在下方 `docker run` 中**补上相同 `-e` 行**（见第 5 节）。
- 若曾挂载自定义 `config.yaml`，补上 `-v /path/config.yaml:/app/config.yaml:ro`。

```bash
docker pull plushaddict/assethub:latest

docker stop assethub && docker rm assethub
docker run -d \
  --name assethub \
  --restart unless-stopped \
  -p 8080:8080 \
  -v assethub-data:/data \
  plushaddict/assethub:latest
```

需要钉死版本时，把最后一行镜像改成如 `plushaddict/assethub:X.Y.Z` 再执行。

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

## 9. 发版到 Docker Hub（维护者）

推送符合 `v*` 的 git tag 后，GitHub Actions（[`.github/workflows/docker-publish.yml`](../.github/workflows/docker-publish.yml)）会构建并推送：

| Hub tag | 来源 |
| --- | --- |
| `X.Y.Z` | git tag `vX.Y.Z` |
| `latest` | 同一次正式发版一并打上 |

**仓库 Secrets（Settings → Secrets and variables → Actions）：**

| Name | 说明 |
| --- | --- |
| `DOCKERHUB_USERNAME` | Docker Hub 用户名（镜像为 `{USERNAME}/assethub`，当前应为 `plushaddict`） |
| `DOCKERHUB_TOKEN` | Docker Hub Access Token |

**发版命令示例：**

```bash
git checkout main
git pull
# 确认待发布提交已在 main
git tag -a vX.Y.Z -m "vX.Y.Z"
git push origin vX.Y.Z
```

在 Actions 中查看 `docker-publish` 是否成功。构建失败时保留该 tag 作记录，修完后打**新** semver tag 再推，不要默认 force 挪已失败的正式版 tag。

镜像构建参数会注入页脚版本号（`VERSION=${{ github.ref_name }}`）。

---

## 10. 常见问题

| 现象 | 处理 |
| --- | --- |
| 页面打不开 | `docker logs assethub`；查端口与安全组 |
| `volumes.assethub-data:` 要不要填？ | 不用；空着即自动命名卷。要自定义路径见 **3.2** |
| 群晖「构建」仍是旧版本 | Compose 缺 `pull_policy: always`；改用 **3.1** / `docker-compose.synology.yml` 后重建 |
| 想改默认登录密码 | 优先 `-e ASSETHUB_PASSWORD` 等（第 5 节）；或挂载自定义 config（备选） |
| 上传 / 保存 OSS 失败 | 查 AK；保存设置需 `oss:GetBucketInfo` |
| 换机 / 升级后数据没了 | 未挂同一命名卷或同一宿主机目录 |
| 架构不兼容 | 官方镜像为 `linux/amd64`；ARM 需模拟或自建多架构 |
| Actions 发版失败 | 查 Secrets；Login 绿、Build 红 → 看构建日志。失败 tag 勿 force 重推，改打下一版本 |
