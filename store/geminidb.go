package store

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
)

// geminiDBUserName 是兼容接口实例自带的数据库账号名，建实例时固定生成。
// 兼容接口的数据面只接受该账号 + 实例管理员密码，不接受 IAM 凭据与委托临时凭据，
// 因此这里用静态凭据构造客户端；区域字段仅参与请求签名，不参与路由。
const geminiDBUserName = "rwuser"

// GeminiDBStore 是基于 GeminiDB 兼容接口的存储适配器（六边形架构的存储出站端口实现）。
// 兼容接口对外提供与既有线协议一致的接口，因此客户端复用该协议的 SDK，
// 只需把 endpoint 指向实例负载均衡地址、改用实例数据库账号鉴权即可。
type GeminiDBStore struct {
	client    *dynamodb.Client
	tableName string
}

var _ types.Store = (*GeminiDBStore)(nil)

// NewGeminiDBStore 从运行环境读取连接信息并构造存储客户端：
//
//	GEMINIDB_ENDPOINT  实例负载均衡地址，形如 http://<lb-ip>:80
//	GEMINIDB_REGION    参与签名的区域标识（不参与路由）
//	GEMINIDB_PASSWORD  实例管理员密码
//
// 表需要预先创建：兼容接口不会在首次写入时自动建表，建表时须指定预置读写容量
// （按需计费模式不被接受），主键为字符串类型的 id 字段。
// 集群形态的兼容接口实例使用高可用端口，须以实际放通的端口为准。
func NewGeminiDBStore(tableName string) *GeminiDBStore {
	endpoint := os.Getenv("GEMINIDB_ENDPOINT")
	if endpoint == "" {
		panic("Need GEMINIDB_ENDPOINT environment variable")
	}

	region := os.Getenv("GEMINIDB_REGION")
	if region == "" {
		panic("Need GEMINIDB_REGION environment variable")
	}

	password := os.Getenv("GEMINIDB_PASSWORD")
	if password == "" {
		panic("Need GEMINIDB_PASSWORD environment variable")
	}

	cfg := aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(geminiDBUserName, password, ""),
	}

	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &GeminiDBStore{
		client:    client,
		tableName: tableName,
	}
}

// All 按主键顺序返回一页商品。每页最多 20 条，通过返回的 Next 游标翻页。
func (d *GeminiDBStore) All(ctx context.Context, next *string) (types.ProductRange, error) {
	productRange := types.ProductRange{
		Products: []types.Product{},
	}

	input := &dynamodb.ScanInput{
		TableName: &d.tableName,
		Limit:     aws.Int32(20),
	}

	if next != nil {
		input.ExclusiveStartKey = map[string]ddbtypes.AttributeValue{
			"id": &ddbtypes.AttributeValueMemberS{Value: *next},
		}
	}

	result, err := d.client.Scan(ctx, input)

	if err != nil {
		return productRange, fmt.Errorf("failed to scan items from the store: %w", err)
	}

	err = attributevalue.UnmarshalListOfMaps(result.Items, &productRange.Products)
	if err != nil {
		return productRange, fmt.Errorf("failed to unmarshal scanned items: %w", err)
	}

	if len(result.LastEvaluatedKey) > 0 {
		if key, ok := result.LastEvaluatedKey["id"]; ok {
			nextKey := key.(*ddbtypes.AttributeValueMemberS).Value
			productRange.Next = &nextKey
		}
	}

	return productRange, nil
}

// Get 按主键读取单个商品；不存在时返回 (nil, nil)。
func (d *GeminiDBStore) Get(ctx context.Context, id string) (*types.Product, error) {
	response, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &d.tableName,
		Key: map[string]ddbtypes.AttributeValue{
			"id": &ddbtypes.AttributeValueMemberS{Value: id},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get item from the store: %w", err)
	}

	if len(response.Item) == 0 {
		return nil, nil
	}

	product := types.Product{}
	err = attributevalue.UnmarshalMap(response.Item, &product)

	if err != nil {
		return nil, fmt.Errorf("error getting item %w", err)
	}

	return &product, nil
}

// Put 按主键写入（或覆盖）单个商品。
func (d *GeminiDBStore) Put(ctx context.Context, product types.Product) error {
	item, err := attributevalue.MarshalMap(&product)
	if err != nil {
		return fmt.Errorf("unable to marshal product: %w", err)
	}

	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &d.tableName,
		Item:      item,
	})

	if err != nil {
		return fmt.Errorf("cannot put item: %w", err)
	}

	return nil
}

// Delete 按主键删除单个商品；主键不存在时按幂等语义正常返回。
func (d *GeminiDBStore) Delete(ctx context.Context, id string) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &d.tableName,
		Key: map[string]ddbtypes.AttributeValue{
			"id": &ddbtypes.AttributeValueMemberS{Value: id},
		},
	})

	if err != nil {
		return fmt.Errorf("can't delete item: %w", err)
	}

	return nil
}
