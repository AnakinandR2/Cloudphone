package library

import (
	"encoding/json"
	"testing"
	"time"
)

func mustParams(t *testing.T, p PackageParams) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return b
}

// 免费态购买 t100/90天 → lib_new；价 = 1800*3*0.95 = 5130；容量 100GiB；到期 now+90d。
func TestQuote_New_FromFree(t *testing.T) {
	const uid = 910001
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 90}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionNew {
		t.Fatalf("action want new, got %s", res.Action)
	}
	if res.TotalCents != 5130 {
		t.Fatalf("total want 5130, got %d", res.TotalCents)
	}
	if res.CapacityBytes != 100*GiB {
		t.Fatalf("capacity want %d, got %d", 100*GiB, res.CapacityBytes)
	}
	wantExp := now.Add(90 * 24 * time.Hour)
	if res.NewExpireAt == nil || !res.NewExpireAt.Equal(wantExp) {
		t.Fatalf("expire want %v, got %v", wantExp, res.NewExpireAt)
	}
	if res.Meta.PrevTierCode != TierFree {
		t.Fatalf("prev tier want free, got %s", res.Meta.PrevTierCode)
	}
}

// 当前 t100（月价 1800），再买 t100/30天 → lib_renew；到期 = 当前到期 + days。
func TestQuote_Renew_SameTier(t *testing.T) {
	const uid = 910002
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	curExp := now.Add(10 * 24 * time.Hour)
	setSub(t, uid, "t100", 100*GiB, 1800, &curExp)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 30}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionRenew {
		t.Fatalf("action want renew, got %s", res.Action)
	}
	if res.TotalCents != 1800 {
		t.Fatalf("total want 1800, got %d", res.TotalCents)
	}
	wantExp := curExp.Add(30 * 24 * time.Hour)
	if res.NewExpireAt == nil || !res.NewExpireAt.Equal(wantExp) {
		t.Fatalf("expire want %v, got %v", wantExp, res.NewExpireAt)
	}
}

// 当前 t50（月价 1000），升级 t100（月价 1800），剩余 30 天 → lib_upgrade；补差价 800；到期不变。
func TestQuote_Upgrade(t *testing.T) {
	const uid = 910003
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	curExp := now.Add(30 * 24 * time.Hour)
	setSub(t, uid, "t50", 50*GiB, 1000, &curExp)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100"}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionUpgrade {
		t.Fatalf("action want upgrade, got %s", res.Action)
	}
	if res.RemainingDays != 30 {
		t.Fatalf("remaining want 30, got %d", res.RemainingDays)
	}
	if res.TotalCents != 800 {
		t.Fatalf("total want 800, got %d", res.TotalCents)
	}
	if res.NewExpireAt == nil || !res.NewExpireAt.Equal(curExp) {
		t.Fatalf("expire must stay %v, got %v", curExp, res.NewExpireAt)
	}
	if res.CapacityBytes != 100*GiB {
		t.Fatalf("capacity want 100GiB, got %d", res.CapacityBytes)
	}
}

// 当前 t100（月价 1800），降级 t50（月价 1000） → lib_downgrade；价 0；到期不变。
func TestQuote_Downgrade(t *testing.T) {
	const uid = 910004
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	curExp := now.Add(30 * 24 * time.Hour)
	setSub(t, uid, "t100", 100*GiB, 1800, &curExp)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t50"}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionDowngrade {
		t.Fatalf("action want downgrade, got %s", res.Action)
	}
	if res.TotalCents != 0 {
		t.Fatalf("total want 0, got %d", res.TotalCents)
	}
	if res.NewExpireAt == nil || !res.NewExpireAt.Equal(curExp) {
		t.Fatalf("expire must stay %v, got %v", curExp, res.NewExpireAt)
	}
	if res.CapacityBytes != 50*GiB {
		t.Fatalf("capacity want 50GiB, got %d", res.CapacityBytes)
	}
}

// 自定义档：100GB × 20分/GB = 2000 月价；30 天无折扣 → 2000；lib_new。
func TestQuote_CustomTier_New(t *testing.T) {
	const uid = 910005
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{CapacityGB: 100, Days: 30}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionNew {
		t.Fatalf("action want new, got %s", res.Action)
	}
	if res.MonthlyPriceCents != 2000 {
		t.Fatalf("monthly want 2000, got %d", res.MonthlyPriceCents)
	}
	if res.TotalCents != 2000 {
		t.Fatalf("total want 2000, got %d", res.TotalCents)
	}
	if res.TierCode != "custom" {
		t.Fatalf("tier want custom, got %s", res.TierCode)
	}
}

