package models

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sort"
	"sublink/node"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The source's own timestamp determines the window: refreshing a cached feed,
// restarting the app, or running another profile cannot create extra history.
type CDNObservation struct {
	Source   string    `gorm:"primaryKey"`
	Carrier  string    `gorm:"primaryKey"`
	Port     int       `gorm:"primaryKey;autoIncrement:false"`
	IP       string    `gorm:"primaryKey"`
	Window   time.Time `gorm:"primaryKey;index"`
	SampleAt time.Time
	Quality  float64
}

func saveCDNObservations(candidates []cdnCandidate, now time.Time) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("window < ?", now.UTC().Truncate(30*time.Minute).Add(-7*24*time.Hour)).Delete(&CDNObservation{}).Error; err != nil {
			return err
		}
		observations := make([]CDNObservation, 0, len(candidates))
		for _, c := range candidates {
			observations = append(observations, CDNObservation{Source: c.Source, Carrier: c.Carrier, Port: c.Port, IP: c.IP,
				Window: c.UpdatedAt.UTC().Truncate(30 * time.Minute), SampleAt: c.UpdatedAt.UTC(), Quality: c.Quality})
		}
		if len(observations) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(observations, 100).Error
	})
}

func rankCDNRecommendations(ctx context.Context, p *CDNProfile, template node.CDNTemplate, ranges []netip.Prefix, now time.Time) ([]CDNResult, error) {
	candidates, statuses := collectCDNFeeds(ctx, ranges, now, p.RecommendationMode == "mixed")
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := saveCDNObservations(candidates, now); err != nil {
		return nil, fmt.Errorf("保存公开推荐历史失败：%w", err)
	}
	var observations []CDNObservation
	if err := DB.Where("carrier = ? AND port = ? AND window >= ?", p.Network.Carrier, template.Port, now.UTC().Truncate(30*time.Minute).Add(-7*24*time.Hour)).Find(&observations).Error; err != nil {
		return nil, err
	}
	for i := range statuses {
		s := &statuses[i]
		if s.Error != "" {
			latest := time.Time{}
			for _, o := range observations {
				if o.Source == s.ID && now.Sub(o.SampleAt) <= 48*time.Hour && !o.SampleAt.After(now.Add(5*time.Minute)) && o.SampleAt.After(latest) {
					latest = o.SampleAt
				}
			}
			if !latest.IsZero() {
				s.Cached, s.UpdatedAt = true, &latest
				for _, o := range observations {
					if o.Source == s.ID && o.SampleAt.Equal(latest) {
						candidates = append(candidates, cdnCandidate{IP: o.IP, Carrier: o.Carrier, Port: o.Port, Source: o.Source, UpdatedAt: o.SampleAt, Quality: o.Quality, Cached: true})
					}
				}
			}
		}
		for _, c := range candidates {
			if c.Source == s.ID && c.Carrier == p.Network.Carrier && c.Port == template.Port && c.IP != p.SourceIP {
				s.Count++
			}
		}
		if s.Count > 0 && s.UpdatedAt != nil && (p.Network.SourceUpdatedAt == nil || s.UpdatedAt.After(*p.Network.SourceUpdatedAt)) {
			p.Network.SourceUpdatedAt = s.UpdatedAt
		}
	}
	p.Network.Sources = statuses
	return selectCDNRecommendations(p, template, ranges, candidates, observations, now)
}

