package main

import (
	"os"

	"huaweicloud.com/go-runtime/pkg/runtime"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/bus"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/domain"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/handlers"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/store"
)

// eventSource 是领域事件的事件来源标识，同一来源内事件标识唯一。
const eventSource = "products-api"

// delete-product 函数：DELETE /{id}，删除商品并在成功后发布 ProductDeleted 事件。
func main() {
	tableName, ok := os.LookupEnv("TABLE")
	if !ok {
		panic("Need TABLE environment variable")
	}

	channelID, ok := os.LookupEnv("EVENT_CHANNEL_ID")
	if !ok {
		panic("Need EVENT_CHANNEL_ID environment variable")
	}

	geminiDB := store.NewGeminiDBStore(tableName)
	eventGrid := bus.NewEventGridBus(channelID)
	products := domain.NewProductsDomain(geminiDB).
		WithEvents(domain.NewProductsEvents(eventGrid, eventSource))
	handler := handlers.NewAPIGHandler(products)

	runtime.Register(handlers.RuntimeHandler(handler.DeleteHandler))
}
