# CAS Gateway

基于 Go 的 CAS 单点登录网关代理服务，用于代理内部系统并统一进行 CAS 认证。

## 功能特性

- 🔐 集成 CAS 单点登录系统
- 🔄 反向代理后端服务
- 🛡️ 统一的 CAS 认证中间件
- 💾 Session 会话管理

## 快速开始

### 配置

复制 `config.example.yaml` 为 `config.yaml` 并修改配置：

```yaml
server:
  port: 8080
  session_key: "your-secret-key-here" # 32字节密钥

cas:
  base_url: "https://cas.example.com/"
  login_path: "/login"              # 可选，默认为 "/login"
  validate_path: "/p3/serviceValidate"  # 可选，默认为 "/p3/serviceValidate"
  use_json: true  # 使用JSON格式（添加format=json参数），推荐使用

route:
  name: finops
  path: "/"                          # 路由路径前缀，"/" 表示所有请求
  target: "http://127.0.0.1:8000"   # 后端服务地址
```

#### 配置说明

**`server`** - 服务器配置
- `port`: 服务监听端口
- `session_key`: 会话加密密钥（必须至少 32 字节）

**`cas`** - CAS 认证配置
- `base_url`: CAS 服务器基础 URL（必须以 `/` 结尾）
- `login_path`: CAS 登录路径，默认为 `/login`
- `validate_path`: CAS ticket 验证路径，默认为 `/p3/serviceValidate`
- `use_json`: 是否使用 JSON 格式验证（推荐启用）

**`route`** - 路由配置（单个路由）
- `name`: 路由名称（用于日志标识）
- `path`: 路由路径前缀（如 `/` 或 `/finops`），所有请求都会转发到后端服务
- `target`: 后端服务目标地址

**`session_key` 生成方式**：
```bash
# Linux/Mac
openssl rand -base64 32

# Windows PowerShell
[Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))

# Python
python -c "import secrets; print(secrets.token_urlsafe(32))"
```

**安全提示**：
- 生产环境务必使用强随机密钥
- 不要将真实密钥提交到代码仓库
- 多个服务器实例应使用相同的 `session_key` 以共享会话
- ⚠️ **重要**：修改 `session_key` 会导致所有已登录用户需要重新登录（旧的 Cookie 无法被新密钥解密）

### 运行

```bash
go mod download
go run main.go
```

### 构建

```bash
go build -o cas-gateway main.go
./cas-gateway
```

## 项目结构

```
.
├── main.go              # 程序入口
├── config/              # 配置管理
│   └── config.go
├── auth/                # 认证模块
│   ├── provider.go      # 认证提供者接口
│   └── cas/             # CAS 认证实现
│       ├── cas_provider.go
│       └── types.go
├── proxy/               # 反向代理
│   └── proxy.go
├── middleware/          # 中间件
│   └── auth.go
└── models/              # 数据模型
    └── config.go
```

## 认证机制说明

### Session (CookieStore) vs JWT Token

本项目使用 **Session (CookieStore)** 方式存储认证信息，而非 JWT Token。以下是两种方式的对比：

| 特性 | Session (CookieStore) | JWT Token |
|------|----------------------|-----------|
| **数据存储位置** | 客户端 Cookie（加密） | 客户端 Cookie/Header |
| **服务器状态** | 无状态（数据在 Cookie 中） | 无状态（Token 自包含） |
| **会话撤销** | ✅ 可立即撤销（删除 Cookie） | ❌ 无法主动撤销（需等待过期） |
| **数据大小** | 较小（仅存储必要信息） | 较大（包含完整用户信息） |
| **安全性** | 高（HttpOnly + 加密） | 中（依赖签名密钥） |
| **水平扩展** | ✅ 支持（需相同 session_key） | ✅ 支持（无需共享） |
| **适用场景** | 网关、需要快速撤销会话 | API、微服务间通信 |

**为什么选择 Session (CookieStore)？**

1. ✅ **快速撤销会话**：用户登出或安全事件时，可立即使会话失效
2. ✅ **数据量小**：仅存储用户标识和认证状态，适合 Cookie 限制
3. ✅ **安全性好**：HttpOnly 防止 XSS，加密防止篡改
4. ✅ **简单部署**：无需额外的 Redis/数据库，适合单实例或少量实例部署

**多实例部署说明**：

- 只要所有实例使用**相同的 `session_key`**，无论负载均衡器使用什么规则（轮询、最少连接、IP-hash 等），都能正常工作
- 因为 CookieStore 将数据加密存储在客户端 Cookie 中，任何实例都能解密
- 无需 Redis 等共享存储（除非需要会话统计或主动撤销功能）

### 修改 session_key 的影响

**场景**：服务使用 `session_key=1` 运行一段时间后，修改为 `session_key=2` 并重启

**客户端会发生什么？**

1. ❌ **所有已登录用户会被强制登出**
   - 旧的 Cookie 是用 `session_key=1` 加密的
   - 新服务使用 `session_key=2` 无法解密旧的 Cookie
   - 服务器会认为用户未认证，重定向到 CAS 登录页

2. ✅ **用户需要重新登录**
   - 用户访问时会自动跳转到 CAS 登录页
   - 登录成功后，新 Cookie 使用 `session_key=2` 加密
   - 之后可以正常使用

**最佳实践**：

- 🔒 **生产环境不要随意修改 `session_key`**
- 🔄 **如需修改**：建议在低峰期进行，并提前通知用户
- 🔑 **密钥泄露**：如果 `session_key` 泄露，必须立即修改并强制所有用户重新登录

## License

MIT
