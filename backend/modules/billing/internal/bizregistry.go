package billing

import "sync"

// 业务类型注册表：让外部模块（如 library）把自有业务并入 billing 统一订单，
// billing 仅持有函数指针，无对外部模块的编译期依赖（依赖反转，遵守 Modulith 边界）。

// BizQuoteResult 已注册业务类型的报价结果。
//   - TotalCents：服务端权威总价（分）。
//   - MetaJSON：不透明业务载荷，原样存入订单项 meta_json，履约时回传。
type BizQuoteResult struct {
	TotalCents int64
	MetaJSON   []byte
}

// BizQuoteFunc 报价：按 userID + 不透明 params 算权威价并产出 meta_json。
type BizQuoteFunc func(userID int, params []byte) (BizQuoteResult, error)

// BizFulfillFunc 履约：支付成功后按 meta_json 落地业务变更。
type BizFulfillFunc func(userID int, orderID uint, metaJSON []byte) error

type registeredBiz struct {
	quote   BizQuoteFunc
	fulfill BizFulfillFunc
}

var (
	bizRegistryMu sync.RWMutex
	bizRegistry   = map[string]registeredBiz{}
)

// RegisterBizType 供公开门面（modules/billing/billing.go）转调的导出入口。
func RegisterBizType(bizType string, quote BizQuoteFunc, fulfill BizFulfillFunc) {
	registerBizType(bizType, quote, fulfill)
}

// registerBizType 注册一个外部业务类型（装配期调用，幂等覆盖）。
func registerBizType(bizType string, quote BizQuoteFunc, fulfill BizFulfillFunc) {
	bizRegistryMu.Lock()
	defer bizRegistryMu.Unlock()
	bizRegistry[bizType] = registeredBiz{quote: quote, fulfill: fulfill}
}

// lookupBizType 取已注册业务类型的处理器。
func lookupBizType(bizType string) (registeredBiz, bool) {
	bizRegistryMu.RLock()
	defer bizRegistryMu.RUnlock()
	rb, ok := bizRegistry[bizType]
	return rb, ok
}

// isRegisteredBiz 是否为已注册业务类型。
func isRegisteredBiz(bizType string) bool {
	bizRegistryMu.RLock()
	defer bizRegistryMu.RUnlock()
	_, ok := bizRegistry[bizType]
	return ok
}

// isValidBizType 合法业务类型 = 内置集合 ∪ 已注册集合。
func isValidBizType(bizType string) bool {
	return validBizTypes[bizType] || isRegisteredBiz(bizType)
}
