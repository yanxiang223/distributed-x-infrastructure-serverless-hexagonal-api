# ============================================================================
# 商品 API：API 网关 + 函数工作流 + 兼容接口数据库 + 事件通道
# ============================================================================
# 部署顺序：先在本仓库根目录执行 `make package` 生成函数代码包，
# 再进入本目录执行 `terraform init` / `apply`。
# ============================================================================

locals {
  # 只读入口：仅查询存储，不发布领域事件
  read_only_functions = ["get-products", "get-product"]

  # 写入入口：写入存储并在成功后发布领域事件
  write_functions = ["put-product", "delete-product"]

  # 存储与鉴权共用的运行环境变量
  shared_function_env = {
    TABLE             = var.project_name
    GEMINIDB_ENDPOINT = "http://${huaweicloud_geminidb_instance.store.lb_ip_address}:${var.geminidb_data_port}"
    GEMINIDB_REGION   = var.region
    GEMINIDB_PASSWORD = var.geminidb_password
  }

  # 事件通道调用所需的凭据，仅在显式提供时注入函数运行环境
  event_channel_env = merge(
    var.access_key == null ? {} : { HW_ACCESS_KEY = var.access_key },
    var.secret_key == null ? {} : { HW_SECRET_KEY = var.secret_key },
    var.security_token == null ? {} : { HW_SECURITY_TOKEN = var.security_token },
    {
      HW_REGION_NAME   = var.region
      HW_PROJECT_ID    = var.project_id
      EVENT_CHANNEL_ID = huaweicloud_eg_custom_event_channel.products.id
    },
  )

  read_function_env  = local.shared_function_env
  write_function_env = merge(local.shared_function_env, local.event_channel_env)

  # 网关侧调用超时不得短于后端函数自身超时，取两类函数中的较大值
  max_function_timeout = max(var.function_timeout_read, var.function_timeout_write)

  # 对外接口定义：路径、方法、路径参数与后端函数一一对应
  api_definitions = {
    "get-products" = {
      method       = "GET"
      path         = "/"
      description  = "查询商品列表"
      path_params  = []
      function_urn = huaweicloud_fgs_function.read["get-products"].urn
    }
    "get-product" = {
      method       = "GET"
      path         = "/{id}"
      description  = "查询单个商品"
      path_params  = ["id"]
      function_urn = huaweicloud_fgs_function.read["get-product"].urn
    }
    "put-product" = {
      method       = "PUT"
      path         = "/{id}"
      description  = "写入商品并在成功后发布变更事件"
      path_params  = ["id"]
      function_urn = huaweicloud_fgs_function.write["put-product"].urn
    }
    "delete-product" = {
      method       = "DELETE"
      path         = "/{id}"
      description  = "删除商品并在成功后发布变更事件"
      path_params  = ["id"]
      function_urn = huaweicloud_fgs_function.write["delete-product"].urn
    }
  }
}

# ----------------------------------------------------------------------------
# Layer 1：网络
# ----------------------------------------------------------------------------

resource "huaweicloud_vpc" "main" {
  name = "${var.project_name}-vpc"
  cidr = var.vpc_cidr

  tags = local.common_tags
}

resource "huaweicloud_vpc_subnet" "main" {
  name       = "${var.project_name}-subnet"
  vpc_id     = huaweicloud_vpc.main.id
  cidr       = var.subnet_cidr
  gateway_ip = cidrhost(var.subnet_cidr, 1)
}

resource "huaweicloud_networking_secgroup" "app" {
  name        = "${var.project_name}-sg"
  description = "Security group for ${var.project_name}"

  tags = local.common_tags
}

# ----------------------------------------------------------------------------
# Layer 2：存储（兼容接口实例）
# ----------------------------------------------------------------------------
# 兼容接口实例对外提供与既有线协议一致的接口（不会自动建表，商品表须在首次写入前预建）。
# 数据面只接受实例自带的数据库账号，不接受 IAM 凭据，故函数侧以实例账号鉴权。

resource "huaweicloud_geminidb_instance" "store" {
  name              = "${var.project_name}-store"
  mode              = "Cluster"
  password          = var.geminidb_password
  availability_zone = local.availability_zone
  vpc_id            = huaweicloud_vpc.main.id
  subnet_id         = huaweicloud_vpc_subnet.main.id
  security_group_id = huaweicloud_networking_secgroup.app.id

  datastore {
    # 兼容接口不对外提供版本号，版本字段须为空串
    type           = "dynamodb"
    storage_engine = "rocksDB"
    version        = ""
  }

  flavor {
    num       = var.geminidb_node_num
    size      = var.geminidb_storage_size
    spec_code = var.geminidb_spec_code
    storage   = "ULTRAHIGH"
  }

  tags = local.common_tags
}

