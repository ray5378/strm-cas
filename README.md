# strm-cas

独立的 `.cas` 生成器项目，兼容当前 OpenList-CAS / 189PC 使用的 CAS 生成规则。

现在项目包含两部分：

1. **CLI 扫描器**：扫描 `/strm`、下载、生成 `.cas`、写状态库
2. **Web 控制台**：查询 API + 前端页面

---

## 一、Docker 部署

### 基础镜像

已按你的要求使用 **Alpine** 作为运行基础镜像。

构建是多阶段：

- 构建阶段：`golang:1.25-alpine`
- 运行阶段：`alpine:3.22`

### 构建镜像

```bash
cd /root/.openclaw/workspace/strm-cas
docker build -t strm-cas:latest .
```

### 启动 Web 控制台

```bash
docker compose up -d --build
```

默认访问：

```text
http://127.0.0.1:18457/
```

### 手动执行一次扫描

```bash
docker compose run --rm strm-cas-job
```

或者显式执行：

```bash
docker compose run --rm strm-cas-job strm-cas -scan-strm
```

---

## 二、持久化目录说明

容器内会使用这三个目录：

- `/strm`：放 `.strm` 文件
- `/cache`：放未完成下载的临时文件 `.part`
- `/download`：放状态库、日志、最终生成的 `.cas`

### docker-compose 默认挂载

```yaml
volumes:
  - ./data/strm:/strm
  - ./data/cache:/cache
  - ./data/download:/download
```

### 推荐说明

#### 1. `/strm`
建议挂载你实际存放 `.strm` 的目录。

#### 2. `/cache`
建议单独持久化。
这样容器重启后，未完成下载还有机会继续续传。

#### 3. `/download`
必须持久化。
因为这里会保存：

- 生成出来的 `.cas`
- `strm-cas.db`
- `strm-cas-summary.json`

如果这个目录不持久化：
- 数据库会丢
- `.cas` 会丢
- 日志会丢

---

## 三、CLI 能力

### 默认行为

- 递归扫描 `/strm` 下所有 `.strm`
- 读取 `.strm` 内的 HTTP/HTTPS 链接
- 严格串行下载，一次只下载一个链接
- 未完成下载保存在 `/cache`
- 完整下载后移动到 `/download`
- 在 `/download` 的同级目录生成同名源文件的 `.cas`
- 生成完成后删除下载下来的原文件
- 已有 `.cas` 自动跳过
- 单个任务失败不中断整个批次
- 输出 JSON 汇总日志
- 使用本地状态数据库统计 `/strm` 当前未处理 / 已处理数量

### 默认状态库

```text
/download/strm-cas.db
```

### 查看统计

```bash
go run ./cmd/strm-cas -stats
```

### 扫描并处理

```bash
go run ./cmd/strm-cas -scan-strm
```

---

## 四、Web 控制台

### 启动 API + 前端

```bash
go run ./cmd/strm-cas-api
```

默认监听：

```text
:18457
```

浏览器打开：

```text
http://127.0.0.1:18457/
```

### 页面功能

页面支持：

- 展示 `/strm` 总 `.strm` 数
- 展示还没处理数量
- 展示已生成 `.cas` 数量
- 展示失败数量
- 展示跳过数量
- 支持状态筛选
- 支持搜索
- 支持详情读取
- 支持“开始扫描 /strm”按钮
- 支持前端清理数据库按钮（带二级确认）
- 支持显示当前任务进度
- 支持分页显示已下载任务
- 支持分页显示已完成任务

### 前端结构

前端是按模块拆分的，不是巨石文件：

- `cmd/strm-cas-api/web/assets/api.js`
- `cmd/strm-cas-api/web/assets/store.js`
- `cmd/strm-cas-api/web/assets/components.js`
- `cmd/strm-cas-api/web/assets/app.js`

---

## 五、API 概览

### 统计总览

```http
GET /api/overview
```

### 数据库记录分页 + 筛选

```http
GET /api/records?status=failed&search=test&page=1&page_size=10
```

### 单条详情

```http
GET /api/records/detail?path=/strm/a/b/test.strm
```

### 当前运行态

```http
GET /api/runtime
```

### 已下载任务分页

```http
GET /api/runtime/downloaded?page=1&page_size=10
```

### 已完成任务分页

```http
GET /api/runtime/completed?page=1&page_size=10&status=done
```

### 开始扫描

```http
POST /api/scan/start
```

### 清理数据库

```http
POST /api/db/clear
```

---

## 六、可配置环境变量

- `STRM_CAS_LISTEN`，默认 `:18457`
- `STRM_CAS_STRM_ROOT`，默认 `/strm`
- `STRM_CAS_CACHE_DIR`，默认 `/cache`
- `STRM_CAS_DOWNLOAD_DIR`，默认 `/download`
- `STRM_CAS_DB_PATH`，默认 `/download/strm-cas.db`
- `STRM_CAS_LOG_PATH`，默认 `/download/strm-cas-summary.json`
- `STRM_CAS_HTTP_TIMEOUT`，例如 `30m`

---

## 七、CAS 结构

```json
{
  "name": "movie.mkv",
  "size": 123456789,
  "md5": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "sliceMd5": "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy",
  "create_time": "1745540000"
}
```

最终文件内容为：

- `base64(JSON)`

## 八、189PC 分片规则

- `<= 10 MiB * 999`：分片大小 `10 MiB`
- `> 10 MiB * 999 && <= 10 MiB * 2 * 999`：分片大小 `20 MiB`
- 更大文件：按最多约 `1999` 片倒推，且最小 `50 MiB`
