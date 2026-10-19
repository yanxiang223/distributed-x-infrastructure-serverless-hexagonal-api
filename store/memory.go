package store

import (
	"context"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
)

// MemoryStore 是纯内存实现的存储端口适配器，用于单元测试与本地调试。
// 它不依赖任何云服务，保证领域层可以脱离云环境被验证。
type MemoryStore struct {
	storage map[string]types.Product
}

// Just to make sure MemoryStore implements the Store interface
var _ types.Store = (*MemoryStore)(nil)

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		storage: make(map[string]types.Product),
	}
}

func (m *MemoryStore) All(ctx context.Context, next *string) (types.ProductRange, error) {
	productRange := types.ProductRange{
		Products: []types.Product{},
		Next:     stringPtr("random next string"),
	}

	for _, v := range m.storage {
		productRange.Products = append(productRange.Products, v)
	}

	return productRange, nil
}

func (m *MemoryStore) Get(ctx context.Context, id string) (*types.Product, error) {
	p, ok := m.storage[id]
	if !ok {
		return nil, nil
	}

	return &p, nil
}

func (m *MemoryStore) Put(ctx context.Context, p types.Product) error {
	m.storage[p.Id] = p

	return nil
}

func (m *MemoryStore) Delete(ctx context.Context, id string) error {
	delete(m.storage, id)

	return nil
}

func stringPtr(s string) *string {
	return &s
}
