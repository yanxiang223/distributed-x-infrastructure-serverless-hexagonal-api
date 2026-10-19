# 部署指南

## 1. 前置条件

| 项 | 要求 | 说明 |
|---|---|---|
| 华为云账号 | 已实名认证，具备 **Tenant Administrator** 或等效权限 | 需创建 VPC、GeminiDB、API 网关、函数工作流等资源 |
| 区域 | 推荐 `cn-north-4` | 兼容接口实例与专享版 API 网关的区域可用性以实际开通情况为准 |
| Terraform | `>= 1.5.0` | 基础设施编排 |
| huaweicloud Provider | `>= 1.70` | 由 `terraform init` 自动获取 |
| Go | `>= 1.21` | 交叉编译函数可执行文件 |
| zip | 任意版本 | 打包函数代码包；本机无 `zip` 时可用任一种压缩工具生成同名 `function.zip`，内容为单文件 `handler` |
| 兼容接口协议客户端 | 任意版本 | 建表用（见步骤 7）。兼容接口以该协议对外提供数据面，故沿用该协议的客户端工具 |
| Artillery | `>= 2.0`（可选） | 压测 |

## 2. 部署顺序总览

```
编译函数代码包  →  准备变量文件  →  初始化  →  预演  →  应用  →  建表  →  验证
   make package       tfvars        init      plan     apply   建表     curl
```

函数代码包必须在 `terraform apply` **之前**生成：基础设施配置会校验代码包的存在与内容摘要，
并把代码包上传到对象存储供函数拉取。

商品表必须在**首次调用写入接口之前**建好——兼容接口不会在首次写入时自动建表，见步骤 7。

## 3. 步骤详解

### 步骤 1：获取访问密钥

在华为云『我的凭证』页面获取 Access Key ID 与 Secret Access Key；
在同一页面记录**项目 ID**（函数调用事件通道时需要）。

> 密钥只通过环境变量或本地 `terraform.tfvars` 传入。`terraform.tfvars` 已被 `.gitignore` 屏蔽。

### 步骤 2：编译并打包函数代码包

```bash
cd <仓库根目录>
make package
```

**预期输出**：`functions/` 下四个函数目录各自生成 `handler`（Linux 可执行文件）与 `function.zip`。

```bash
ls functions/*/function.zip
```

**预期输出**：

```
functions/delete-product/function.zip
functions/get-product/function.zip
functions/get-products/function.zip
functions/put-product/function.zip
```

> 编译产物名必须与函数执行入口名称一致，本仓库统一为 `handler`。
> 各函数目录下的 `event.json` 是网关事件样例，可在函数工作流控制台的「测试」功能中直接使用。

### 步骤 3：准备变量文件

```bash
cd infra
cp terraform.tfvars.example terraform.tfvars
```

编辑 `terraform.tfvars`，至少填写以下四项：

| 变量 | 说明 |
|---|---|
| `access_key` / `secret_key` | 访问密钥，函数运行环境也需要 |
| `project_id` | 项目 ID |
| `geminidb_password` | 兼容接口实例的管理员密码（自行设定，需满足密码复杂度要求） |

也可改用环境变量传入，避免写入任何文件：

```bash
export TF_VAR_access_key="<your-access-key>"
export TF_VAR_secret_key="<your-secret-key>"
export TF_VAR_project_id="<your-project-id>"
export TF_VAR_geminidb_password="<your-instance-password>"
```

### 步骤 4：初始化

```bash
terraform init
```

**预期输出**：`Terraform has been successfully initialized!`

### 步骤 5：预演资源变更

```bash
terraform plan
```

**预期输出**：`Plan: N to add, 0 to change, 0 to destroy.`

### 步骤 6：应用

```bash
terraform apply -auto-approve
```

**预期输出**：`Apply complete! Resources: N added, 0 changed, 0 destroyed.`

> 首次创建兼容接口实例与专享版 API 网关需要数分钟，请耐心等待。

### 步骤 7：创建商品表

兼容接口**不会在首次写入时自动建表**，须事先建好，否则首次写入会返回
`ResourceNotFoundException: Cannot do operations on a non-existent table`。

建表请求**必须携带预置读写容量**——仅指定按需计费会被服务端拒绝
（`ValidationException: No provisioned throughput specified for the table`）。

```bash
export TABLE=products-api
export GEMINIDB_ENDPOINT="<实例负载均衡地址>:80"

aws dynamodb create-table \
  --table-name "$TABLE" \
  --attribute-definitions AttributeName=id,AttributeType=S \
  --key-schema AttributeName=id,KeyType=HASH \
  --provisioned-throughput ReadCapacityUnits=1,WriteCapacityUnits=1 \
  --endpoint-url "http://$GEMINIDB_ENDPOINT"
```

表中不存在的商品在读取时会由业务层转换为 `404`，无需额外处理。

### 步骤 8：记录输出

```bash
terraform output
```

关键输出：

