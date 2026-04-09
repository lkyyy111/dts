# server part

本程序运行于服务器端，用于辅助客户端之间通过网络传输

## Configuration

1. 服务端ip与监听端口
2. 数据库连接
3. 静态文件托管目录

## dev tips

1. go proxy:

```
go env -w GO111MODULE=on
go env -w GOPROXY=https://goproxy.cn,direct
```

