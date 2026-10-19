package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
)

// 领域事件类型，构成对外的事件契约：订阅方按 Type 过滤并路由。
const (
	EventProductCreated = "ProductCreated"
	EventProductUpdated = "ProductUpdated"
	EventProductDeleted = "ProductDeleted"
)

// ProductsEvents 负责把领域事件投递到事件总线。
// 它同样只依赖 types.Bus 端口，底层是事件通道还是消息队列对领域层透明。
type ProductsEvents struct {
	bus    types.Bus
	source string
}

func NewProductsEvents(b types.Bus, source string) *ProductsEvents {
	return &ProductsEvents{
		bus:    b,
		source: source,
	}
}

// PublishProductChange 发布单条商品变更领域事件，事件载荷为商品对象的 JSON 表示。
func (p *ProductsEvents) PublishProductChange(ctx context.Context, eventType string, product types.Product) error {
	detail, err := json.Marshal(product)
	if err != nil {
		return fmt.Errorf("cannot marshal product %q: %w", product.Id, err)
	}

	return p.Publish(ctx, []types.Event{{
		Source:     p.source,
		Detail:     string(detail),
		DetailType: eventType,
		Resources:  []string{product.Id},
	}})
}

// Publish 批量发布事件；只要有任意一条投递失败就返回错误。
func (p *ProductsEvents) Publish(ctx context.Context, events []types.Event) error {
	failedEvents, err := p.bus.Put(ctx, events)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(failedEvents) > 0 {
		first := failedEvents[0]
		return fmt.Errorf("failed to publish %d of %d event(s); first failure: %s (%s)",
			len(failedEvents), len(events), first.FailureMessage, first.FailureCode)
	}

	return nil
}