| 输出 | 用途 |
|---|---|
| `api_url` | API 公网调用地址 |
| `api_vpc_ingress_address` | API 在虚拟私有云内的入口地址 |
| `event_channel_id` | 事件通道 ID，供订阅方配置订阅 |
| `geminidb_endpoint` | 兼容接口实例的负载均衡地址 |

## 4. 函数运行环境变量

变量由基础设施配置自动注入，无需手工设置。排查问题时可用于核对：

| 变量 | 用途 | 注入对象 |
|---|---|---|
| `TABLE` | 商品表名称，须按步骤 7 预先建表 | 全部函数 |
| `GEMINIDB_ENDPOINT` | 实例负载均衡地址与数据面端口 | 全部函数 |
| `GEMINIDB_REGION` | 参与请求签名的区域标识 | 全部函数 |
| `GEMINIDB_PASSWORD` | 实例管理员密码 | 全部函数 |
| `EVENT_CHANNEL_ID` | 事件通道 ID | 写入类函数 |
| `HW_REGION_NAME` | 事件网格服务区域 | 写入类函数 |
| `HW_PROJECT_ID` | 项目 ID | 写入类函数 |
| `HW_ACCESS_KEY` / `HW_SECRET_KEY` | 事件网格开放 API 的访问密钥 | 写入类函数 |
| `HW_SECURITY_TOKEN` | 临时凭据的会话令牌（使用永久密钥时不注入） | 写入类函数 |

## 5. 配置项索引

| 变量 | 默认值 | 说明 |
|---|---|---|
| `region` | `cn-north-4` | 资源部署区域 |
| `project_name` | `products-api` | 资源命名前缀，同时作为商品表名 |
| `vpc_cidr` | `10.0.0.0/16` | 虚拟私有云网段 |
| `subnet_cidr` | `10.0.1.0/24` | 子网网段 |
| `availability_zones` | `["a","b","c"]` | 可用区后缀，与区域拼接 |
| `geminidb_spec_code` | `geminidb.cassandra.large.4` | 兼容接口实例规格，以目标区域实际可售规格为准 |
| `geminidb_node_num` | `3` | 集群节点数 |
| `geminidb_storage_size` | `100` | 单节点存储容量（GB） |
| `geminidb_data_port` | `80` | 数据面端口，集群形态为 80，单节点形态为 8000 |
| `apig_edition` | `BASIC` | API 网关专享版规格 |
| `apig_available_zones` | `["cn-north-4a"]` | 网关专享实例所在可用区，须与区域实际可售资源匹配 |
| `apig_bandwidth_size` | `5` | 公网入口带宽（Mbit/s） |
| `apig_bandwidth_charging_mode` | `traffic` | 公网入口带宽分配方式 |
| `function_memory_size` | `256` | 函数内存（MB） |
| `function_timeout_read` | `10` | 查询类函数执行超时（秒） |
| `function_timeout_write` | `20` | 写入类函数执行超时（秒），需覆盖落库后的事件发布耗时 |
| `log_retention_days` | `7` | 函数日志保留天数 |
| `bind_function_log_group` | `true` | 是否把函数日志绑定到自建日志组；账号拒绝绑定时置 `false` 回退，见 6.7 |
| `obs_bucket_name` | 自动拼装 | 存放函数代码包的桶名，需全局唯一 |

## 6. 常见问题

### 6.1 函数写入报鉴权失败

兼容接口的数据面只接受**实例自带的数据库账号**，不接受 IAM 凭据与委托临时凭据。
请核对函数运行环境中的 `GEMINIDB_PASSWORD` 与实例管理员密码一致；
实例密码在建实例后不可找回，如需变更只能重置。

### 6.2 函数启动期连接存储超时

数据面端口随部署形态变化：**集群形态使用高可用端口 80，单节点形态使用 8000**。
用错了会出现连接超时而非鉴权失败。确认方式（在可访问该网段的机器上执行）：

```bash
for p in 80 8000; do
  timeout 4 bash -c "</dev/tcp/<lb_ip>/$p" 2>/dev/null && echo "port $p OPEN" || echo "port $p closed"
done
```

哪个端口放通就用哪个，并把 `geminidb_data_port` 调整为对应值后重新应用。

### 6.3 创建实例时报 `Invalid database version`

兼容接口不对外提供版本号，`datastore.version` **必须为空字符串**。
写成具体版本号会被服务端拒绝。

### 6.4 函数创建失败，提示找不到代码包

请先执行 `make package` 生成 `functions/<name>/function.zip`，
再执行 `terraform apply`。基础设施配置会读取代码包并校验内容摘要。

### 6.5 接口返回 `missing 'id' parameter in path`

说明路径参数未解析成功。请确认：

1. 请求路径形如 `/iXR`，末段非空；
2. API 定义中已为 `{id}` 配置 `PATH` 类型的请求参数（本仓库的 Terraform 已自动配置）。

函数在取不到网关映射的路径参数时会回退到从路径末段解析，两种情况都应正常返回。

### 6.6 事件未送达订阅方

写入类函数在事件投递失败时**不回滚写入、也不改变接口响应**，失败详情记录在云日志服务中。
排查步骤：