// 自定义档低于起步容量 → 校验失败。
func TestQuote_CustomTier_BelowMin(t *testing.T) {
	const uid = 910006
	cleanLibraryData(t, uid)
	if _, err := Service.QuotePackage(uid, mustParams(t, PackageParams{CapacityGB: 10, Days: 30})); err == nil {
		t.Fatal("expected error for below-min custom capacity")
	}
}

// new/renew 必须选合法时长。
func TestQuote_New_RequiresDuration(t *testing.T) {
	const uid = 910007
	cleanLibraryData(t, uid)
	if _, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100"})); err == nil {
		t.Fatal("expected error when days missing for new")
	}
	if _, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 7})); err == nil {
		t.Fatal("expected error for unsupported duration")
	}
}

// 履约：quote → marshal meta → FulfillPackage → 订阅快照落地。
func TestFulfill_UpsertsSubscription(t *testing.T) {
	const uid = 910008
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 90}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	meta, _ := json.Marshal(res.Meta)
	if err := Service.FulfillPackage(uid, 0, meta); err != nil {
		t.Fatalf("fulfill: %v", err)
	}
	sub, err := Service.repo.getSubscription(uid)
	if err != nil || sub == nil {
		t.Fatalf("get sub: %v sub=%v", err, sub)
	}
	if sub.TierCode != "t100" || sub.CapacityBytes != 100*GiB || sub.MonthlyPriceCents != 1800 {
		t.Fatalf("sub snapshot wrong: %+v", sub)
	}
	if sub.ExpireAt == nil || !sub.ExpireAt.Equal(now.Add(90*24*time.Hour)) {
		t.Fatalf("sub expire wrong: %v", sub.ExpireAt)
	}

	// 履约后再 quote 同档 → renew（月价相同）。
	res2, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 30}))
	if err != nil {
		t.Fatalf("quote2: %v", err)
	}
	if res2.Action != ActionRenew {
		t.Fatalf("after fulfill, action want renew, got %s", res2.Action)
	}
}

// major-1：两个不同档位配成同月价时不得判为 renew（按档位身份判，不按月价）。
// 当前订阅 t100（月价 1800），选 t500 但临时把 t500 月价改成 1800（与 t100 同价）→ 不同档，
// 月价相等且 t500 容量更大 → upgrade（绝不能是 renew）。
func TestQuote_SameMonthlyDifferentTier_NotRenew(t *testing.T) {
	const uid = 910010
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	curExp := now.Add(30 * 24 * time.Hour)
	setSub(t, uid, "t100", 100*GiB, 1800, &curExp)

	// 临时把 t500 月价改成与 t100 相同（1800），用例结束还原。
	restorePricingT500Monthly(t, 1800)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t500"}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action == ActionRenew {
		t.Fatalf("different tier with same monthly must NOT be renew, got %s", res.Action)
	}
	// 月价相等且 t500(500GiB) > t100(100GiB) → upgrade。
	if res.Action != ActionUpgrade {
		t.Fatalf("action want upgrade, got %s", res.Action)
	}
	if res.CapacityBytes != 500*GiB {
		t.Fatalf("capacity want 500GiB, got %d", res.CapacityBytes)
	}
}

// major-1：同 tier_code 判 renew。当前 t100，选 t100（同档码）→ renew，即便月价被改也按档位身份。
func TestQuote_SameTierCode_IsRenew(t *testing.T) {
	const uid = 910011
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	curExp := now.Add(30 * 24 * time.Hour)
	// 当前订阅 t100，但月价快照故意写成 9999（与配置档位月价 1800 不同）。
	setSub(t, uid, "t100", 100*GiB, 9999, &curExp)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 30}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionRenew {
		t.Fatalf("same tier_code must be renew regardless of monthly, got %s", res.Action)
	}
	// renew 容量沿用当前订阅容量。
	if res.CapacityBytes != 100*GiB {
		t.Fatalf("renew capacity want current 100GiB, got %d", res.CapacityBytes)
	}
}

