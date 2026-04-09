# Distributed Travel Space

## How to start?

### local part

1. 安装`nodejs`，然后安装依赖(可能需要配置代理或镜像源)：

```bash
cd local
npm install
```

1. 如需使用服务器端的网络服务，你需要配置环境变量，确保IP地址和端口对应：

```bash
cp .env.example .env
```

然后修改`.env`文件

1. 启动项目：

```bash
npx expo start
```

### server part

1. 安装`go`, 然后你可能需要配置代理：

```bash
go env -w GO111MODULE=on
go env -w GOPROXY=https://goproxy.cn,direct
```

1. 直接启动项目或者构建可执行文件

```bash
cd server
# 启动
go run main.go
# 或者构建
go build main.go
```

1. 默认端口为8088，如有端口冲突，可自行更改配置，配置文件位于`internal/config/config.yaml`
2. 常用联调接口示例：

```bash
# 绑定用户与空间（幂等）
curl -X POST http://127.0.0.1:8088/api/v1/spaces \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"01USER\",\"space_id\":\"01SPACE\",\"space_name\":\"Test Space\"}"

# push 同步（简化协议）
curl -X POST http://127.0.0.1:8088/api/v1/sync \
  -H "Content-Type: application/json" \
  -d "{\"space_id\":\"01SPACE\",\"client_ts\":1710000000000,\"changes\":{\"users\":{\"upserts\":[],\"deletes\":[]},\"spaces\":{\"upserts\":[],\"deletes\":[]},\"space_members\":{\"upserts\":[],\"deletes\":[]},\"posts\":{\"upserts\":[],\"deletes\":[]},\"photos\":{\"upserts\":[],\"deletes\":[]},\"comments\":{\"upserts\":[],\"deletes\":[]},\"expenses\":{\"upserts\":[],\"deletes\":[]}}}"

# pull 增量
curl "http://127.0.0.1:8088/api/v1/sync?space_id=01SPACE&last_synced_at=0"
```

1. WebSocket 地址：`ws://127.0.0.1:8088/api/v1/ws?space_id=01SPACE&user_id=01USER`
  - 同一个 `space_id` 内的连接会互相广播消息
  - 不同 `space_id` 完全隔离

### What's more

如果你在本地电脑开发调试，你可能需要：

1. 想验证客户端与服务器端的数据同步，需要开启服务器端的数据库，你可以通过我们提供的docker-compose文件开启一个postgreSQL容器，数据库配置与端口配置可自行修改，需保证docker-compose中的配置与server端的配置文件一致；或者自行连接你已有的数据库。
2. 想验证客户端与服务器端的文件传输，服务器端提供了静态文件托管服务，你可以通过修改server端的配置文件，从而让客户端可以访问你的服务器端的静态文件；或者自行使用你已有的OSS服务。

