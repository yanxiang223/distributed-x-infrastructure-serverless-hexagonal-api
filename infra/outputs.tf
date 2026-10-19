output "api_url" {
  description = "API 调用地址（RELEASE 环境）。路径参数按 /{id} 追加，例如 <api_url>/iXR。"
  value       = "https://${huaweicloud_apig_instance.main.ingress_address}"
}

output "api_vpc_ingress_address" {
  description = "API 在虚拟私有云内的入口地址，供同网段资源调用"
  value       = huaweicloud_apig_instance.main.vpc_ingress_address
}

output "event_channel_id" {
  description = "商品变更领域事件通道 ID，订阅方按事件类型过滤后路由到下游"
  value       = huaweicloud_eg_custom_event_channel.products.id
}

output "event_channel_name" {
  description = "商品变更领域事件通道名称"
  value       = huaweicloud_eg_custom_event_channel.products.name
}

output "geminidb_endpoint" {
  description = "兼容接口实例的负载均衡地址"
  value       = huaweicloud_geminidb_instance.store.lb_ip_address
  sensitive   = true
}

output "geminidb_data_port" {
  description = "兼容接口数据面端口，与函数运行环境中的配置保持一致"
  value       = var.geminidb_data_port
}

output "table_name" {
  description = "商品表名称（须在首次写入前按部署指南步骤 7 预建，兼容接口不会自动建表）"
  value       = var.project_name
}

output "function_names" {
  description = "已部署的函数名称列表"
  value = concat(
    [for name in local.read_only_functions : huaweicloud_fgs_function.read[name].name],
    [for name in local.write_functions : huaweicloud_fgs_function.write[name].name],
  )
}

output "function_agency_name" {
  description = "函数绑定的云服务委托名称，函数以该委托身份写入日志与拉取代码包"
  value       = huaweicloud_identity_agency.function.name
}

output "log_group_name" {
  description = "函数日志所在的云日志服务日志组"
  value       = huaweicloud_lts_group.app.group_name
}
