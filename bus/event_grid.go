package bus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/region"
	eg "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eg/v1"
	egmodel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/eg/v1/model"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
)

const (
	// 事件通道单次投递支持的最大事件条数，超出部分自动分批。
	maxBatchSize = 10

	// CloudEvents 规范版本，事件通道按该版本校验事件结构。
	cloudEventsSpecVersion = "1.0"

	// 事件载荷的内容类型，采用 MIME 格式。
	cloudEventsContentType = "application/json"

	// 事件通道服务端点模板，按区域拼装；可用 EG_ENDPOINT 覆盖。
	defaultEndpointTemplate = "https://eg.%s.myhuaweicloud.com"
)

// EventGridBus 是事件网格自定义事件通道的适配器（六边形架构的事件出站端口实现）。
// 它把领域事件映射为 CloudEvents 结构后投递到指定事件通道，
// 订阅方按事件类型过滤后路由到下游处理函数。
type EventGridBus struct {
	client    *eg.EgClient
	channelID string
}

var _ types.Bus = (*EventGridBus)(nil)

// NewEventGridBus 从运行环境读取鉴权信息并构造事件通道客户端：
//
//	HW_REGION_NAME     区域标识，如 cn-north-4
//	HW_PROJECT_ID      项目 ID
//	HW_ACCESS_KEY      访问密钥 ID
//	HW_SECRET_KEY      访问密钥
//	HW_SECURITY_TOKEN  临时凭据的会话令牌（使用永久访问密钥时无需设置）
//	EG_ENDPOINT        服务端点，可选；默认按区域拼装
//
// 凭据由部署侧注入到函数运行环境，不写入代码与配置文件。
func NewEventGridBus(channelID string) *EventGridBus {
	if channelID == "" {
		panic("Need EVENT_CHANNEL_ID environment variable")
	}

	regionName := mustEnv("HW_REGION_NAME")

	credentialsBuilder := auth.NewBasicCredentialsBuilder().
		WithAk(mustEnv("HW_ACCESS_KEY")).
		WithSk(mustEnv("HW_SECRET_KEY")).
		WithProjectId(mustEnv("HW_PROJECT_ID"))

	if sessionToken := os.Getenv("HW_SECURITY_TOKEN"); sessionToken != "" {
		credentialsBuilder = credentialsBuilder.WithSecurityToken(sessionToken)
	}

	credentials, err := credentialsBuilder.SafeBuild()
	if err != nil {
		panic(fmt.Sprintf("cannot build credentials: %v", err))
	}

	endpoint := os.Getenv("EG_ENDPOINT")
	if endpoint == "" {
		endpoint = fmt.Sprintf(defaultEndpointTemplate, regionName)
	}

	client := eg.NewEgClient(
		eg.EgClientBuilder().
			WithCredential(credentials).
			WithRegion(region.NewRegion(regionName, endpoint)).
			Build(),
	)

	return &EventGridBus{
		client:    client,
		channelID: channelID,
	}
}

// Put 把领域事件分批投递到事件通道，返回投递失败的事件明细。
// 单条投递失败不会中断同批其余事件的投递。
func (e *EventGridBus) Put(ctx context.Context, events []types.Event) ([]types.FailedEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return batchEvents(events, maxBatchSize, func(batchEvents []types.Event) ([]types.FailedEvent, error) {
		cloudEvents := make([]egmodel.CloudEvents, 0, len(batchEvents))

		for _, event := range batchEvents {
			cloudEvents = append(cloudEvents, toCloudEvent(event))
		}

		result, err := e.client.PutEvents(&egmodel.PutEventsRequest{
			ChannelId: e.channelID,
			Body: &egmodel.PutEventsReq{
				Events: &cloudEvents,
			},
		})

		failedEvents := []types.FailedEvent{}
		if err != nil {
			return failedEvents, fmt.Errorf("failed to publish events to the event channel: %w", err)
		}

		if result.Events == nil {
			return failedEvents, nil
		}

		for i, entry := range *result.Events {
			if entry.ErrorCode == nil {
				continue
			}

			failedEvents = append(failedEvents, types.FailedEvent{
				Event:          batchEvents[i],
				FailureCode:    derefString(entry.ErrorCode),
				FailureMessage: derefString(entry.ErrorMsg),
			})
		}

		return failedEvents, nil
	})
}

// toCloudEvent 把领域事件映射为 CloudEvents 1.0 结构。
// 业务载荷解析为 JSON 对象写入 data 字段，避免投递后变成被转义过的字符串。
func toCloudEvent(event types.Event) egmodel.CloudEvents {
	var decoded interface{}
	if err := json.Unmarshal([]byte(event.Detail), &decoded); err != nil {
		// 非 JSON 载荷按原文投递
		decoded = event.Detail
	}

	var subject *string
	if len(event.Resources) > 0 && event.Resources[0] != "" {
		subject = &event.Resources[0]
	}

	eventTime := time.Now().UTC().Format(time.RFC3339)
	contentType := cloudEventsContentType

	eventID := newEventID(event.Resources)

	return egmodel.CloudEvents{
		Id:              eventID,
		Source:          event.Source,
		Specversion:     cloudEventsSpecVersion,
		Type:            event.DetailType,
		Datacontenttype: &contentType,
		Data:            &decoded,
		Time:            &eventTime,
		Subject:         subject,
	}
}

// newEventID 生成事件唯一标识。事件通道要求同一事件源内 id 唯一，
// 因此以业务对象标识为前缀拼接随机值：既满足唯一性约束，也便于按对象检索事件。
func newEventID(resources []string) string {
	prefix := "product"
	if len(resources) > 0 && resources[0] != "" {
		prefix = resources[0]
	}

	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}

	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(buffer))
}

// batchEvents 把事件按最大批量切分后逐批投递，并汇总各批的失败明细。
func batchEvents(events []types.Event, maxBatchSize uint, batchFn func([]types.Event) ([]types.FailedEvent, error)) ([]types.FailedEvent, error) {
	skip := 0
	recordsAmount := len(events)
	batchAmount := int(math.Ceil(float64(recordsAmount) / float64(maxBatchSize)))

	batchFailedEvents := []types.FailedEvent{}

	for i := 0; i < batchAmount; i++ {
		lowerBound := skip
		upperBound := skip + int(maxBatchSize)

		if upperBound > recordsAmount {
			upperBound = recordsAmount
		}

		batchEvents := events[lowerBound:upperBound]
		skip += int(maxBatchSize)

		failedEvents, err := batchFn(batchEvents)
		if err != nil {
			return batchFailedEvents, err
		}

		if len(failedEvents) > 0 {
			batchFailedEvents = append(batchFailedEvents, failedEvents...)
		}
	}

	return batchFailedEvents, nil
}

func mustEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("Need %s environment variable", name))
	}

	return value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
