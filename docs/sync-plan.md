# WatermelonDB Sync 实施计划

## 一、WatermelonDB Sync 机制梳理

### 核心思想

WatermelonDB 是本地优先的数据库，数据主要在客户端读写，服务器只是"汇总站"。Sync 要解决的问题是：**多个客户端的数据如何通过服务器保持一致**。

### 同步过程

一次完整的同步分两步，**严格按顺序执行**：

```
Pull（拉）→ Push（推）
```

客户端调用一次 `synchronize()`，这两步自动依次执行。

### Pull（拉取）

客户端问服务器："从时间 T 以来，服务器上有什么变化？"

请求：

```
GET /sync?last_pulled_at=T
```

- `last_pulled_at`：上次成功 Pull 时服务器返回的时间戳，首次同步时为 `null` 或 `0`
- 服务器返回**所有表**在时间 T 之后的增量变化

服务器必须返回：

```json
{
  "changes": {
    "表名1": {
      "created": [{ "id": "xxx", "col1": "val1", ... }],
      "updated": [{ "id": "yyy", "col1": "new_val", ... }],
      "deleted": ["id_zzz", "id_www"]
    },
    "表名2": {
      "created": [],
      "updated": [],
      "deleted": []
    }
  },
  "timestamp": 1700000000123
}
```

要点：

- `created`：自 T 以来新建的记录，**完整的所有字段**
- `updated`：自 T 以来被修改的记录，**完整的所有字段**（不是只传改了的字段）
- `deleted`：自 T 以来被删除的记录，**只传 ID**
- `timestamp`：**服务器当前时间**，客户端存下来，下次 Pull 时作为 `last_pulled_at` 传上来
- 字段名必须用 schema 定义中的原始名（snake_case）
- **不要**返回 WatermelonDB 内部字段 `_status` 和 `_changed`

客户端收到后，WatermelonDB **自动**把这些变化应用到本地数据库：

- `created` → 本地 INSERT（已存在则 UPDATE）
- `updated` → 本地 UPDATE（不存在则 CREATE）
- `deleted` → 本地永久删除

### Push（推送）

Pull 完成后，客户端把**本地未推送的变化**发给服务器。

请求：

```
POST /sync?last_pulled_at=T
Body: { 客户端的 changes 对象，格式同 Pull 返回的 changes }
```

服务器要做的事：

1. 遍历每张表的 created/updated/deleted
2. `created` → INSERT（如果 ID 已存在则 UPDATE，不报错）
3. `updated` → UPDATE（如果不存在则 INSERT，不报错）
4. `deleted` → DELETE（不存在则忽略）
5. **冲突检测**：如果某条记录在服务器上被修改过的时间晚于 `last_pulled_at`，说明 Pull 之后有别人改了这条记录 → 返回错误 → 客户端重新 Pull 再 Push
6. 全部成功返回 200；任何失败则事务回滚，返回错误

### 完整时序示例

```
时间线 ─────────────────────────────────────────────────►

客户端A:  [离线创建了一条expenses记录]
          |
          |  触发同步
          |
          ├─ Pull ──► GET /sync?last_pulled_at=0
          |            返回: {changes: {全部数据}, timestamp: T1}
          |            本地应用changes
          |
          ├─ Push ──► POST /sync
          |            Body: {expenses: {created: [{id:x, ...}], ...}}
          |            服务器保存，返回 200
          |
          同步完成，记录 lastPulledAt = T1


客户端B:  [在线，之前同步过，lastPulledAt = T1]
          |
          |  触发同步
          |
          ├─ Pull ──► GET /sync?last_pulled_at=T1
          |            服务器查 T1 之后的变化 → 发现客户端A推的那条expenses
          |            返回: {changes: {expenses: {created: [{id:x,...}]}}, timestamp: T2}
          |            本地应用 → 客户端B现在也有那条expenses了
          |
          ├─ Push ──► 没有本地变化，changes 为空
          |
          同步完成，记录 lastPulledAt = T2
```

### 服务器端怎么追踪"变化"

这是服务器实现的核心难题。官方推荐：

**每张表加 `last_modified` 字段**：

- 服务器每次 INSERT 或 UPDATE 时，自动设为服务器当前时间戳
- Pull 时查 `last_modified > lastPulledAt` 就能找到所有变化

**怎么区分 created 和 updated**：

- 方案 A：加 `server_created` 字段，服务器 INSERT 时设为 `NOW()`。Pull 时，如果 `server_created > lastPulledAt` → `created`；否则 → `updated`（推荐，精确）
- 方案 B：不区分，全部作为 `updated` 返回，前端加 `sendCreatedAsUpdated: true`。更简单，但边界情况处理稍弱

**怎么追踪删除**：

- 方案 A：软删除（`deleted_at` 或 `is_deleted`），Pull 时查被标记删除的记录（推荐，本项目已有 `deleted_at`）
- 方案 B：墓碑表，单独一张 `deleted_records` 表记录被删记录的 ID 和时间戳

