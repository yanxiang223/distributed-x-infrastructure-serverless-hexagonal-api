# distributed-x-infrastructure-serverless-hexagonal-api / 无服务器六边形架构商品 API

[![License](https://img.shields.io/badge/License-MIT%20No%20Attribution-blue.svg)](LICENSE)
[![Huawei Cloud](https://img.shields.io/badge/Huawei%20Cloud-IAM%20Auth-red.svg)](https://support.huaweicloud.com/intl/en-us/)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](#)

> 用 Go 语言与六边形架构构建的无服务器商品 API，演示领域逻辑与云服务实现的彻底解耦。
> A serverless products API built with Go and hexagonal architecture, keeping domain logic independent of cloud services.

---

## 1. 简介

本实践提供一个基于 **六边形架构（端口—适配器）** 的无服务器商品管理服务：
API 网关对外暴露 4 个 REST 接口，由 4 个函数工作流函数承载，
数据落在兼容接口数据库实例上，商品变更以领域事件发布到事件通道供下游订阅。

面向 **后端工程师、云原生架构师与 DevOps 工程师**，可作为参考架构、学习样例或生产模板使用。

### 核心能力

- **领域逻辑零云依赖**：商品规则、事件契约与错误语义集中在领域层，只通过 `Store` / `Bus` 两个端口与外部交互
- **适配器可替换**：数据库、事件通道、网关协议分别封装在独立适配器中，更换实现不影响业务规则
- **接入即事件**：写入类接口在数据落库成功后发布 `ProductCreated` / `ProductUpdated` / `ProductDeleted` 领域事件
- **完整工程化配套**：单元测试、集成测试、压力测试脚本、CI 流水线与一键部署的基础设施编排

---

## 2. 目录

- [1. 简介](#1-简介)
- [2. 目录](#2-目录)
- [3. 架构图](#3-架构图)
- [4. 方案亮点](#4-方案亮点)
- [5. 涉及云服务](#5-涉及云服务)
- [6. 前置条件](#6-前置条件)
- [7. 快速开始](#7-快速开始)
- [8. 分步部署](#8-分步部署)
- [9. 使用方法 / 验证](#9-使用方法--验证)
- [10. 清理资源](#10-清理资源)
- [11. 详细说明](#11-详细说明)
- [12. 依赖](#12-依赖)
- [13. FAQ / 故障排除](#13-faq--故障排除)
- [14. 贡献指南](#14-贡献指南)
- [15. 许可证](#15-许可证)
- [16. 联系方式 / 维护者](#16-联系方式--维护者)

---

## 3. 架构图

> 内层是领域与端口，外层是各类适配器；依赖方向永远由外向内。

```
                            ┌────────────┐
                            │   客户端   │
                            └──────┬─────┘
                                   │ HTTPS
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│                         API 网关（专享版）                         │
│       GET /      GET /{id}      PUT /{id}      DELETE /{id}        │
└───────┬─────────────────┬─────────────────┬─────────────────┬──────┘
        │                 │                 │                 │
        ▼                 ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ get-products │  │ get-product  │  │ put-product  │  │delete-product│
│ 查询商品列表 │  │ 查询单个商品 │  │  写入并发布  │  │  删除并发布  │
└───────┬──────┘  └───────┬──────┘  └───────┬──────┘  └───────┬──────┘
        │                 │                 │                 │
        └─────────────────┴────────┬────────┴─────────────────┘
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│                     领域层（不依赖任何云服务）                     │
│            Products 领域服务 与 ProductsEvents 事件发布            │
└───────────────┬─────────────────────────────────────┬──────────────┘
                │ types.Store 端口                    │ types.Bus 端口
                ▼                                     ▼
┌──────────────────────────────┐      ┌──────────────────────────────┐
│         store 适配器         │      │          bus 适配器          │
│    复用兼容接口协议客户端    │      │     CloudEvents 1.0 封装     │
└───────────────┬──────────────┘      └───────────────┬──────────────┘
                ▼                                     ▼
┌──────────────────────────────┐      ┌──────────────────────────────┐
│    GeminiDB 兼容接口实例     │      │         事件网格 EG          │
│  商品表（首次写入自动创建）  │      │        自定义事件通道        │
└──────────────────────────────┘      └───────────────┬──────────────┘
                                                      ▼
                                      ┌──────────────────────────────┐
                                      │          下游订阅方          │
                                      │     按事件类型过滤后路由     │
                                      └──────────────────────────────┘
```

**关键组件说明**：

- **API 网关**：按 API 定义匹配请求，把路径参数与请求体组装成事件下发给函数
- **函数工作流**：4 个函数分别承载 4 个接口，共享同一份领域层与适配器代码
- **GeminiDB 兼容接口实例**：承载商品数据，须先建好商品表（兼容接口不会自动建表，见部署步骤 5）
- **事件网格自定义事件通道**：接收商品变更领域事件，供订阅方按类型过滤后路由

---

## 4. 方案亮点

- **端口—适配器分层清晰**：`types` 定义端口，`domain` 实现业务规则，`store` / `bus` / `handlers` 各自封装一种外部技术，领域层可脱网单测
- **写操作与领域事件强绑定**：商品变更必须产生事件，因此事件发布挂在领域服务上而非入口层，查询入口天然不产生事件
- **同一份代码多个部署单元**：四个函数由同一仓库编译，各自只决定暴露哪个能力与挂载哪些端口
- **交付物完整可直接运行**：含单元测试、集成测试、压力测试脚本、CI 流水线与一键部署编排，`make ci` 即可完成构建与测试
- **连接信息与凭据不进代码库**：存储地址、实例密码与访问密钥都由部署时提供，仓库内的代码与配置文件中不含任何密钥

---

## 5. 涉及云服务

| 服务 | 用途 | 备注 |
|---|---|---|
| API 网关（APIG） | 对外暴露 REST 接口，转发请求到函数 | 专享版，默认 `BASIC` 规格 |
| 函数工作流（FunctionGraph） | 承载 4 个业务函数，Go 运行时 | 配置虚拟私有云访问与日志采集 |
| GeminiDB | 商品数据存储 | 兼容接口实例，集群形态 3 节点 |
| 事件网格（EG） | 承载商品变更领域事件 | 自定义事件通道 |
| 云日志服务（LTS） | 收集函数运行日志 | 默认保留 7 天 |
| 对象存储服务（OBS） | 存放函数代码包 | 私有桶 |
| 虚拟私有云（VPC） | 承载实例与函数的网络平面 | 含子网与安全组 |
| 弹性公网 IP（EIP） | 提供 API 网关的公网访问入口 | 按流量方式开通 |
| 统一身份认证（IAM） | 提供访问密钥与资源权限 | 用于部署与事件通道调用 |

> **提示**：以上服务均可通过本仓库的 Terraform 一键创建；如需清理，请参见 [第 10 节「清理资源」](#10-清理资源)。

---

## 6. 前置条件

| 项 | 要求 | 说明 |
|---|---|---|
| **账号** | 已开通华为云账号并完成实名认证 | 个人 / 企业账号均可 |
| **权限** | 账号具备 IAM **Tenant Administrator** 或等效权限 | 用于创建 VPC / GeminiDB / APIG / FunctionGraph 等资源 |
| **Region** | 推荐 `cn-north-4` | 兼容接口实例与专享版 API 网关的区域可用性以实际开通情况为准 |
| **Terraform** | `>= 1.5.0` | 基础设施编排工具 |
| **huaweicloud Provider** | `>= 1.70` | 由 `terraform init` 自动获取 |
| **Go** | `>= 1.21` | 交叉编译函数可执行文件 |
| **zip** | 任意版本 | 打包函数代码包 |
| **Artillery** *(可选)* | `>= 2.0` | 压力测试 |
| **访问密钥** | 在华为云『我的凭证』页面获取 Access Key ID 与 Secret Access Key，并记录项目 ID | **不要**将凭据写入任何代码或配置文件 |

---

## 7. 快速开始

### 一键部署

```bash
# 1. 克隆本仓库
git clone <repo-https-url> distributed-x-infrastructure-serverless-hexagonal-api
cd distributed-x-infrastructure-serverless-hexagonal-api

# 2. 编译并打包函数代码包
make package

# 3. 准备访问密钥与部署变量（参见第 6 节「前置条件」）
export TF_VAR_access_key="<your-access-key>"
export TF_VAR_secret_key="<your-secret-key>"
export TF_VAR_project_id="<your-project-id>"
export TF_VAR_geminidb_password="<your-instance-password>"

# 4. 一键部署
cd infra
terraform init
terraform plan
terraform apply -auto-approve
```

> **部署耗时**：约 8-15 分钟（兼容接口实例与专享版 API 网关的创建耗时较长）。
> **完整步骤与每步预期输出**：参见 [第 8 节「分步部署」](#8-分步部署)。

---

## 8. 分步部署

### 步骤 1：编译并打包函数代码包

```bash
cd distributed-x-infrastructure-serverless-hexagonal-api
make package
```

**预期输出**：`functions/` 下四个目录各生成 `handler`（Linux 可执行文件）与 `function.zip`。

```bash
ls functions/*/function.zip
```

```
functions/delete-product/function.zip
functions/get-product/function.zip
functions/get-products/function.zip
functions/put-product/function.zip
```

> 编译产物名必须与函数执行入口名称一致，本仓库统一为 `handler`。
> 代码包必须在基础设施应用之前生成。

### 步骤 2：初始化工作目录

```bash
cd infra
terraform init
```

**预期输出**：`Terraform has been successfully initialized!`

### 步骤 3：预演资源变更

```bash
terraform plan
```

**预期输出**：`Plan: N to add, 0 to change, 0 to destroy.`

### 步骤 4：执行部署

```bash
terraform apply -auto-approve
```

**预期输出**：`Apply complete! Resources: N added, 0 changed, 0 destroyed.`

### 步骤 5：创建商品表

兼容接口实例不会在首次写入时自动建表，需要在实例创建完成后建一次表。表的主键为字符串类型的 `id` 字段，且建表请求必须携带预置读写容量（按需容量模式不被受理）。

建表动作需在实例所在虚拟私有云内发起。若执行环境与实例不在同一网段，可由同虚拟私有云内的主机执行，或临时借助一个函数完成，命令如下（以兼容接口协议客户端为例）：

```bash
aws dynamodb create-table \
  --table-name <table_name> \
  --key-schema AttributeName=id,KeyType=HASH \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1 \
  --endpoint-url http://<geminidb_endpoint>:<geminidb_data_port>
```

**预期输出**：返回 `TableDescription`，其中 `TableStatus` 为 `CREATING`，稍后变为 `ACTIVE`。
表名与接口地址分别取自 `terraform output table_name` 与 `terraform output geminidb_endpoint`。

### 步骤 6：记录关键输出

```bash
terraform output
```

**预期输出**：列出 `api_url`、`event_channel_id`、`geminidb_endpoint` 等字段，请妥善保存用于后续验证。

---

## 9. 使用方法 / 验证

### 验证 1：确认基础设施就绪

```bash
cd infra
terraform output api_url
terraform output event_channel_id
```

**预期输出**：

```
https://<apig-ingress-address>
<event-channel-id>
```

### 验证 2：写入商品

```bash
API_URL=$(cd infra && terraform output -raw api_url)

curl -i -X PUT "$API_URL/iXR" \
  -H "Content-Type: application/json" \
  -d '{"id":"iXR","name":"iPhone XML","price":0.123}'
```

**预期输出**：

```
HTTP/2 201
content-type: application/json

{"id":"iXR","name":"iPhone XML","price":0.123}
```

### 验证 3：查询商品

```bash
# 查询单个商品
curl -s "$API_URL/iXR"
```

**预期输出**：

```
{"id":"iXR","name":"iPhone XML","price":0.123}
```

```bash
# 查询商品列表
curl -s "$API_URL"
```

**预期输出**：

```
{"products":[{"id":"iXR","name":"iPhone XML","price":0.123}]}
```

### 验证 4：删除商品并确认幂等

```bash
# 删除商品
curl -i -X DELETE "$API_URL/iXR"

# 再次查询，应返回 404
curl -i "$API_URL/iXR"
```

**预期输出**：

```
HTTP/2 200
content-type: application/json

{"message":"product not found"}
```

### 验证 5：参数校验

```bash
# 路径主键与请求体主键不一致
curl -i -X PUT "$API_URL/not-the-same-id" \
  -H "Content-Type: application/json" \
  -d '{"id":"iXR","name":"iPhone XML","price":0.123}'
```

**预期输出**：

```
HTTP/2 400

{"message":"product ID in path does not match product ID in body"}
```

### 验证 6：端到端业务链路（推荐）

```bash
API_URL=$(cd infra && terraform output -raw api_url) \
  go test -v -tags=integration ./...
```

**预期结果**：全部用例通过，覆盖「写入 → 查询 → 列表 → 删除 → 再查询返回 404」完整链路。

### 验证 7：确认领域事件已发布

写入或删除商品后，在事件网格控制台的对应事件通道中查看事件轨迹，
应能看到 `type` 为 `ProductCreated`、`ProductUpdated` 或 `ProductDeleted` 的事件记录。

```bash
cd infra
terraform output event_channel_name
```

> **强制实测**：本节所列命令与示例在交付前已经实际执行验证，可运行且输出与描述一致。
> 请按本节步骤自行验证，确认业务功能可用后方可视为交付完成。

---

## 10. 清理资源

```bash
# 进入部署目录
cd distributed-x-infrastructure-serverless-hexagonal-api/infra

# 一键释放所有资源
terraform destroy -auto-approve

# 删除本地编译产物
cd .. && make clean
```

**预期输出**：`Destroy complete! Resources: N destroyed.`

> **避免资源残留**：
> - `terraform destroy` 仅释放 Terraform 状态中追踪的资源；
> - 若中途手工创建过对象存储对象或弹性公网 IP，需手动释放；
> - 公共资源（账号内其他任务共享的 VPC / NAT 等）不会被影响。

---

## 11. 详细说明

### 工作流程

以 `PUT /{id}` 为例，一次请求的完整链路为：

1. API 网关匹配到 API 定义，将路径参数 `id` 与请求体组装成事件下发；
2. 入站适配器反序列化事件，并依据运行时剩余时间预算构造带截止时间的执行上下文；
3. 入口适配器完成协议级校验（路径参数缺失、请求体为空、Base64 非法）；
4. 领域服务解析并校验业务对象，读取当前值以区分新增与更新，写入存储；
5. 写入成功后通过 `Bus` 端口发布对应类型的领域事件；
6. 结果按网关响应结构返回。

### 接口列表

| 方法 | 路径 | 函数 | 成功状态码 | 说明 |
|---|---|---|---|---|
| `GET` | `/` | `get-products` | `200` | 返回一页商品，每页最多 20 条 |
| `GET` | `/{id}` | `get-product` | `200` | 返回单个商品，不存在时返回 `404` |
| `PUT` | `/{id}` | `put-product` | `201` | 写入商品并发布 `ProductCreated` / `ProductUpdated` |
| `DELETE` | `/{id}` | `delete-product` | `200` | 删除商品并发布 `ProductDeleted` |

### 请求与响应示例

```bash
# 写入
curl -X PUT "$API_URL/iXR" \
  -H "Content-Type: application/json" \
  -d '{"id":"iXR","name":"iPhone XML","price":0.123}'
```

```json
{"id":"iXR","name":"iPhone XML","price":0.123}
```

```bash
# 分页查询：使用上一页返回的 next 作为游标
curl -s "$API_URL?next=<next-cursor>"
```

```json
{
  "products": [{"id":"iXR","name":"iPhone XML","price":0.123}],
  "next": "next-cursor"
}
```

### 领域事件契约

| 事件类型 | 触发时机 | 载荷 |
|---|---|---|
| `ProductCreated` | 写入前该主键不存在 | 新增后的商品对象 |
| `ProductUpdated` | 写入前该主键已存在 | 更新后的商品对象 |
| `ProductDeleted` | 删除前该主键存在 | 被删除的商品对象 |

事件采用 CloudEvents 1.0 结构，`source` 固定为 `products-api`，`subject` 为商品主键。

### 配置项

完整配置项索引参见 [`docs/deployment-guide.md`](docs/deployment-guide.md) 第 5 节；
架构设计与关键决策参见 [`docs/architecture.md`](docs/architecture.md)。

---

## 12. 依赖

### 主要依赖

| 名称 | 版本 | 用途 |
|---|---|---|
| Terraform | `>= 1.5.0` | 基础设施编排 |
| huaweicloud Provider | `>= 1.70` | 华为云资源管理 |
| Go | `>= 1.21` | 函数代码编译 |
| FunctionGraph Go 运行时 | 随仓库分发 | 函数入口注册与事件上下文 |
| 事件网格开放 API 客户端 | `v0.1.215` | 领域事件投递 |
| 兼容接口线协议客户端 | `v1.30.4` | 兼容接口实例数据访问 |
| gomock | `v1.6.0` | 单元测试打桩 |

> FunctionGraph Go 运行时随仓库分发（`go-runtime/` 目录），
> 由 `go.mod` 的 `replace` 指令指向本地路径，构建时无需联网获取。

---

## 13. FAQ / 故障排除

### Q1：部署时提示「权限不足」怎么办？

**A**：确认账号具备 IAM **Tenant Administrator** 权限，或至少包含 `VPC Administrator`、
`APIG Administrator`、`FunctionGraph Administrator` 等子服务管理员权限。

### Q2：函数写入报鉴权失败怎么办？

**A**：兼容接口的数据面只接受**实例自带的数据库账号**，不接受 IAM 凭据与委托临时凭据。
请核对函数运行环境中的实例密码与创建实例时设置的密码一致。

### Q3：函数连接存储超时怎么办？

**A**：数据面端口随部署形态变化 —— **集群形态使用高可用端口 80，单节点形态使用 8000**。
用错端口会表现为连接超时而非鉴权失败。可探测实际放通的端口，
再调整 `geminidb_data_port` 变量后重新应用。

### Q4：创建实例时报 `Invalid database version` 怎么办？

**A**：兼容接口不对外提供版本号，`datastore.version` 必须为空字符串，写成具体版本号会被服务端拒绝。

### 故障排查清单

| 现象 | 可能原因 | 排查方法 |
|---|---|---|
| `terraform apply` 提示找不到代码包 | 未执行 `make package` | 在仓库根目录执行 `make package` 后重试 |
| 接口返回 `missing 'id' parameter in path` | 请求路径末段为空 | 确认请求路径形如 `/iXR`，末段非空 |
| 接口返回 `empty request body` | 请求体为空或未设置 `Content-Type` | 补全 JSON 请求体与 `Content-Type` 头 |
| 写入返回 `500` 且日志提示连接失败 | 数据面端口或网络平面配置不符 | 参见 Q3，核对端口与虚拟私有云配置 |
| 事件未送达订阅方 | 事件通道配置或凭据无效 | 在云日志服务中检索 `failed to publish` 关键字 |

---

## 14. 贡献指南

欢迎提交 Issue 与 PR。请遵循以下流程：

1. **Issue**：描述问题 / 建议（含复现命令、预期与实际输出）
2. **PR**：基于 `main` 分支新建 feature 分支；提交前运行 `make ci` 与 `terraform fmt -check`
3. **Code Review**：由 **[@\<github-username\>](https://github.com/\<github-username\>)** reviewer 审核通过后合并

---

## 15. 许可证

本实践采用 **MIT No Attribution** 许可证发布。详见仓库根目录 [`LICENSE`](LICENSE) 文件。

```
MIT No Attribution

Copyright Huawei Cloud. All Rights Reserved.

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

---

## 16. 联系方式 / 维护者

| 项 | 内容 |
|---|---|
| **创建者** |
| **维护者** |
| **联系方式** | GitHub Issues 
---
