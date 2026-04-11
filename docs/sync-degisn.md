# Sync design

本文详细讲述项目的同步机制。

## watermelonDB提供了什么？

1. 软删除，根据[官方文档](https://watermelondb.dev/docs/CRUD#delete-a-record)，由于涉及同步，需要调用`await somePost.markAsDeleted()`实现软删除。
2. `synchronize()`函数，这个函数中有两个*callback*函数，`pullChanges`和`pushChanges`，前端可以：
  1. 实现*callback*函数中调用API的逻辑
  2. 定义一个`MyFunc()`，调用实现了*callback*的`synchronize()`函数。然后在需要的场景，例如按了一个按钮后，触发`MyFunc()`
3. API规范

## 需要注意什么？

本地和服务器的数据库表，定义会不一样。这些字段需要理解：

- `created_at`/`updated_at`：用户视角的数据元信息，服务器和本地数据库都有，由客户端通过watermelon的装饰器自动填入
- `deleted_at`：
  - 本地watermelonDB没有这个字段，因为它有软删除机制，通过其隐藏的`_status`字段标记已删除
  - 服务器端需要有这个字段，因为服务器端是通用数据库，需要一个`deleted_at`做显式删除标记，而不能直接删除。标记删除后等客户端拉取更新，就要把该记录放到API回复中的delete里。
- `last_modified`/`server_created_at`:
  - 本地没有这个字段，只有服务器端有
  - 这两个字段是用于辅助同步的，当客户端pull时，比对客户端调用API里传来的last_pulled_at参数
    - server_created_at > last_pulled_at: 说明对客户端来说是新建的，塞到API回复的created中
    - last_modified > last_pulled_at: 说明对客户端来说是修改过的，塞到API回复的updated中
  - 需要区分`created_at`/`updated_at`

## 同步过程

假设以空间为同步单位，用户点击“同步”后，同步过程如下：

1. 客户端 `POST spaces`，参数userid和spaceid
2. 客户端 `GET sync`, 参数user_id, space_id, last_pulled_at.
  1. 服务器的处理方式如下：
    1. 对于user，space, space_members表：
      1. 先根据space_id做筛选，选出space内的user，space，space_members
      2. 直接把筛选出来的数据塞到changes的created里
    2. 对于其他数据表：
      1. 先根据space_id做筛选，选出space内的数据
      2. 根据last_pulled_at分别往changes里塞相应的created，updated，deleted
    3. 返回此回复。注意服务器要把客户端没有的字段删去。
  2. 客户端接收到回复，watermelonDB处理此回复
3. 客户端 `POST sync`，
  1. 客户端会直接对watermelon本地自动生成的changes，调用API即可。其中：
    1. `photo` 的 `remote_url` 可能是空的
    2. WatermelonDB 自动生成的 `changes` 是全局 changes，而不是按某个 `space_id` 自动筛过的 changes
    3. 当前阶段客户端可先直接全局 push，服务端负责按记录本身的外键、合法性和约束落库；真正的“按空间隔离”主要体现在 Pull：`GET /sync?space_id=...` 只返回该空间相关的 `users / spaces / space_members / posts / photos / expenses / comments`
    4. 后续如有需要，前端可在 `pushChanges` 前自行按当前空间做预过滤
  2. 服务器接收到 push 过来的 `changes` 后，在单个事务中按表遍历 `created / updated / deleted`。处理方式分为两类：
    1. 对于 `user`，`space`，`space_members` 表：
      1. 这三张表属于空间同步的核心关系表，主要作用是维护“空间是谁、空间里有哪些人、这些人分别是谁”
      2. 服务端先按记录本身的主键和关系进行校验与规范化，其中 `space_members.id` 按 `{space_id}_{user_id}` 重新生成，避免客户端传错 id
      3. `created`：若记录不存在则插入；若已存在则按更新处理，并恢复 `deleted_at`
      4. `updated`：若记录不存在则按插入处理；若已存在则按更新处理，并刷新 `last_modified`
      5. `deleted`：不物理删除，而是设置 `deleted_at`
      6. 这三张表主要承担关系维护职责，不按普通业务数据那样依赖客户端的 `created_at / updated_at` 做同步判断
    2. 对于其他数据表（`photos`，`expenses`，`comments`，`posts`）：
      1. 先根据记录本身的 `space_id` 或关联关系确认其归属空间
      2. `created`：若记录不存在则插入；若已存在则按更新处理
      3. `updated`：若记录不存在则按插入处理；若已存在则先做冲突检测，若服务端该记录的 `last_modified > last_pulled_at`，则返回 `409 conflict`；否则更新并刷新 `last_modified`
      4. `deleted`：不物理删除，而是设置 `deleted_at`
      5. 对这些数据表，服务端使用 `last_modified / server_created_at / deleted_at` 来支持后续 Pull 时的 `created / updated / deleted` 分类
    3. 整体成功返回 `200` 和 `{ "ok": true }`，任意失败则事务回滚
4. 客户端 检查photos（检查所有记录，不要按照space_id筛选，因为可能有其他空间本地照片post失败的情况）：
  1. remote_url为空，说明是你添加的图片。你需要post该photo，服务器会把remote_url填入服务器的数据库，你下次sync则会得到该remote_url。
  2. 前端需处理photo表查到photo记录，但local_uri为空的异常情况
  3. 这意味着，事实上photo只有create和delete，不会有update