func selectCDNRecommendations(p *CDNProfile, template node.CDNTemplate, ranges []netip.Prefix, candidates []cdnCandidate, observations []CDNObservation, now time.Time) ([]CDNResult, error) {
	mixed := p.RecommendationMode == "mixed"
	p.Network.RecommendationMode = "single"
	if mixed {
		p.Network.RecommendationMode = "mixed"
	}
	type history struct {
		Windows map[time.Time]bool
		Quality float64
		Samples int
	}
	histories := map[string]*history{}
	sourceWindows := map[string]map[time.Time]bool{}
	allWindows := map[time.Time]bool{}
	for _, o := range observations {
		if !mixed && o.Source != "uouin" {
			continue
		}
		if o.Carrier != p.Network.Carrier || o.Port != template.Port || o.Window.Before(now.UTC().Truncate(30*time.Minute).Add(-7*24*time.Hour)) {
			continue
		}
		if sourceWindows[o.Source] == nil {
			sourceWindows[o.Source] = map[time.Time]bool{}
		}
		sourceWindows[o.Source][o.Window], allWindows[o.Window] = true, true
		key := o.Source + "/" + o.IP
		if histories[key] == nil {
			histories[key] = &history{Windows: map[time.Time]bool{}}
		}
		h := histories[key]
		if !h.Windows[o.Window] {
			h.Windows[o.Window] = true
			h.Quality += o.Quality
			h.Samples++
		}
	}
	p.Network.HistoryWindows = len(allWindows)
	type ranked struct {
		Result   CDNResult
		BySource map[string]cdnCandidate
		Score    float64
	}
	byIP := map[string]*ranked{}
	for _, c := range candidates {
		if !mixed && c.Source != "uouin" {
			continue
		}
		if c.Carrier != p.Network.Carrier || c.Port != template.Port || c.IP == p.SourceIP || now.Sub(c.UpdatedAt) > 48*time.Hour || c.UpdatedAt.After(now.Add(5*time.Minute)) {
			continue
		}
		ip, err := PublicCDNIP(c.IP)
		if err != nil || !inCDNRanges(netip.MustParseAddr(ip), ranges) {
			continue
		}
		if byIP[ip] == nil {
			byIP[ip] = &ranked{Result: CDNResult{IP: ip, Recommendation: true}, BySource: map[string]cdnCandidate{}}
		}
		r := byIP[ip]
		old, ok := r.BySource[c.Source]
		if !ok || c.UpdatedAt.After(old.UpdatedAt) || (c.UpdatedAt.Equal(old.UpdatedAt) && c.Quality > old.Quality) {
			r.BySource[c.Source] = c
		}
	}
	p.Network.CandidateCount = len(byIP)
	for _, ip := range p.PinnedIPs {
		parsed, err := PublicCDNIP(ip)
		if err != nil || parsed != ip || ip == p.SourceIP || !inCDNRanges(netip.MustParseAddr(ip), ranges) {
			return nil, fmt.Errorf("保留 IP %s 不再符合 Cloudflare 地址要求，请先取消保留", ip)
		}
		if byIP[ip] == nil {
			byIP[ip] = &ranked{Result: CDNResult{IP: ip, Recommendation: true}, BySource: map[string]cdnCandidate{}}
		}
		byIP[ip].Result.Pinned = true
	}
	if len(byIP) == 0 {
		return nil, errors.New("所选公开来源未取得该运营商、端口的新鲜有效地址；已有节点保留，请稍后重试或切换推荐模式")
	}
	items := []*ranked{}
	for ip, r := range byIP {
		windows := map[time.Time]bool{}
		stability := 0.0
		for _, feed := range cdnFeeds {
			c, ok := r.BySource[feed.ID]
			if !ok {
				continue
			}
			name := feed.Name
			if c.Cached {
				name += "（缓存）"
				r.Result.Cached = true
			}
			r.Result.Sources = append(r.Result.Sources, name)
			if r.Result.SourceUpdatedAt == nil || c.UpdatedAt.After(*r.Result.SourceUpdatedAt) {
				at := c.UpdatedAt
				r.Result.SourceUpdatedAt = &at
			}
			freshness := 40 * math.Max(0, math.Min(1, 1-now.Sub(c.UpdatedAt).Hours()/48))
			r.Score = math.Max(r.Score, freshness+25*c.Quality)
			if h := histories[feed.ID+"/"+ip]; h != nil {
				for window := range h.Windows {
					windows[window] = true
				}
				if total := len(sourceWindows[feed.ID]); total >= 3 && h.Samples >= 2 {
					stability = math.Max(stability, 15*float64(h.Samples)/float64(total)*(h.Quality/float64(h.Samples))*math.Min(1, float64(h.Samples-1)/4))
				}
			}
		}
		// ponytail: a transparent ranking heuristic, not calibrated speed prediction;
		// tune weights only after collecting actual target-network feedback.
		r.Score += math.Min(20, 10*math.Max(0, float64(len(r.BySource)-1))) + stability
		if !mixed {
			// Restore the original provider order; history must not reorder single-source results.
			r.Score = r.BySource["uouin"].Quality
		}
		r.Result.HistorySamples = len(windows)
		r.Result.Basis = fmt.Sprintf("%d 个公开来源 · 上榜 %d 期", len(r.BySource), len(windows))
		if !mixed {
			r.Result.Basis = fmt.Sprintf("麒麟原始顺序 · 上榜 %d 期", len(windows))
		}
		if r.Result.Cached {
			r.Result.Basis += " · 含新鲜缓存"
		}
		if len(r.BySource) == 0 {
			r.Result.Basis = "用户保留 · 本次来源未收录"
		}
		items = append(items, r)
	}
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Result.Pinned != b.Result.Pinned {
			return a.Result.Pinned
		}
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Result.IP < b.Result.IP
	})
	selected, used, prefixes := []CDNResult{}, map[string]bool{}, map[string]int{}
	for pass := 0; pass < 2; pass++ {
		for _, r := range items {
			if len(selected) == p.Count {
				break
			}
			ip := r.Result.IP
			prefix := netip.PrefixFrom(netip.MustParseAddr(ip), 24).Masked().String()
			if used[ip] || (mixed && pass == 0 && !r.Result.Pinned && prefixes[prefix] >= 2) {
				continue
			}
			result := r.Result
			result.Link, result.Name = template.WithIP(ip, cdnNodeName(p.Name, len(selected)))
			selected = append(selected, result)
			used[ip], prefixes[prefix] = true, prefixes[prefix]+1
		}
	}
	return selected, nil
}

func SetCDNPinnedIP(id uint, rawIP string, keep bool) (CDNProfile, error) {
	cdnTask.Lock()
	defer cdnTask.Unlock()
	var p CDNProfile
	if cdnTask.id == id {
		return p, errors.New("任务运行中，请等待完成再修改保留 IP")
	}
	if err := DB.First(&p, id).Error; err != nil {
		return p, err
	}
	if p.CandidateMode != "recommend" {
		return p, errors.New("保留好用 IP 仅用于异地线路推荐")
	}
	ip, err := PublicCDNIP(rawIP)
	if err != nil {
		return p, err
	}
	known, pinned := false, false
	for _, r := range p.Results {
		if r.IP == ip && r.Recommendation {
			known = true
		}
	}
	for _, old := range p.PinnedIPs {
		if old == ip {
			pinned = true
		}
	}
	if keep && !known && !pinned {
		return p, errors.New("只能保留当前配置已生成的推荐 IP")
	}
	if keep && !pinned {
		if len(p.PinnedIPs) >= p.Count {
			return p, errors.New("保留数量已达到推荐数量，请先取消部分保留")
		}
		p.PinnedIPs = append(p.PinnedIPs, ip)
	}
	if !keep {
		ips := []string{}
		for _, old := range p.PinnedIPs {
			if old != ip {
				ips = append(ips, old)
			}
		}
		p.PinnedIPs = ips
	}
	for i := range p.Results {
		if p.Results[i].IP == ip {
			p.Results[i].Pinned = keep
		}
	}
	return p, DB.Save(&p).Error
}