1. 在云日志服务中按函数名检索日志，查找 `failed to publish` 关键字；
2. 核对函数运行环境中的 `EVENT_CHANNEL_ID`、`HW_PROJECT_ID` 与访问密钥是否有效；
3. 核对事件通道是否为 `ENABLED` 状态，订阅是否已建立。

### 6.7 创建函数时报 `FSS.1212`

```
Lts config error, group or stream not existed
```

部分账号下函数工作流不允许把函数日志绑定到模板新建的日志组与日志流。
该报错与资源是否真的存在无关——新建的日志组同样无法绑定，而不指定日志组、
仅打开日志开关时创建正常。

把变量 `bind_function_log_group` 置为 `false` 重新应用即可：函数日志会回退到
函数工作流自动配置的日志组，仍然正常记录，只是不再统一落到带保留策略的自建日志组。

### 6.8 函数调用时报 `no operational permissions vpc`

函数一旦接入虚拟私有云，委托权限不足会依次表现为两个阶段：

1. 更新函数元数据时：`FSS.1037` + `VPC.2201 Policy doesn't allow vpc:subnets:get`
   —— 授予只读的虚拟私有云权限即可通过；
2. 实际调用函数时：`no operational permissions vpc, check the func xrole permissions`
   —— 只读权限不够，函数需要在子网内创建网卡，委托须含 **`VPC Administrator`**。

委托授权变更后约 **10 分钟**才生效，这段窗口内调用会持续返回旧结论，请勿误判为配置未生效。

### 6.9 创建网关专享实例失败

| 错误码 | 含义 | 处置 |
|---|---|---|
| `APIC.9999` | 内部错误。多数情况下是**请求缺少必填的可用区**，真因被这个错误码盖住 | 检查 `apig_available_zones` 是否已赋值 |
| `APIC.9219` | 该可用区资源不足 | 换用同区域其它可用区 |
| `APIC.9220` | 账号余额不足 | 充值后重新应用 |

### 6.10 释放资源时子网删除失败：`subnet is still used` / `VPC.0209`

函数接入虚拟私有云后，函数工作流会在子网内**为每个函数**创建一个名为
`FunctionGraph-Access-VPC-Port` 的接入网卡（端口属性 `device_owner` 形如
`compute:<可用区>`，但**不出现在弹性云服务器实例列表中**）。**删除函数不会同步回收这些网卡**，于是：

- 删除子网报 `VPC.0209 subnet is still used, such as computer,LB.`；
- 若跳过子网直接删虚拟私有云，则报 `VPC.0104 Router contains subnets, please delete subnet first.`；
- 强行删除这些网卡报 `VPC.2500 Port's device_owner is not empty`——属平台托管，租户无删除权限；
- 在 `terraform destroy` 里表现为子网长时间停在 `Still destroying...`，
  10 分钟后以 `Error: error deleting Subnet: timeout while waiting for state to become 'DELETED'` 结束。
  此时**计费资源均已释放完毕**，仅剩这个免费的子网与虚拟私有云。

> 实测记录（4 个函数的部署）：子网内残留 4 张 `FunctionGraph-Access-VPC-Port`
> （`device_owner` 为 `compute:cn-north-4a` / `compute:cn-north-4g`），函数删除 40 分钟后仍未回收；
> **连续两次 `terraform destroy` 均在子网处 10 分钟超时**，说明这是确定性阻塞而非偶发时序问题。

**排查方法**（两个易踩的坑）：

1. 虚拟私有云服务有**两套端口查询接口**，结果不同：普通接口
   `GET /v1/{project_id}/ports`（SDK 中的 `ListPorts`）会**过滤掉系统端口**，
   直接查会得到「0 个端口」并据此误判；必须用 **Neutron 视角**
   `GET /v2.0/ports?network_id=<子网ID>`（SDK 中的 `NeutronListPorts`）
   或控制台「虚拟私有云 → 端口」页面，才能看到这些接入网卡；
2. 删除子网时**必须同时传 `subnet_id` 和 `vpc_id`**，只传子网 ID 会报一个
   与真实原因无关的 `VPC.0201 VPC ID is invalid`。

**处置**：等待平台异步回收后重跑 `terraform destroy` 即可（子网与虚拟私有云均为免费资源，
不影响计费）。若长时间不回收，提工单请后台清理残留的接入网卡。

**预防**：验证结束后若需彻底回收网络资源，可先用 `terraform destroy -target` 逐层释放
（先函数、再数据存储），等函数工作流回收网卡后再释放子网，避免撞上 10 分钟超时。

## 7. 清理资源

```bash
cd infra
terraform destroy -auto-approve
```

**预期输出**：`Destroy complete! Resources: N destroyed.`

> **避免资源残留**：
> - 仅释放 Terraform 状态中追踪的资源；若手工创建过对象存储对象或弹性公网 IP，需另行释放；
> - 函数本地编译产物（`functions/*/handler`、`functions/*/function.zip`）可用 `make clean` 删除。