# ----------------------------------------------------------------------------
# Layer 3：事件通道
# ----------------------------------------------------------------------------

resource "huaweicloud_eg_custom_event_channel" "products" {
  name        = "${var.project_name}-channel"
  description = "商品变更领域事件通道（ProductCreated / ProductUpdated / ProductDeleted）"
}

# ----------------------------------------------------------------------------
# Layer 3.5：函数委托
# ----------------------------------------------------------------------------
# 函数工作流需以委托身份访问云日志服务（写运行日志）、对象存储（拉取代码包），
# 并在函数接入虚拟私有云时代管网卡。这里显式声明委托并限定到所需授权项，
# 不使用控制台自动创建的默认委托。
# 授权不足的两个表现：更新函数元数据报 FSS.1037（读不到子网），
# 调用函数报 "no operational permissions vpc"（无法在私有网络内创建网卡）；
# 因此虚拟私有云侧需授予管理权限而非只读权限。

resource "huaweicloud_identity_agency" "function" {
  name                   = "${var.project_name}-fgs-agency"
  description            = "允许函数工作流写入运行日志、读取函数代码包并代管私有网络网卡"
  delegated_service_name = "op_svc_cff"
  all_resources_roles    = ["LTS Administrator", "OBS OperateAccess", "VPC Administrator"]
  domain_roles           = ["VPC Administrator"]
}

# ----------------------------------------------------------------------------
# Layer 4：日志
# ----------------------------------------------------------------------------

resource "huaweicloud_lts_group" "app" {
  group_name  = "${var.project_name}-logs"
  ttl_in_days = var.log_retention_days
}

resource "huaweicloud_lts_stream" "functions" {
  group_id    = huaweicloud_lts_group.app.id
  stream_name = "${var.project_name}-functions"
  ttl_in_days = var.log_retention_days
}

# ----------------------------------------------------------------------------
# Layer 5：函数代码包
# ----------------------------------------------------------------------------
# 代码包由 `make package` 在本地交叉编译后生成，
# 这里上传到对象存储并由函数按 OBS 地址拉取，避免内联代码的大小限制。

resource "huaweicloud_obs_bucket" "code" {
  bucket        = local.obs_bucket_name
  acl           = "private"
  force_destroy = true

  tags = local.common_tags
}

resource "huaweicloud_obs_bucket_object" "function_code" {
  for_each = toset(local.function_names)

  bucket       = huaweicloud_obs_bucket.code.bucket
  key          = "functions/${each.key}/function.zip"
  source       = "${local.function_package_dir}/${each.key}/function.zip"
  content_type = "application/zip"
  etag         = filemd5("${local.function_package_dir}/${each.key}/function.zip")
}

# ----------------------------------------------------------------------------
# Layer 6：函数
# ----------------------------------------------------------------------------

# 查询类函数：只读存储，不发布事件
resource "huaweicloud_fgs_function" "read" {
  for_each = toset(local.read_only_functions)

  name        = "${var.project_name}-${each.key}"
  description = "商品查询函数 ${each.key}"

  app         = "default"
  handler     = "handler"
  runtime     = "Go1.x"
  memory_size = var.function_memory_size
  timeout     = var.function_timeout_read

  vpc_id     = huaweicloud_vpc.main.id
  network_id = huaweicloud_vpc_subnet.main.id

  # agency：函数运行时以该委托身份调用其它云服务（写日志、读代码包）。
  # app_agency：函数工作流服务自身访问用户资源时使用的委托，
  # 与 agency 分属两条链路，运行时调用云服务只认前者。
  agency     = huaweicloud_identity_agency.function.name
  app_agency = huaweicloud_identity_agency.function.name

  # 日志组/流按开关绑定；部分账号下函数工作流不接受自定义日志组（FSS.1212），
  # 此时回退到平台自动配置，函数日志能力不受影响。
  log_group_id   = var.bind_function_log_group ? huaweicloud_lts_group.app.id : null
  log_stream_id  = var.bind_function_log_group ? huaweicloud_lts_stream.functions.id : null
  enable_lts_log = true

  code_type = "obs"
  code_url  = "https://${huaweicloud_obs_bucket.code.bucket}.obs.${var.region}.myhuaweicloud.com/${huaweicloud_obs_bucket_object.function_code[each.key].key}"

  user_data = jsonencode(local.read_function_env)
  tags      = local.common_tags
}

