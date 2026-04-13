# Sync design

本文详细讲述项目的同步机制。

## watermelonDB提供了什么？

1. 软删除，根据[官方文档](https://watermelondb.dev/docs/CRUD#delete-a-record)，由于涉及同步，需要调用`await somePost.markAsDeleted()`实现软删除。
2. `synchronize()`函数，这个函数中有两个*callback*函数，`pullChanges`和`pushChanges`，前端可以：
    1. 实现*callback*函数中调用API的逻辑
    2. 定义一个`MyFunc()`，调用实现了*callback*的`synchronize()`函数。然后在需要的场景，例如按了一个按钮后，触发`MyFunc()`
3. API规范
4. 实现*callback*函数中调用API的逻辑
5. 定义一个`MyFunc()`，调用实现了*callback*的`synchronize()`函数。然后在需要的场景，例如按了一个按钮后，触发`MyFunc()`
6. API规范

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

假设以空间为同步单位，用户点击"同步"后，同步过程如下：

1. 客户端 `POST spaces`，参数userid和spaceid
2. 客户端 `GET sync`, 参数user_id, space_id, last_pulled_at.
3. 服务器的处理方式如下：
    1. 对于user，space, space_members表：
        - 先根据space_id做筛选，选出space内的user，space，space_members
        - 直接把筛选出来的数据塞到changes的created里
    2. 对于其他数据表：
        - 先根据space_id做筛选，选出space内的数据
        - 根据last_pulled_at分别往changes里塞相应的created，updated，deleted
    3. 返回此回复。注意服务器要把客户端没有的字段删去。
4. 客户端接收到回复，watermelonDB处理此回复
5. 客户端 `POST sync`，
    1. **第一类：`users` / `spaces` / `space_members`（核心关系表）**
        - 这三张表是空间同步的**核心关系表**，表示「空间是谁、空间里有哪些人、用户昵称与空间名称」等，**不按**普通业务数据表的增量规则处理。
        - **不参与 delete**：客户端不会对它们发 `deleted`；若全局 `changes` 中误带 `deleted`，服务端**忽略**。业务上用户离开空间仅在本地不再展示，不触发对这三张表的真实删除同步。
        - **不做 conflict 检查**，**不做** `last_modified` 比较；对 `created` / `updated` 中的记录在通过基础校验（ID 非空、`users.id` / `spaces.id` 与 `space_members` 相关 ULID、`space_members` 的 `space_id` / `user_id` 指向已存在记录）后直接更新到数据库。(注：目前这三张关系表没有last_modified等字段，所以也就不存在各种检查，目前服务器直接做无脑接受。后面需要做设计改进)
        - `space_members.id` 服务端按 `{space_id}_{user_id}` **规范化**。
        - Pull 时这三张表的数据**只出现在**对应 `changes` 的 `created` 中（`updated` / `deleted` 为空即可）。
    2. **第二类：`photos` / `expenses` / `comments` / `posts`（普通数据表）**
        - 按当前 **WatermelonDB 风格增量同步**处理 `created` / `updated` / `deleted`。
        - `updated` 时若服务端该行的 `last_modified > last_pulled_at`，返回 **409 conflict**。
        - 使用 `deleted_at`、`last_modified`、`server_created_at` 等辅助后续 Pull 的 `created` / `updated` / `deleted` 分类。
    3. **全局 push 与按空间隔离**：WatermelonDB 生成的是**全局** `changes`，不会自动按 `space_id` 过滤；**当前阶段允许**直接全局 `POST`，服务端按记录自身字段与约束落库。真正的「按空间隔离」主要体现在 Pull：`GET /api/v1/sync?space_id=...` 只返回该空间相关的上述各表数据。后续如需可在 `pushChanges` 前按当前空间预过滤。
    4. 其他：`photo` 的 `remote_url` 可能为空；整体成功返回 `200` 与 `{ "ok": true }`，失败则事务回滚。
6. 客户端 检查photos（检查所有记录，不要按照space_id筛选，因为可能有其他空间本地照片post失败的情况）：
    - remote_url为空，说明是你添加的图片。你需要post该photo，服务器会把remote_url填入服务器的数据库，你下次sync则会得到该remote_url。
    - 前端需处理photo表查到photo记录，但local_uri为空的异常情况
    - 这意味着，事实上photo只有create和delete，不会有update
