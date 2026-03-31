# Data Design

This doc contains all the data design information of this project.

## ID of users and travel spaces

We use [ULID](https://ulid.page/) to identify users and spaces:

- 128-bit
- suited for distributed systems and lexicographically sortable
- encoded as a **26** char string
- libraries:
  - [npm ulid](https://www.npmjs.com/package/ulid)
  - [go ulid](https://github.com/oklog/ulid)

## Local-First Data

In the frontend, we use `watermelonDB`, which offers local-first capacity. The backend provides `GET /sync` and `POST /sync` APIs to achieve "Pull/Push" for frontend.

| 表名 (Table)          | 字段名称          | 数据类型 | 说明                               | 服务器/本地区别                              |
| --------------------- | ----------------- | -------- | ---------------------------------- | -------------------------------------------- |
| **users** (用户)      | id                | String   | 唯一标识 (ULID)                    |                                              |
|                       | nickname          | String   | 用户昵称                           |                                              |
|                       | avatar_local_uri  | string   | 头像uri（本地文件）                | 服务器db没有此字段，为了统一可有此字段但留空 |
|                       | avatar_remote_url | string   | 头像url（上传到云端的对象存储URL） |                                              |
|                       | created_at        | Number   | 这条记录初次记录的时间戳           |                                              |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳         |                                              |
|                       | deleted_at        | Number   | 这条记录删除的时间戳               | 服务器db没有此字段，为了统一可有此字段但留空 |
| **spaces** (旅行空间) | id                | String   | 空间唯一标识(ULID)                 |                                              |
|                       | name              | String   | 空间名称                           |                                              |
|                       | created_at        | Number   | 这条记录初次记录的时间戳           |                                              |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳         |                                              |
| **space_members**     | id                | String   | {space\*id}\_{user_id}拼接         |                                              |
|                       | space_id          | String   | 外键，关联 spaces                  |                                              |
|                       | user_id           | String   | 外键，关联 users                   |                                              |
|                       | created_at        | Number   | 这条记录初次记录的时间戳           |                                              |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳         |                                              |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳             |                                              |
| -----                 | -----             | -----    | -----                              |                                              |
| **photos** (照片)     | id                | String   | 照片ID(ULID)                       |                                              |
|                       | space_id          | String   | 所属空间ID                         |                                              |
|                       | uploader_id       | String   | 上传者ID                           |                                              |
|                       | local_uri         | String   | 离线时的本地文件路径               | 服务器db没有此字段，为了统一可有此字段但留空 |
|                       | remote_url        | String   | 上传到云端后的对象存储URL          |                                              |
|                       | shoted_at         | Number   | 拍摄时间戳 （用户看到的拍摄时间）  |                                              |
|                       | created_at        | Number   | 这条记录初次记录的时间戳           |                                              |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳         |                                              |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳             |                                              |
| **expenses** (开销)   | id                | String   | 账单ID(ULID)                       |                                              |
|                       | space_id          | String   | 所属空间                           |                                              |
|                       | payer_id          | String   | 付款人 (user_id)                   |                                              |
|                       | amount            | Number   | 金额（小数点后两位）               |                                              |
|                       | description       | String   | 消费描述 (如: 晚餐)                |                                              |
|                       | created_at        | Number   | 这条记录初次记录的时间戳           |                                              |
|                       | upadted_at        | Number   | 这条记录上次被修改的时间戳         |                                              |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳             |                                              |

- 时间戳采用13位Unix时间戳
- 注意本地uri和云端url，一般来说，先同步数据库，然后决定有无头像或照片要上传，上传后才能够得到云端url，所以可能会出现暂时为空的情况
- created_at和updated_at由watermelonDB在定义model时使用`@date('created_at')`和`@date('updated_at')`装饰器产生，用于数据同步时的创建/更新
- deleted_at字段用于实现“软删除”
- 业务逻辑注意事项：
  - users， spaces，space_members这三张表涉及应用的核心逻辑，写代码时需要特别留意
  - photos，expenses本质上都是数据，实现逻辑应该是一致的

## Real-time Data

- location：
  - latitude(纬度)：Float
  - longitude(经度)：Float
- battery: Int (0-100)
- updated_at(最后一次更新的时间戳): Number

## APIs

http apis:
  - `POST /api/v1/spaces`
  - `GET /api/v1/sync`
  - `POST /api/v1/sync`
  - `POST /api/v1/photo`
  - `POST /api/v1/avator` (TODO)
websocket apis:
  - `/api/v1/ws`

## How to use?

1. 本地，不涉及与其他客户端同步，此时只需要正常读写本地数据库即可
2. 涉及同步：
  - 同步以一个空间为单位，也就是说用户可以手动选择一个空间“开启同步共享”或者“不共享仅自用”
  - 首先用户需要通过`POST /api/v1/spaces`上传一个user和space的绑定关系：
    - 客户端每次同步时均需要先`POST /api/v1/spaces`，因为无法确定上传到的服务器是网络服务器，还是未来局域网同步时的p2p中的服务器
    - 服务器需处理此请求，根据本地数据库检查是否有更新需要插入
  - 然后用户进行`POST /api/v1/sync`和`GET /api/v1/sync`，同步数据：
    - 客户端需实现watermelonDB提供的同步接口，在其中调用上述两个sync api
    - 客户端需分别处理这两个api请求
  - 当完成一次数据同步后
    - 客户端需要检查avator和photo记录：
      - 如果local_uri为空，说明其他客户端有图片更新，需要根据remote_url下载到本地，然后填入local_uri
      - 如果remote_url为空，说明本地更新的图片未上传，需要通过`POST /api/v1/photo`和`POST /api/v1/avator`上传，将返回的remote_url填入本地更新即可
    - 服务器需要处理`POST /api/v1/photo`和`POST /api/v1/avator`请求
    - 本功能模块有以下设计疑问：仅考虑服务器情况，此设计没有问题，但如果变为局域网p2p同步，需要新的办法来检测图片的上传下载
3. 涉及实时：
  - 客户端直接使用websocket连接收发信息即可
  - 服务器需处理websocket连接，实现同一个空间中多个websocket连接的消息转发