package billing

import "time"

// 费用日志聚合（契约 §1.6）：把 runtime_charges 按「实例 + 开机会话」聚合成一行，
// 含分段（segments）与临时时长合计。

// RuntimeLogSegment 一个 quota_type 的连续分段。
type RuntimeLogSegment struct {
	QuotaType string    `json:"quota_type"`
	Minutes   int       `json:"minutes"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	// Reason 该段计费的具体原因（落账时生成），便于排查；旧数据可能为空。
	Reason string `json:"reason"`
}

// RuntimeLogItem 一行聚合的开机会话费用记录。
type RuntimeLogItem struct {
	CpID               string              `json:"cp_id"`
	InstanceName       string              `json:"instance_name"`
	PowerOnAt          time.Time           `json:"power_on_at"`
	PowerOffAt         *time.Time          `json:"power_off_at"`
	QuotaType          string              `json:"quota_type"`
	TempMinutesCharged int                 `json:"temp_minutes_charged"`
	Running            bool                `json:"running"`
	Segments           []RuntimeLogSegment `json:"segments"`
}

// RuntimeLogPage 费用日志分页结果。
type RuntimeLogPage struct {
	Items           []RuntimeLogItem `json:"items"`
	Total           int64            `json:"total"`
	DailyCapMinutes int              `json:"daily_cap_minutes"`
}

// RuntimeLog 聚合费用日志。runningRefs 为「仍在运行的会话标识」集合（由 phone 提供，可为 nil）。
// from/to 非空时按时间段筛选，只返回与该区间有重叠的开机会话。
func (s *runtimeEngineServiceImpl) RuntimeLog(userID, page, size int, runningRefs map[string]bool, from, to *time.Time) (*RuntimeLogPage, error) {
	off, lim := pageOffset(page, size)
	refs, total, err := s.charges.listSessions(userID, off, lim, from, to)
	if err != nil {
		return nil, err
	}
	cap, err := s.pricing.DailyCapMinutes()
	if err != nil {
		return nil, err
	}
	out := &RuntimeLogPage{Items: []RuntimeLogItem{}, Total: total, DailyCapMinutes: cap}
	if len(refs) == 0 {
		return out, nil
	}
	charges, err := s.charges.chargesForSessions(userID, refs)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]RuntimeCharge{}
	for _, c := range charges {
		grouped[c.RunSessionRef] = append(grouped[c.RunSessionRef], c)
	}
	for _, ref := range refs {
		cs := grouped[ref]
		if len(cs) == 0 {
			continue
		}
		out.Items = append(out.Items, aggregateSession(ref, cs, runningRefs))
	}
	enrichRuntimeLogNames(out.Items)
	return out, nil
}

// aggregateSession 把一个会话的多条 charge 聚合为一行（含分段与合计 + 整体 quota_type）。
// enrichRuntimeLogNames 用实例富化 provider 给费用日志填充实例名称。
func enrichRuntimeLogNames(items []RuntimeLogItem) {
	cpIDs := make([]string, 0, len(items))
	for _, it := range items {
		if it.CpID != "" {
			cpIDs = append(cpIDs, it.CpID)
		}
	}
	meta := lookupInstanceMeta(cpIDs)
	for i := range items {
		if m, ok := meta[items[i].CpID]; ok {
			items[i].InstanceName = m.Name
		}
	}
}

func aggregateSession(ref string, cs []RuntimeCharge, runningRefs map[string]bool) RuntimeLogItem {
	item := RuntimeLogItem{
		CpID:      cs[0].InstanceID,
		PowerOnAt: cs[0].WindowStart,
	}
	tempTotal := 0
	powerOff := cs[0].WindowEnd
	seen := map[string]bool{}
	for _, c := range cs {
		// 合并「连续同 quota_type」的分钟级 charge 为一段：段边界只看本台自身的计费结果
		// （包月/临时/封顶），不看全局名额总数——本台一直占着开机位时，总名额数从 2 变 3
		// 不该把它的记录拆成两条。同类型且时间相接则累加分钟、延伸 To；quota 变了才另起一段。
		// 合并段保留首条 charge 的 reason（代表该段开始时的具体情形）。
		n := len(item.Segments)
		if n > 0 && item.Segments[n-1].QuotaType == c.QuotaType && item.Segments[n-1].To.Equal(c.WindowStart) {
			item.Segments[n-1].Minutes += c.ChargedMinutes
			item.Segments[n-1].To = c.WindowEnd
		} else {
			item.Segments = append(item.Segments, RuntimeLogSegment{
				QuotaType: c.QuotaType, Minutes: c.ChargedMinutes,
				From: c.WindowStart, To: c.WindowEnd, Reason: c.Reason,
			})
		}
		if c.QuotaType == QuotaTemp {
			tempTotal += c.ChargedMinutes
		}
		if c.WindowStart.Before(item.PowerOnAt) {
			item.PowerOnAt = c.WindowStart
		}
		if c.WindowEnd.After(powerOff) {
			powerOff = c.WindowEnd
		}
		seen[c.QuotaType] = true
	}
	item.TempMinutesCharged = tempTotal
	// 整体 quota_type：单一则用该类型，多类混合为 mixed。
	if len(seen) == 1 {
		for k := range seen {
			item.QuotaType = k
		}
	} else {
		item.QuotaType = QuotaMixed
	}
	if runningRefs != nil && runningRefs[ref] {
		item.Running = true
		item.PowerOffAt = nil
	} else {
		po := powerOff
		item.PowerOffAt = &po
	}
	return item
}
