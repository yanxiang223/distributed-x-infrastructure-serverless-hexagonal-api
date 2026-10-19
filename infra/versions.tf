terraform {
  required_version = ">= 1.5"

  required_providers {
    huaweicloud = {
      source  = "huaweicloud/huaweicloud"
      version = "~> 1.70"
    }
  }

  # 状态文件默认保存在本地。生产环境建议改为对象存储后端：
  # backend "s3" {
  #   endpoint = "https://obs.cn-north-4.myhuaweicloud.com"
  #   bucket   = "your-tfstate-bucket"
  #   key      = "products-api/terraform.tfstate"
  #   region   = "cn-north-4"
  # }
  backend "local" {}
}

provider "huaweicloud" {
  region = var.region

  # 访问密钥留空时由 Provider 从 HW_ACCESS_KEY / HW_SECRET_KEY 环境变量读取。
  # 也可通过 TF_VAR_access_key / TF_VAR_secret_key 传入，避免写入任何文件。
  access_key     = var.access_key
  secret_key     = var.secret_key
  security_token = var.security_token
}
