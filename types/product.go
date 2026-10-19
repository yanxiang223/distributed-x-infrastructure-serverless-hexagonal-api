package types

// Product 是商品聚合的对外数据结构。
// 结构体标签同时服务两个边界：`json` 用于 HTTP 接口的请求 / 响应编解码，
// `dynamodbav` 用于兼容接口存储层的属性编解码（GeminiDB 兼容接口按同一线协议交换属性值）。
type Product struct {
	Id    string  `dynamodbav:"id" json:"id"`
	Name  string  `dynamodbav:"name" json:"name"`
	Price float64 `dynamodbav:"price" json:"price"`
}

// ProductRange 是一页商品查询结果。
// Next 为下一页的起始主键；为空表示已到末页。
type ProductRange struct {
	Products []Product `json:"products"`
	Next     *string   `json:"next,omitempty"`
}
