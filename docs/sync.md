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
