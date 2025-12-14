# Mail Generator

这是一个用于个人使用的简易邮件转发和管理系统。支持域名管理、正则表达式邮箱匹配转发，并提供日志查看功能。

## 架构

- **后端**: Go + Gin + GORM + SQLite + go-smtp
- **前端**: Vue 3 + Vite + Pinia + Ant Design Vue

## 目录结构

- `server/`: 后端代码
- `web/`: 前端代码

## 开发环境快速启动

### 1. 启动后端

默认端口: 8080 (API), 2525 (SMTP)
默认密码: `admin123`

```bash
cd server
go mod tidy
# 运行当前目录下的所有文件
go run .
```

### 2. 启动前端

默认端口: 5173

```bash
cd web
npm install
npm run dev
```

---

## 生产环境构建与部署

### 1. 编译构建

**先决条件**:
- Go 1.20+
- Node.js 16+
- Make (可选，用于快捷构建)

#### 后端构建

使用 Makefile (推荐 Linux/macOS/WSL):
```bash
cd server
# 生成 release 版本 (server/bin/mail-server)
make release

# 或者交叉编译 Linux 版本
make build-linux
```

或者使用原生 Go 命令:
```bash
cd server
# Linux
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o bin/mail-server .
# Windows
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o bin/mail-server.exe .
```

#### 前端构建

```bash
cd web
npm install
npm run build
# 构建产物位于 web/dist 目录
```

### 2. 部署指南 (Linux Systemd + Nginx)

假设你的服务器目录为 `/opt/mail-gen`，结构如下：
```text
/opt/mail-gen/
├── mail-server      # 后端二进制文件
├── dist/            # 前端构建产物 (web/dist 拷贝至此)
└── mail.db          # 数据库文件 (自动生成)
```

#### Systemd 服务配置

创建服务文件 `/etc/systemd/system/mail-server.service`:

```ini
[Unit]
Description=Mail Generator Backend Service
After=network.target

[Service]
Type=simple
User=root
# 建议新建专用用户运行，如: User=mailgen
WorkingDirectory=/opt/mail-gen
ExecStart=/opt/mail-gen/mail-server
Restart=always

# 环境变量配置
Environment="PORT=8080"
Environment="SMTP_PORT=25"
Environment="PASSWORD=your_secure_password"
Environment="DB_FILE=/opt/mail-gen/mail.db"
# JWT 密钥
Environment="JWT_SECRET=your_random_secret_string"
# SMTP 发信模式 (任选其一)

# 模式 A: 使用中继 (Relay Mode) - 推荐家用宽带或云主机
# Environment="SMTP_RELAY_HOST=smtp.gmail.com"
# Environment="SMTP_RELAY_PORT=587"
# Environment="SMTP_RELAY_USER=yourname@gmail.com"
# Environment="SMTP_RELAY_PASS=your_app_password"

# 模式 B: 直连发送 (Direct Mode) - 推荐 VPS (需开放 25 端口 + PTR 记录)
# 留空 SMTP_RELAY_HOST 即开启直连模式
Environment="SMTP_RELAY_HOST="
Environment="DEFAULT_ENVELOPE=postmaster@yourdomain.com"

[Install]
WantedBy=multi-user.target
```

启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable mail-server
sudo systemctl start mail-server
```

#### Nginx 配置

配置 Nginx 反向代理 API 并托管静态文件。
编辑 `/etc/nginx/sites-available/mail-gen.conf`:

```nginx
server {
    listen 80;
    server_name mail.yourdomain.com;

    # 强制 HTTPS (推荐)
    # return 301 https://$host$request_uri;
    
    # 前端静态资源
    location / {
        root /opt/mail-gen/dist;
        index index.html;
        # 解决 SPA 路由刷新 404 问题
        try_files $uri $uri/ /index.html;
    }

    # 后端 API 代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

启用配置并重载 Nginx:
```bash
sudo ln -s /etc/nginx/sites-available/mail-gen.conf /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## 环境变量说明

| 变量名 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | 8080 | Web API 监听端口 |
| `SMTP_PORT` | 2525 | SMTP 服务监听端口 (生产环境建议 25) |
| `PASSWORD` | admin123 | 管理后台登录密码 |
| `DB_FILE` | mail.db | SQLite 数据库路径 |
| `JWT_SECRET` | very-secret-key | JWT 签名密钥 (生产环境请务必修改) |
| `SMTP_RELAY_HOST` | - | 外部 SMTP 中继服务器地址。**留空则启用直连发送模式** |
| `SMTP_RELAY_PORT` | 587 | 外部 SMTP 端口 |
| `SMTP_RELAY_USER` | - | 外部 SMTP 账号 |
| `SMTP_RELAY_PASS` | - | 外部 SMTP 密码/应用密码 |
| `DEFAULT_ENVELOPE`| postmaster@localhost | 转发邮件时使用的发件人 (Envelope From) |

## 发信模式说明

### 1. 中继模式 (Relay Mode)
通过 Gmail、Outlook 等第三方服务转发邮件。
- **优点**: 配置简单，无需担心 IP 信誉、PTR 记录，不容易进垃圾箱。
- **缺点**: 受限于第三方服务的发送额度。
- **配置**: 填写 `SMTP_RELAY_HOST` 及相关认证信息。

### 2. 直连模式 (Direct Mode)
直接连接目标邮件服务器发送。
- **优点**: 自主可控，无额度限制。
- **缺点**: 要求高。需要服务器**开放 TCP 25 端口**（出站），必须配置 **PTR 记录** (反向解析) 和 SPF 记录，否则极易被拒收。
- **配置**: 将 `SMTP_RELAY_HOST` 留空即可开启。
