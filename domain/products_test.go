//go:build unit
// +build unit

package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/store"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types/mocks"
)

func TestGetProductNotFound(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	domain := NewProductsDomain(memoryStore)

	product, err := domain.GetProduct(context.Background(), "1")
	if err != nil {
		t.Errorf("GetProduct returned an error: %v", err)
	}

	if product != nil {
		t.Error("GetProduct returned unexpected product")
	}
}

func TestGetExistingProduct(t *testing.T) {
	ctx := context.Background()

	memoryStore := store.NewMemoryStore()
	memoryStore.Put(ctx, types.Product{
		Id:    "iXR",
		Name:  "iPhone XML",
		Price: 0.123,
	})

	domain := NewProductsDomain(memoryStore)

	product, err := domain.GetProduct(context.Background(), "iXR")
	if err != nil {
		t.Errorf("GetProduct returned an error: %v", err)
	}

	if product == nil {
		t.Errorf("GetProduct returned nil object")
		return
	}

	if product.Name != "iPhone XML" {
		t.Errorf("GetProduct returned wrong product name")
	}

	if product.Price != 0.123 {
		t.Errorf("GetProduct returned wrong price")
	}
}

func TestGetInternalStoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	store := mocks.NewMockStore(ctrl)
	store.EXPECT().
		Get(ctx, gomock.Eq("1")).
		Return(nil, errors.New("internal error"))

	domain := NewProductsDomain(store)

	product, err := domain.GetProduct(ctx, "1")
	if product != nil {
		t.Error("Got unexpected product")
	}

	if err == nil {
		t.Error("Expecting an error to be returned")
		return
	}

	if err.Error() != "internal error" {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestAllProductsWithInvalidNext(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	store := mocks.NewMockStore(ctrl)
	store.EXPECT().
		All(ctx, gomock.Nil()).
		AnyTimes()

	domain := NewProductsDomain(store)

	t.Parallel()

	t.Run("with nil 'next'", func(t *testing.T) {
		domain.AllProducts(ctx, nil)
	})

	t.Run("with empty 'next'", func(t *testing.T) {
		next := ""
		domain.AllProducts(ctx, &next)
	})

	t.Run("with empty spaces 'next'", func(t *testing.T) {
		next := "  "
		domain.AllProducts(ctx, &next)
	})
}

func TestAllProductsInternalStoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()

	store := mocks.NewMockStore(ctrl)
	store.EXPECT().
		All(ctx, gomock.All()).
		Return(types.ProductRange{}, errors.New("internal error"))

	domain := NewProductsDomain(store)

	_, err := domain.AllProducts(ctx, nil)
	if err == nil {
		t.Error("Expecting an error to be returned")
		return
	}

	if err.Error() != "internal error" {
		t.Errorf("Got unexpected error: %s", err)
	}
}

func TestAllProducts(t *testing.T) {
	memoryStore := store.NewMemoryStore()
	domain := NewProductsDomain(memoryStore)
	ctx := context.Background()

	t.Run("with an empty store", func(t *testing.T) {
		productRange, err := domain.AllProducts(ctx, nil)
		if err != nil {
			t.Errorf("Got unexpected error: %v", err)
		}

		if len(productRange.Products) != 0 {
			t.Errorf("Got unexpected products")
		}
	})

	t.Run("with products on the store", func(t *testing.T) {
		memoryStore.Put(ctx, types.Product{
			Id:    "iXR",
			Name:  "iPhone XML",
			Price: 0.123,
		})

		productRange, err := domain.AllProducts(ctx, nil)
		if err != nil {
			t.Errorf("Got unexpected error: %v", err)
		}

		if len(productRange.Products) != 1 {
			t.Errorf("Got unexpected products")
		}
	})
}

func TestPutProductInvalidBody(t *testing.T) {
	domain := NewProductsDomain(store.NewMemoryStore())

	_, err := domain.PutProduct(context.Background(), "iXR", []byte("invalid body"))
	if !errors.Is(err, ErrJsonUnmarshal) {
		t.Errorf("Expecting ErrJsonUnmarshal, got: %v", err)
	}
}

func TestPutProductIdMismatch(t *testing.T) {
	domain := NewProductsDomain(store.NewMemoryStore())

	body := []byte(`{"id":"iXR","name":"iPhone XML","price":0.123}`)
	_, err := domain.PutProduct(context.Background(), "another-id", body)
	if !errors.Is(err, ErrProductIdMismatch) {
		t.Errorf("Expecting ErrProductIdMismatch, got: %v", err)
	}
}

// captureBus 记录领域服务实际投递的事件，供断言使用。
type captureBus struct {
	events []types.Event
	err    error
}

func (c *captureBus) Put(_ context.Context, events []types.Event) ([]types.FailedEvent, error) {
	if c.err != nil {
		return nil, c.err
	}

	c.events = append(c.events, events...)
	return nil, nil
}

