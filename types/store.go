package types

//go:generate mockgen -destination=./mocks/mock_store.go -package=mocks github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types Store

import (
	"context"
)

// Store 是六边形架构中的存储出站端口（driven port）。
// 领域层只依赖本接口，具体实现在 store 包（兼容接口存储 / 内存存储），
// 因此更换底层存储不影响领域逻辑。
type Store interface {
	All(context.Context, *string) (ProductRange, error)
	Get(context.Context, string) (*Product, error)
	Put(context.Context, Product) error
	Delete(context.Context, string) error
}
