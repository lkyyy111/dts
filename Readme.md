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
