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

| 表名 (Table)          | 字段名称          | 数据类型 | 说明                                                            | 开发注意项 |
| --------------------- | ----------------- | -------- | --------------------------------------------------------------- | ---------- |
| **users** (用户)      | id                | String   | 唯一标识 (ULID)                                                 |            |
|                       | nickname          | String   | 用户昵称                                                        |            |
| **spaces** (旅行空间) | id                | String   | 空间唯一标识(ULID)                                              |            |
|                       | name              | String   | 空间名称                                                        |            |
| **space_members**     | id                | String   | {space\*id}\_{user_id}拼接                                      |            |
|                       | space_id          | String   | 外键，关联 spaces                                               |            |
|                       | user_id           | String   | 外键，关联 users                                                |            |
| -----                 | -----             | -----    | -----                                                           |            |
| **photos** (照片)     | id                | String   | 照片ID(ULID)                                                    |            |
|                       | space_id          | String   | 所属空间ID                                                      |            |
|                       | uploader_id       | String   | 上传者ID                                                        |            |
|                       | remote_url        | String   | 上传到云端后的对象存储URL                                       |            |
|                       | post_id           | String   | 照片属于哪篇帖子                                                |            |
|                       | shoted_at         | Number   | 拍摄时间戳 （用户看到的拍摄时间）                               |            |
|                       | created_at        | Number   | 这条记录初次记录的时间戳                                        |            |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳                                      |            |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳                                          | 仅服务器有 |
|                       | last_modified     |          |                                                                 | 仅服务器有 |
|                       | server_created_at |          |                                                                 | 仅服务器有 |
| **expenses** (开销)   | id                | String   | 账单ID(ULID)                                                    |            |
|                       | space_id          | String   | 所属空间                                                        |            |
|                       | payer_id          | String   | 付款人 (user_id)                                                |            |
|                       | amount            | Number   | 金额（小数点后两位）                                            |            |
|                       | description       | String   | 消费描述 (如: 晚餐)                                             |            |
|                       | created_at        | Number   | 这条记录初次记录的时间戳                                        |            |
|                       | upadted_at        | Number   | 这条记录上次被修改的时间戳                                      |            |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳                                          | 仅服务器有 |
|                       | last_modified     |          |                                                                 | 仅服务器有 |
|                       | server_created_at |          |                                                                 | 仅服务器有 |
| **comments** (评论)   | id                | String   | 评论ID(ULID)                                                    |            |
|                       | space_id          | String   |                                                                 |            |
|                       | content           | String   | 评论内容                                                        |            |
|                       | commenter_id      | String   | 评论者id                                                        |            |
|                       | post_id           | String   | 帖子id                                                          |            |
|                       | commented_at      | Number   | 用户看到的评论时间                                              |            |
|                       | created_at        | Number   | 这条记录初次记录的时间戳                                        |            |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳                                      |            |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳                                          | 仅服务器有 |
|                       | last_modified     |          |                                                                 | 仅服务器有 |
|                       | server_created_at |          |                                                                 | 仅服务器有 |
| **posts**（帖子）     |                   |          | posts意味着多张照片的集合，评论和照片描述需以一个post为基本单位 |            |
|                       | id                | String   | 帖子ID                                                          |            |
|                       | space_id          | String   |                                                                 |            |
|                       | created_at        | Number   | 这条记录初次记录的时间戳                                        |            |
|                       | updated_at        | Number   | 这条记录上次被修改的时间戳                                      |            |
|                       | deleted_at        | Number   | 这条记录被删除的时间戳                                          | 仅服务器有 |
|                       | last_modified     |          |                                                                 | 仅服务器有 |
|                       | server_created_at |          |                                                                 | 仅服务器有 |

- 时间戳采用13位Unix时间戳
- created_at和updated_at由watermelonDB在定义model时使用`@date('created_at')`和`@date('updated_at')`装饰器产生，用于数据同步时的创建/更新
- deleted_at字段用于服务器实现“软删除”
- 业务逻辑注意事项：
  - `users`、`spaces`、`space_members` 为**核心关系表**：Pull 时整包放入 `created`，Push 时不做 delete/conflict/`last_modified` 逻辑，见 `docs/sync-degisn.md`。
  - `photos`、`expenses`、`comments`、`posts` 为**普通数据表**，增量同步与冲突规则一致。
- photo的local_uri统一为：`${App存储目录}/photos/${photo_id}.jpg`.前端需处理文件不存在的异常情况。

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
- `POST /api/v1/photos`
- `POST /api/v1/avatars`

websocket apis:

- `/api/v1/ws`