# 写入类函数：写入存储并在成功后发布领域事件
resource "huaweicloud_fgs_function" "write" {
  for_each = toset(local.write_functions)

  name        = "${var.project_name}-${each.key}"
  description = "商品写入函数 ${each.key}"

  app         = "default"
  handler     = "handler"
  runtime     = "Go1.x"
  memory_size = var.function_memory_size
  timeout     = var.function_timeout_write

  vpc_id     = huaweicloud_vpc.main.id
  network_id = huaweicloud_vpc_subnet.main.id

  # 同查询类函数，两个委托字段均需指定
  agency     = huaweicloud_identity_agency.function.name
  app_agency = huaweicloud_identity_agency.function.name

  log_group_id   = var.bind_function_log_group ? huaweicloud_lts_group.app.id : null
  log_stream_id  = var.bind_function_log_group ? huaweicloud_lts_stream.functions.id : null
  enable_lts_log = true

  code_type = "obs"
  code_url  = "https://${huaweicloud_obs_bucket.code.bucket}.obs.${var.region}.myhuaweicloud.com/${huaweicloud_obs_bucket_object.function_code[each.key].key}"

  user_data = jsonencode(local.write_function_env)
  tags      = local.common_tags
}

# ----------------------------------------------------------------------------
# Layer 7：API 接入层
# ----------------------------------------------------------------------------

# 网关的公网入口所需弹性公网 IP。带宽按流量计费，规格与网关公网入口带宽保持一致。
resource "huaweicloud_vpc_eip" "apig" {
  publicip {
    type = "5_bgp"
  }

  bandwidth {
    name        = "${var.project_name}-apig-bw"
    size        = var.apig_bandwidth_size
    share_type  = "PER"
    charge_mode = var.apig_bandwidth_charging_mode
  }

  tags = local.common_tags
}

resource "huaweicloud_apig_instance" "main" {
  name              = "${var.project_name}-apig"
  edition           = var.apig_edition
  vpc_id            = huaweicloud_vpc.main.id
  subnet_id         = huaweicloud_vpc_subnet.main.id
  security_group_id = huaweicloud_networking_secgroup.app.id

  # 网关实例需显式指定可用区，否则创建请求会被服务端拒绝
  available_zones = var.apig_available_zones

  # 公网入口复用本模板声明的弹性公网 IP：eip_id 与入口带宽参数互斥，
  # 两者不能同时声明，这里选择自带 EIP 以便显式控制带宽与计费方式。
  eip_id = huaweicloud_vpc_eip.apig.id

  maintain_begin = "02:00:00"

  tags = local.common_tags
}

resource "huaweicloud_apig_group" "main" {
  name        = "${var.project_name}-group"
  instance_id = huaweicloud_apig_instance.main.id
  description = "商品 API 分组"
}

resource "huaweicloud_apig_environment" "release" {
  name        = "RELEASE"
  instance_id = huaweicloud_apig_instance.main.id
  description = "生产环境"
}

# 四个接口分别转发到对应函数；路径参数按 API 定义映射后随事件下发给函数。
resource "huaweicloud_apig_api" "products" {
  for_each = local.api_definitions

  instance_id      = huaweicloud_apig_instance.main.id
  group_id         = huaweicloud_apig_group.main.id
  name             = each.key
  type             = "FunctionGraph"
  request_method   = each.value.method
  request_path     = each.value.path
  request_protocol = "HTTPS"
  description      = each.value.description

  # 请求体由网关以明文形式下发给函数
  is_send_fg_body_base64 = false

  func_graph {
    function_urn    = each.value.function_urn
    invocation_type = "sync"
    timeout         = local.max_function_timeout * 1000
  }

  dynamic "request_params" {
    for_each = each.value.path_params

    content {
      name     = request_params.value
      location = "PATH"
      type     = "STRING"
      required = true
    }
  }

  tags = [var.project_name]
}

resource "huaweicloud_apig_api_publishment" "products" {
  for_each = local.api_definitions

  instance_id = huaweicloud_apig_instance.main.id
  api_id      = huaweicloud_apig_api.products[each.key].id
  env_id      = huaweicloud_apig_environment.release.id
}