### 前端要做什么

前端核心代码：

```typescript
import { synchronize } from '@nozbe/watermelondb/sync'

await synchronize({
  database,
  pullChanges: async ({ lastPulledAt, schemaVersion, migration }) => {
    // 请求 GET /sync 端点，返回 { changes, timestamp }
  },
  pushChanges: async ({ changes, lastPulledAt }) => {
    // 请求 POST /sync 端点
  },
  migrationsEnabledAtVersion: 1,
})
```

WatermelonDB **自动**处理：

- 从本地数据库收集未推送的变化（通过内部的 `_status` 和 `_changed` 字段）
- 应用 Pull 下来的 changes 到本地数据库
- 冲突解决（默认：服务器版本覆盖本地版本）

不需要手动操作本地数据库的增删改，`synchronize()` 全包了。

---

## 二、结合项目数据库设计的实施计划

项目的数据设计定义了 7 张表：`users`, `spaces`, `space_members`, `photos`, `expenses`, `comments`, `posts`。

### 步骤 1：服务器端建表 + GORM 模型

#### 为什么需要额外的服务器字段

客户端表的 `created_at` / `updated_at` 是 WatermelonDB 管理的**客户端字段**（客户端时钟设的），不可靠。服务器需要**额外的服务器专用字段**作为 Sync 的时间基准。

| 字段 | 谁设置 | 用途 |
|---|---|---|
| `last_modified` | 服务器，每次 INSERT/UPDATE 时 `NOW()` | Pull 增量查询 |
| `server_created` | 服务器，INSERT 时 `NOW()` | 区分 created vs updated |
| `deleted_at` | 已有字段，软删除 | Pull 的 deleted 列表 |

#### 以 expenses 为例的 SQL 建表

```sql
CREATE TABLE expenses (
  id              VARCHAR(26) PRIMARY KEY,   -- ULID
  space_id        VARCHAR(26) NOT NULL,
  payer_id        VARCHAR(26) NOT NULL,
  amount          DECIMAL(10,2) NOT NULL,
  description     TEXT NOT NULL,
  created_at      BIGINT NOT NULL,           -- 客户端设的
  updated_at      BIGINT NOT NULL,           -- 客户端设的
  deleted_at      BIGINT,                    -- 软删除

  -- 服务器管理字段，客户端不知道这些字段的存在
  last_modified   BIGINT NOT NULL,           -- 服务器每次写入时 NOW()
  server_created  BIGINT NOT NULL            -- 服务器首次 INSERT 时 NOW()
);
```

其他 6 张表同理，每张都加 `last_modified` 和 `server_created`。

#### Go GORM 模型示例

```go
type Expense struct {
    ID            string  `gorm:"primaryKey"`
    SpaceID       string  `gorm:"column:space_id"`
    PayerID       string  `gorm:"column:payer_id"`
    Amount        float64 `gorm:"column:amount"`
    Description   string  `gorm:"column:description"`
    CreatedAt     int64   `gorm:"column:created_at"`
    UpdatedAt     int64   `gorm:"column:updated_at"`
    DeletedAt     *int64  `gorm:"column:deleted_at"`
    LastModified  int64   `gorm:"column:last_modified"`
    ServerCreated int64   `gorm:"column:server_created"`
}
```

### 步骤 2：服务器端实现 Pull 端点（`GET /api/v1/sync`）

伪代码：

```
输入: last_pulled_at (毫秒时间戳，首次为 0)
输出: { changes: {...}, timestamp: NOW() }

timestamp = current_server_time_ms()
changes = {}

对每张表 (users, spaces, space_members, photos, expenses, comments, posts):

  // 查找变化了的记录（未删除的）
  rows = SELECT * FROM 表名
         WHERE last_modified > last_pulled_at
           AND deleted_at IS NULL

  created = [去掉 last_modified 和 server_created 后的完整记录
             for row in rows if row.server_created > last_pulled_at]
  updated = [去掉 last_modified 和 server_created 后的完整记录
             for row in rows if row.server_created <= last_pulled_at]

  // 查找被软删除的记录
  deleted_ids = SELECT id FROM 表名
                WHERE deleted_at IS NOT NULL
                  AND deleted_at > last_pulled_at

  changes[表名] = { created, updated, deleted: deleted_ids }

return { changes, timestamp }
```

关键点：

- 返回给客户端的记录**不要包含** `last_modified` 和 `server_created`
- `timestamp` 的取值要和查询在同一事务/锁内，保证一致性

### 步骤 3：服务器端实现 Push 端点（`POST /api/v1/sync`）

伪代码：

