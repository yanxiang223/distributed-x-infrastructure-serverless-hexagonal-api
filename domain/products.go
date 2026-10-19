package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
)

var (
	ErrJsonUnmarshal     = errors.New("failed to parse product from request body")
	ErrProductIdMismatch = errors.New("product ID in path does not match product ID in body")
)

// Products 是商品领域的核心服务（六边形架构的领域层）。
// 它只依赖 types.Store / types.Bus 两个出站端口，不感知任何云服务细节。
type Products struct {
	store  types.Store
	events *ProductsEvents
}

// NewProductsDomain 构造领域服务。默认只具备读写能力，不发布领域事件。
func NewProductsDomain(s types.Store) *Products {
	return &Products{
		store: s,
	}
}

// WithEvents 为领域服务挂载领域事件发布能力。
// 商品变更必须产生领域事件是本领域的业务不变量，因此该能力挂在领域服务上而非入口层；
// 只读入口（查询类函数）不挂载，写操作即不产生事件。
func (d *Products) WithEvents(e *ProductsEvents) *Products {
	d.events = e
	return d
}

func (d *Products) GetProduct(ctx context.Context, id string) (*types.Product, error) {
	product, err := d.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return product, nil
}

func (d *Products) AllProducts(ctx context.Context, next *string) (types.ProductRange, error) {
	if next != nil && strings.TrimSpace(*next) == "" {
		next = nil
	}

	productRange, err := d.store.All(ctx, next)
	if err != nil {
		return productRange, fmt.Errorf("%w", err)
	}

	return productRange, nil
}

// PutProduct 写入商品并在成功后发布变更事件。
// 写入前先读取当前值，用于区分「新增」与「更新」两类领域事件。
func (d *Products) PutProduct(ctx context.Context, id string, body []byte) (*types.Product, error) {
	product := types.Product{}
	if err := json.Unmarshal(body, &product); err != nil {
		return nil, fmt.Errorf("%w", ErrJsonUnmarshal)
	}

	if product.Id != id {
		return nil, fmt.Errorf("%w", ErrProductIdMismatch)
	}

	existing, err := d.store.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	changeType := EventProductUpdated
	if existing == nil {
		changeType = EventProductCreated
	}

	err = d.store.Put(ctx, product)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	d.publish(ctx, changeType, product)

	return &product, nil
}

// DeleteProduct 删除商品并在成功后发布变更事件。
// 删除前先读取商品内容，使事件载荷包含完整的被删除对象；
// 商品不存在时仍然按幂等语义正常返回。
func (d *Products) DeleteProduct(ctx context.Context, id string) error {
	existing, err := d.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	err = d.store.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if existing != nil {
		d.publish(ctx, EventProductDeleted, *existing)
	}

	return nil
}

// publish 发布领域事件。
// 数据写入已经成功，事件投递失败不回滚写入、也不改变接口响应，
// 只记录日志以便通过云日志服务追溯并人工补偿。
func (d *Products) publish(ctx context.Context, eventType string, product types.Product) {
	if d.events == nil {
		return
	}

	if err := d.events.PublishProductChange(ctx, eventType, product); err != nil {
		log.Printf("failed to publish %s event for product %s: %v", eventType, product.Id, err)
	}
}