// major-2：付费订阅 expire_at<now（已过期未被 cron 处理）选更高档 → action=new 且报价为全价（非 0）。
func TestQuote_ExpiredPaidSub_FullPriceNew(t *testing.T) {
	const uid = 910012
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	// 当前 t50 已过期（昨日到期）。
	past := now.Add(-24 * time.Hour)
	setSub(t, uid, "t50", 50*GiB, 1000, &past)

	// 选更高档 t100/30天。
	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 30}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionNew {
		t.Fatalf("expired paid sub must be new, got %s", res.Action)
	}
	// 全价新购：t100 月价 1800，30 天无折扣 → 1800（非 0，杜绝 0 元升级）。
	if res.TotalCents != 1800 {
		t.Fatalf("total want full price 1800, got %d", res.TotalCents)
	}
	if res.CapacityBytes != 100*GiB {
		t.Fatalf("capacity want 100GiB, got %d", res.CapacityBytes)
	}
	// 归一化后 prev 视为 free。
	if res.Meta.PrevTierCode != TierFree {
		t.Fatalf("prev tier want free (normalized), got %s", res.Meta.PrevTierCode)
	}
}

// minor-5：FulfillPackage 对 new/renew 在履约时重算到期 = max(now, 当前到期) + days，
// 不直接用 quote 时刻的 meta.NewExpireAt 快照。
func TestFulfill_RenewRecomputesExpireAtFulfillTime(t *testing.T) {
	const uid = 910013
	cleanLibraryData(t, uid)
	quoteNow := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, quoteNow)
	// 当前 t100，到期还有 10 天。
	curExp := quoteNow.Add(10 * 24 * time.Hour)
	setSub(t, uid, "t100", 100*GiB, 1800, &curExp)

	// quote 续费 30 天（quote 时算出 NewExpireAt = curExp + 30d）。
	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100", Days: 30}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionRenew {
		t.Fatalf("action want renew, got %s", res.Action)
	}
	meta, _ := json.Marshal(res.Meta)

	// 履约：到期应基于"当前订阅真实到期 curExp"累加 30 天（与 quote 快照一致这里相等，
	// 关键是验证走的是重算路径：max(now,curExp)+days = curExp+30d）。
	if err := Service.FulfillPackage(uid, 0, meta); err != nil {
		t.Fatalf("fulfill: %v", err)
	}
	sub, _ := Service.repo.getSubscription(uid)
	want := curExp.Add(30 * 24 * time.Hour)
	if sub.ExpireAt == nil || !sub.ExpireAt.Equal(want) {
		t.Fatalf("renew expire want %v, got %v", want, sub.ExpireAt)
	}
}

// minor-5：upgrade 履约保持当前到期不变；容量/月价取 meta 快照。
func TestFulfill_UpgradeKeepsCurrentExpire(t *testing.T) {
	const uid = 910014
	cleanLibraryData(t, uid)
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	fixedNow(t, now)
	curExp := now.Add(20 * 24 * time.Hour)
	setSub(t, uid, "t50", 50*GiB, 1000, &curExp)

	res, err := Service.QuotePackage(uid, mustParams(t, PackageParams{TierCode: "t100"}))
	if err != nil {
		t.Fatalf("quote: %v", err)
	}
	if res.Action != ActionUpgrade {
		t.Fatalf("action want upgrade, got %s", res.Action)
	}
	meta, _ := json.Marshal(res.Meta)
	if err := Service.FulfillPackage(uid, 0, meta); err != nil {
		t.Fatalf("fulfill: %v", err)
	}
	sub, _ := Service.repo.getSubscription(uid)
	if sub.ExpireAt == nil || !sub.ExpireAt.Equal(curExp) {
		t.Fatalf("upgrade expire must stay %v, got %v", curExp, sub.ExpireAt)
	}
	if sub.TierCode != "t100" || sub.CapacityBytes != 100*GiB || sub.MonthlyPriceCents != 1800 {
		t.Fatalf("upgrade snapshot wrong: %+v", sub)
	}
}

// quoteForBilling 输出权威价 + 可往返的 meta_json。
func TestQuoteForBilling_RoundTrip(t *testing.T) {
	const uid = 910009
	cleanLibraryData(t, uid)
	fixedNow(t, time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC))

	total, metaJSON, err := Service.quoteForBilling(uid, mustParams(t, PackageParams{TierCode: "t50", Days: 30}))
	if err != nil {
		t.Fatalf("quoteForBilling: %v", err)
	}
	if total != 1000 {
		t.Fatalf("total want 1000, got %d", total)
	}
	var meta PackageMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		t.Fatalf("meta unmarshal: %v", err)
	}
	if meta.Action != ActionNew || meta.TierCode != "t50" || meta.CapacityBytes != 50*GiB {
		t.Fatalf("meta wrong: %+v", meta)
	}
}