```
输入: changes 对象, last_pulled_at
输出: 200 或 错误

BEGIN TRANSACTION

对 changes 中的每张表:

  对 created 中的每条记录:
    if EXISTS (SELECT 1 FROM 表名 WHERE id = record.id):
      UPDATE 表名 SET <各字段>=record.各字段, last_modified=NOW() WHERE id=record.id
    else:
      INSERT INTO 表名 VALUES (record.各字段, last_modified=NOW(), server_created=NOW())

  对 updated 中的每条记录:
    if EXISTS (SELECT 1 FROM 表名 WHERE id = record.id):
      // 冲突检测
      if (SELECT last_modified FROM 表名 WHERE id=record.id) > last_pulled_at:
        ROLLBACK, 返回 409 冲突错误
      UPDATE 表名 SET <各字段>=record.各字段, last_modified=NOW() WHERE id=record.id
    else:
      INSERT INTO 表名 VALUES (record.各字段, last_modified=NOW(), server_created=NOW())

  对 deleted 中的每个 id:
    if EXISTS (SELECT 1 FROM 表名 WHERE id=id):
      UPDATE 表名 SET deleted_at=NOW(), last_modified=NOW() WHERE id=id
    else:
      忽略

COMMIT
return 200
```

关键点：

- **必须在单个事务内**，失败全部回滚
- 冲突检测是防止"覆盖别人刚推的修改"
- created 时 ID 已存在要当 UPDATE 处理；updated 时不存在要当 INSERT 处理——这两种都是正常的边缘情况

### 步骤 4：客户端补全 WatermelonDB Schema 和 Model

扩展 `schema.ts` 为 7 张表，字段与 data-design.md 对齐。**不需要**包含 `last_modified` 和 `server_created`（服务器内部字段）。

以 expenses 为例：

```typescript
// schema.ts
tableSchema({
  name: 'expenses',
  columns: [
    { name: 'space_id', type: 'string', isIndexed: true },
    { name: 'payer_id', type: 'string' },
    { name: 'amount', type: 'number' },
    { name: 'description', type: 'string' },
    { name: 'created_at', type: 'number' },
    { name: 'updated_at', type: 'number' },
  ],
})
```

```typescript
// Expense.ts
import { Model } from '@nozbe/watermelondb'
import { field, date } from '@nozbe/watermelondb/decorators'

export default class Expense extends Model {
  static table = 'expenses'
  @field('space_id') spaceId
  @field('payer_id') payerId
  @field('amount') amount
  @field('description') description
  @date('created_at') createdAt
  @date('updated_at') updatedAt
}
```

### 步骤 5：客户端实现 Sync 函数

```typescript
import { synchronize } from '@nozbe/watermelondb/sync'
import { database } from '@/model'

export async function sync() {
  const API_URL = process.env.EXPO_PUBLIC_API_URL

  await synchronize({
    database,
    pullChanges: async ({ lastPulledAt, schemaVersion, migration }) => {
      const params = new URLSearchParams({
        last_pulled_at: String(lastPulledAt ?? 0),
        schema_version: String(schemaVersion),
        migration: JSON.stringify(migration),
      })
      const resp = await fetch(`${API_URL}/api/v1/sync?${params}`)
      if (!resp.ok) throw new Error(await resp.text())
      const { changes, timestamp } = await resp.json()
      return { changes, timestamp }
    },
    pushChanges: async ({ changes, lastPulledAt }) => {
      const resp = await fetch(
        `${API_URL}/api/v1/sync?last_pulled_at=${lastPulledAt}`,
        { method: 'POST', body: JSON.stringify(changes) }
      )
      if (!resp.ok) throw new Error(await resp.text())
    },
    migrationsEnabledAtVersion: 1,
  })
}
```

### 步骤 6：联调测试

1. 启动 PostgreSQL（`docker compose up -d`）
2. 启动服务器（`go run main.go`）
3. 用客户端触发 sync，验证首次全量同步
4. 客户端离线创建记录 → 再同步 → 验证 Push
5. 另一个客户端同步 → 验证 Pull 能拿到数据
6. 制造冲突（两个客户端同时改同一条记录）→ 验证冲突检测

---

## 三、关于文件（照片/头像）同步

数据库 Sync 只同步结构化数据。照片和头像的文件传输是额外的步骤，发生在数据库 Sync 之后：

1. 数据库 Sync 完成
2. 客户端检查本地 photo/avatar 记录：
   - `local_uri` 为空但 `remote_url` 不为空 → 根据 `remote_url` 下载文件到本地，填入 `local_uri`
   - `local_uri` 不为空但 `remote_url` 为空 → 通过 `POST /api/v1/photos` 或 `POST /api/v1/avatars` 上传文件，将返回的 `remote_url` 填入本地记录
3. 文件传输完成后，再触发一次数据库 Sync 把更新后的 `local_uri` / `remote_url` 同步上去

这个流程独立于上面的数据库 Sync 机制，可以在数据库 Sync 跑通后再实现。
