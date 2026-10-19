package main

import (
	"os"

	"huaweicloud.com/go-runtime/pkg/runtime"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/domain"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/handlers"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/store"
)

// get-product 函数：GET /{id}，返回单个商品。
// 只读入口不挂载事件发布能力。
func main() {
	tableName, ok := os.LookupEnv("TABLE")
	if !ok {
		panic("Need TABLE environment variable")
	}

	geminiDB := store.NewGeminiDBStore(tableName)
	products := domain.NewProductsDomain(geminiDB)
	handler := handlers.NewAPIGHandler(products)

	runtime.Register(handlers.RuntimeHandler(handler.GetHandler))
}