func newDomainWithCapture(store types.Store) (*Products, *captureBus) {
	bus := &captureBus{}
	return NewProductsDomain(store).WithEvents(NewProductsEvents(bus, "products-api")), bus
}

func TestPutProductPublishesCreatedEventForNewProduct(t *testing.T) {
	ctx := context.Background()
	domain, bus := newDomainWithCapture(store.NewMemoryStore())

	body := []byte(`{"id":"iXR","name":"iPhone XML","price":0.123}`)
	if _, err := domain.PutProduct(ctx, "iXR", body); err != nil {
		t.Fatalf("PutProduct returned an error: %v", err)
	}

	if len(bus.events) != 1 {
		t.Fatalf("Expecting 1 published event, got %d", len(bus.events))
	}

	event := bus.events[0]
	if event.DetailType != EventProductCreated {
		t.Errorf("Expecting %s, got %s", EventProductCreated, event.DetailType)
	}

	if len(event.Resources) != 1 || event.Resources[0] != "iXR" {
		t.Errorf("Got unexpected resources: %v", event.Resources)
	}

	if event.Source != "products-api" {
		t.Errorf("Got unexpected source: %s", event.Source)
	}
}

func TestPutProductPublishesUpdatedEventForExistingProduct(t *testing.T) {
	ctx := context.Background()
	memoryStore := store.NewMemoryStore()
	memoryStore.Put(ctx, types.Product{Id: "iXR", Name: "old name", Price: 1})
	domain, bus := newDomainWithCapture(memoryStore)

	body := []byte(`{"id":"iXR","name":"iPhone XML","price":0.123}`)
	if _, err := domain.PutProduct(ctx, "iXR", body); err != nil {
		t.Fatalf("PutProduct returned an error: %v", err)
	}

	if len(bus.events) != 1 {
		t.Fatalf("Expecting 1 published event, got %d", len(bus.events))
	}

	if bus.events[0].DetailType != EventProductUpdated {
		t.Errorf("Expecting %s, got %s", EventProductUpdated, bus.events[0].DetailType)
	}
}

func TestDeleteProductPublishesDeletedEvent(t *testing.T) {
	ctx := context.Background()
	memoryStore := store.NewMemoryStore()
	memoryStore.Put(ctx, types.Product{Id: "iXR", Name: "iPhone XML", Price: 0.123})
	domain, bus := newDomainWithCapture(memoryStore)

	if err := domain.DeleteProduct(ctx, "iXR"); err != nil {
		t.Fatalf("DeleteProduct returned an error: %v", err)
	}

	if len(bus.events) != 1 {
		t.Fatalf("Expecting 1 published event, got %d", len(bus.events))
	}

	if bus.events[0].DetailType != EventProductDeleted {
		t.Errorf("Expecting %s, got %s", EventProductDeleted, bus.events[0].DetailType)
	}

	if bus.events[0].Detail != `{"id":"iXR","name":"iPhone XML","price":0.123}` {
		t.Errorf("Got unexpected event detail: %s", bus.events[0].Detail)
	}
}

func TestDeleteMissingProductPublishesNoEvent(t *testing.T) {
	ctx := context.Background()
	domain, bus := newDomainWithCapture(store.NewMemoryStore())

	if err := domain.DeleteProduct(ctx, "missing"); err != nil {
		t.Fatalf("DeleteProduct returned an error: %v", err)
	}

	if len(bus.events) != 0 {
		t.Errorf("Expecting no published event, got %d", len(bus.events))
	}
}

func TestReadOnlyDomainPublishesNoEvent(t *testing.T) {
	ctx := context.Background()
	memoryStore := store.NewMemoryStore()
	// 未调用 WithEvents：查询类入口使用的只读领域服务不应产生事件
	domain := NewProductsDomain(memoryStore)

	body := []byte(`{"id":"iXR","name":"iPhone XML","price":0.123}`)
	if _, err := domain.PutProduct(ctx, "iXR", body); err != nil {
		t.Fatalf("PutProduct returned an error: %v", err)
	}

	if err := domain.DeleteProduct(ctx, "iXR"); err != nil {
		t.Fatalf("DeleteProduct returned an error: %v", err)
	}
}

func TestWriteSucceedsWhenPublishFails(t *testing.T) {
	ctx := context.Background()
	bus := &captureBus{err: errors.New("event channel unavailable")}
	domain := NewProductsDomain(store.NewMemoryStore()).
		WithEvents(NewProductsEvents(bus, "products-api"))

	body := []byte(`{"id":"iXR","name":"iPhone XML","price":0.123}`)
	product, err := domain.PutProduct(ctx, "iXR", body)
	if err != nil {
		t.Fatalf("Write must succeed even if event publishing fails, got: %v", err)
	}

	if product == nil {
		t.Fatal("Expecting the written product to be returned")
	}

	stored, err := domain.GetProduct(ctx, "iXR")
	if err != nil {
		t.Fatalf("GetProduct returned an error: %v", err)
	}

	if stored == nil {
		t.Error("Expecting the product to be persisted")
	}
}
