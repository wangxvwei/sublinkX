package models

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IDs identify the original providers, not mirror URLs. A second mirror must
// share its provider ID so it cannot earn another corroboration bonus.
var cdnFeeds = []struct{ ID, Name, URL string }{
	{"uouin", "麒麟优选", cdnRecommendationSource},
	{"wetest", "微测 WeTest", "https://bestcf.pages.dev/wetest/ipv4.txt"},
	{"cfyes", "CFYes", "https://bestcf.pages.dev/cfyes/ipv4.txt"},
}

type CDNFeedStatus struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	Count     int        `json:"count"`
	Error     string     `json:"error,omitempty"`
	Cached    bool       `json:"cached,omitempty"`
}

type cdnCandidate struct {
	IP, Carrier, Source string
	Port                int
	UpdatedAt           time.Time
	Quality             float64
	Cached              bool
}

type cdnCachedFeed struct {
	Data []byte
	Err  error
	At   time.Time
}

var cdnFeedCache struct {
	sync.Mutex
	Entries map[string]cdnCachedFeed
}

func fetchCDNFeed(ctx context.Context, address string) ([]byte, error) {
	cdnFeedCache.Lock()
	cached, ok := cdnFeedCache.Entries[address]
	cdnFeedCache.Unlock()
	ttl := 10 * time.Minute
	if cached.Err != nil {
		ttl = time.Minute
	}
	if ok && time.Since(cached.At) < ttl {
		return cached.Data, cached.Err
	}
	data, err := cdnHTTP(ctx, address)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	cdnFeedCache.Lock()
	defer cdnFeedCache.Unlock()
	// ponytail: only three fixed public feeds; add eviction if custom feeds exist.
	if cdnFeedCache.Entries == nil {
		cdnFeedCache.Entries = make(map[string]cdnCachedFeed)
	}
	cdnFeedCache.Entries[address] = cdnCachedFeed{data, err, time.Now()}
	return data, err
}

func readCDNFeed(data []byte, source string, ranges []netip.Prefix, now time.Time) ([]cdnCandidate, time.Time, error) {
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	header := strings.Split(lines[0], "|")
	if len(header) != 3 || len(lines) > 1000 {
		return nil, time.Time{}, errors.New("公开推荐数据格式无效")
	}
	zone := time.FixedZone("UTC+8", 8*3600)
	localNow := now.In(zone)
	updated, err := time.ParseInLocation("2006 01-02 15:04", fmt.Sprintf("%d %s", localNow.Year(), strings.TrimSpace(header[1])), zone)
	if err == nil && localNow.Month() == time.January && updated.Month() == time.December {
		updated = updated.AddDate(-1, 0, 0)
	}
	if err != nil || now.Sub(updated) > 48*time.Hour || updated.After(now.Add(5*time.Minute)) {
		return nil, updated, errors.New("上游数据超过 48 小时或更新时间无效")
	}
	carriers := map[string]string{"电信": "CT", "联通": "CU", "移动": "CM"}
	candidates := []cdnCandidate{}
	seen, totals := map[string]bool{}, map[string]int{}
	for _, line := range lines[1:] {
		address, description, ok := strings.Cut(line, "#")
		fields := strings.Split(description, "|")
		expected, ipColumn := 5, 2
		switch source {
		case "wetest":
			expected, ipColumn = 4, 3
		case "cfyes":
			expected = 3
		case "uouin":
		default:
			return nil, updated, errors.New("未知推荐来源")
		}
		if !ok || len(fields) != expected {
			continue
		}
		carrier := carriers[strings.TrimSpace(fields[1])]
		host, rawPort, err := net.SplitHostPort(strings.TrimSpace(address))
		if err != nil || carrier == "" {
			continue
		}
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 1 || port > 65535 {
			continue
		}
		ip, err := PublicCDNIP(host)
		if err != nil || !netip.MustParseAddr(ip).Is4() || strings.TrimSpace(fields[ipColumn]) != ip || !inCDNRanges(netip.MustParseAddr(ip), ranges) {
			continue
		}
		if source == "uouin" {
			valid := true
			for i, suffix := range map[int]string{3: "ms", 4: "mb/s"} {
				value, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(fields[i])), suffix)), 64)
				if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
					valid = false
				}
			}
			if !valid {
				continue
			}
		}
		key := carrier + "/" + strconv.Itoa(port)
		identity := key + "/" + ip
		if seen[identity] {
			continue
		}
		seen[identity] = true
		totals[key]++
		candidates = append(candidates, cdnCandidate{IP: ip, Carrier: carrier, Source: source, Port: port, UpdatedAt: updated, Quality: float64(totals[key])})
	}
	for i := range candidates {
		c := &candidates[i]
		total := totals[c.Carrier+"/"+strconv.Itoa(c.Port)]
		c.Quality = 1 - (c.Quality-1)/float64(total)
	}
	if len(candidates) == 0 {
		return nil, updated, errors.New("来源没有有效的 Cloudflare 地址")
	}
	return candidates, updated, nil
}

func collectCDNFeeds(ctx context.Context, ranges []netip.Prefix, now time.Time, mixed bool) ([]cdnCandidate, []CDNFeedStatus) {
	feeds := cdnFeeds
	if !mixed {
		feeds = feeds[:1]
	}
	var wg sync.WaitGroup
	all := make([][]cdnCandidate, len(feeds))
	statuses := make([]CDNFeedStatus, len(feeds))
	for i, feed := range feeds {
		wg.Add(1)
		go func(i int, id, name, address string) {
			defer wg.Done()
			status := CDNFeedStatus{ID: id, Name: name, URL: address}
			data, err := fetchCDNFeed(ctx, address)
			if err == nil {
				var updated time.Time
				all[i], updated, err = readCDNFeed(data, id, ranges, now)
				if !updated.IsZero() {
					status.UpdatedAt = &updated
				}
			}
			if err != nil {
				status.Error = err.Error()
			}
			statuses[i] = status
		}(i, feed.ID, feed.Name, feed.URL)
	}
	wg.Wait()
	candidates := []cdnCandidate{}
	for _, rows := range all {
		candidates = append(candidates, rows...)
	}
	return candidates, statuses
}
