package types

import "context"

// Event 是领域事件的传输载体，与具体事件通道实现解耦。
type Event struct {
	Source     string
	Detail     string
	DetailType string
	Resources  []string
}

// FailedEvent 是投递失败的事件及其失败原因。
type FailedEvent struct {
	Event
	FailureCode    string
	FailureMessage string
}

// Bus 是六边形架构中的事件出站端口（driven port）。
// 领域层通过它在业务写操作成功后发布领域事件，
// 具体实现在 bus 包（事件网格通道）。
type Bus interface {
	Put(context.Context, []Event) ([]FailedEvent, error)
}
