package models

import (
	"fmt"
	"strconv"
	"strings"
	"sublink/node"

	"gorm.io/gorm"
)

func cdnNodeName(name string, index int) string {
	return fmt.Sprintf("%s · %02d", name, index+1)
}

func renameCDNLink(link, name string) (string, error) {
	template, err := node.ParseCDNTemplate(link)
	if err != nil {
		return "", err
	}
	link, _ = template.WithIP(template.Server, name)
	return link, nil
}

func nameCDNResults(p *CDNProfile) error {
	for i := range p.Results {
		r := &p.Results[i]
		r.Name = cdnNodeName(p.Name, i)
		link, err := renameCDNLink(r.Link, r.Name)
		if err != nil {
			return err
		}
		r.Link = link
	}
	return nil
}

// Only nodes owned by this profile move groups. Other manually assigned groups
// and subscription membership stay intact, including when legacy groups are shared.
func syncCDNNaming(tx *gorm.DB, p *CDNProfile) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("请填写配置名称")
	}
	if err := nameCDNResults(p); err != nil {
		return err
	}
	var nodes []Node
	if err := tx.Preload("GroupNodes").Where("source = ?", fmt.Sprintf("cdn:%d", p.ID)).Order("id").Find(&nodes).Error; err != nil {
		return err
	}
	names := map[string]string{}
	for _, r := range p.Results {
		names[r.IP] = r.Name
	}
	if len(nodes) > 0 {
		// Legacy subscriptions may still order nodes by their old names.
		ids := make([]int, 0, len(nodes))
		for _, n := range nodes {
			ids = append(ids, n.ID)
		}
		var subs []Subcription
		linked := tx.Table("subcription_nodes").Select("subcription_id").Where("node_id IN ?", ids)
		if err := tx.Preload("Nodes").Where("id IN (?) AND (node_order_ids = '' OR node_order_ids IS NULL)", linked).Find(&subs).Error; err != nil {
			return err
		}
		for _, sub := range subs {
			order := []string{}
			for _, n := range orderSubscriptionNodes(sub.Nodes, sub.NodeOrderIDs, sub.NodeOrder) {
				order = append(order, strconv.Itoa(n.ID))
			}
			if err := tx.Model(&sub).Update("node_order_ids", strings.Join(order, ",")).Error; err != nil {
				return err
			}
		}
		group := GroupNode{Name: p.Name}
		if err := tx.Where("name = ?", p.Name).FirstOrCreate(&group).Error; err != nil {
			return err
		}
		removed := map[int]GroupNode{}
		for _, n := range nodes {
			name, ok := names[n.SourceKey]
			if !ok {
				name = cdnNodeName(p.Name, len(names))
				names[n.SourceKey] = name
			}
			link, err := renameCDNLink(n.Link, name)
			if err != nil {
				return err
			}
			override := n.LinkOverride
			if override != "" {
				override, err = renameCDNLink(override, name)
				if err != nil {
					return err
				}
			}
			n.Name, n.Link, n.LinkOverride = name, link, override
			if err := tx.Model(&n).Updates(map[string]interface{}{"name": name, "link": link, "link_override": override}).Error; err != nil {
				return err
			}
			oldGroups := append([]GroupNode(nil), n.GroupNodes...)
			if err := tx.Model(&n).Association("GroupNodes").Append(&group); err != nil {
				return err
			}
			for _, old := range oldGroups {
				if old.ID != group.ID && (old.Name == p.GroupName || (p.NamingVersion == 0 && old.Name == "CDN 优选")) {
					if err := tx.Model(&n).Association("GroupNodes").Delete(&old); err != nil {
						return err
					}
					removed[old.ID] = old
				}
			}
		}
		for _, old := range removed {
			association := tx.Model(&old).Association("Nodes")
			count := association.Count()
			if association.Error != nil {
				return association.Error
			}
			if count == 0 {
				if err := tx.Delete(&old).Error; err != nil {
					return err
				}
			}
		}
	}
	p.GroupName, p.NamingVersion = p.Name, 1
	return nil
}

func migrateCDNNaming() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var profiles []CDNProfile
		if err := tx.Where("naming_version = 0 OR naming_version IS NULL").Order("id").Find(&profiles).Error; err != nil {
			return err
		}
		for _, p := range profiles {
			if err := syncCDNNaming(tx, &p); err != nil {
				return fmt.Errorf("配置 %d 命名迁移失败：%w", p.ID, err)
			}
			if err := tx.Save(&p).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
