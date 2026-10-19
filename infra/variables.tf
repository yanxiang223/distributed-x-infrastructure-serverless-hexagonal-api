variable "region" {
  description = "资源部署区域"
  type        = string
  default     = "cn-north-4"
}

variable "project_name" {
  description = "项目名称，作为所有资源的命名前缀"
  type        = string
  default     = "products-api"
}

variable "access_key" {
  description = "访问密钥 ID。留空时由 Provider 从 HW_ACCESS_KEY 环境变量读取；函数运行环境同样需要该凭据。"
  type        = string
  default     = null
  sensitive   = true
}

variable "secret_key" {
  description = "访问密钥。留空时由 Provider 从 HW_SECRET_KEY 环境变量读取；函数运行环境同样需要该凭据。"
  type        = string
  default     = null
  sensitive   = true
}

variable "security_token" {
  description = "临时凭据的会话令牌，使用永久访问密钥时无需设置"
  type        = string
  default     = null
  sensitive   = true
}

variable "project_id" {
  description = "项目 ID，函数调用事件通道时需要。可在华为云『我的凭证』页面查询。"
  type        = string
  default     = ""
}

# ----------------------------------------------------------------------------
# 网络
# ----------------------------------------------------------------------------

variable "vpc_cidr" {
  description = "虚拟私有云网段"
  type        = string
  default     = "10.0.0.0/16"
}

variable "subnet_cidr" {
  description = "子网网段"
  type        = string
  default     = "10.0.1.0/24"
}

variable "availability_zones" {
  description = "可用区后缀列表，与区域拼接成完整可用区标识。集群形态要求可用区数量与节点数一致"
  type        = list(string)
  default     = ["a", "b", "c"]
}

# ----------------------------------------------------------------------------
# 存储
# ----------------------------------------------------------------------------

variable "geminidb_password" {
  description = "兼容接口实例的管理员密码，同时作为数据面签名密钥。无默认值，须在部署时提供"
  type        = string
  sensitive   = true
}

variable "geminidb_spec_code" {
  description = "兼容接口实例规格码，请以目标区域实际可售规格为准"
  type        = string
  default     = "geminidb.cassandra.large.4"
}

variable "geminidb_node_num" {
  description = "集群节点数。兼容接口实例为集群形态，节点数至少为 3，且须与可用区数量一致"
  type        = number
  default     = 3
}

variable "geminidb_storage_size" {
  description = "单节点存储容量（GB，取 10 的整数倍）"
  type        = number
  default     = 100
}

variable "geminidb_data_port" {
  description = "兼容接口数据面端口。集群形态使用高可用端口 80，单节点形态使用 8000。"
  type        = number
  default     = 80
}

# ----------------------------------------------------------------------------
# 计算与接入
# ----------------------------------------------------------------------------

variable "obs_bucket_name" {
  description = "存放函数代码包的对象存储桶名，需全局唯一"
  type        = string
  default     = ""
}

variable "apig_edition" {
  description = "API 网关专享版规格"
  type        = string
  default     = "BASIC"
}

variable "apig_available_zones" {
  description = <<-EOT
    网关专享实例所在可用区列表。须与目标区域实际可售资源匹配：
    可用区资源不足时创建请求会返回 APIC.9219，可改为该区域其它可用区。
  EOT
  type        = list(string)
  default     = ["cn-north-4a"]
}

variable "apig_bandwidth_size" {
  description = "API 网关公网入口带宽（Mbit/s）"
  type        = number
  default     = 5
}

variable "apig_bandwidth_charging_mode" {
  description = "API 网关公网入口带宽分配方式，取值范围 bandwidth（按带宽）/ traffic（按流量）"
  type        = string
  default     = "traffic"
}

variable "function_memory_size" {
  description = "函数内存规格（MB）"
  type        = number
  default     = 256
}

variable "function_timeout_read" {
  description = "查询类函数执行超时时间（秒）"
  type        = number
  default     = 10
}

variable "function_timeout_write" {
  description = "写入类函数执行超时时间（秒）。写入类函数在数据落库后还需同步发布变更事件，故取值高于查询类"
  type        = number
  default     = 20
}

variable "log_retention_days" {
  description = "函数日志在云日志服务的保留天数"
  type        = number
  default     = 7
}

variable "bind_function_log_group" {
  description = <<-EOT
    是否把函数日志绑定到本模板创建的日志组与日志流。
    置为 true 时，函数调用日志统一落到带保留策略的自建日志组；
    若目标账号下函数工作流拒绝绑定自定义日志组（创建函数返回 FSS.1212），
    可置为 false 回退到函数工作流自动配置的日志组，函数仍正常记录日志。
  EOT
  type        = bool
  default     = true
}

locals {
  # 可用区拼成兼容接口实例要求的可用区列表（节点数与可用区数量须匹配）
  availability_zone = join(",", [for az in var.availability_zones : "${var.region}${az}"])

  common_tags = {
    project = var.project_name
  }

  # 桶名未显式指定时按区域与项目名拼装
  obs_bucket_name = var.obs_bucket_name != "" ? var.obs_bucket_name : "${var.project_name}-${var.region}-code"

  # 函数代码包路径，由 `make package` 生成
  function_package_dir = "${path.module}/../functions"

  # 需要发布的函数清单
  function_names = ["get-products", "get-product", "put-product", "delete-product"]
}
