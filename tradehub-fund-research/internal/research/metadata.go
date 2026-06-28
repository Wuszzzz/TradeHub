package research

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

var seedFundRelated = map[string]string{
	"000008": "中证500指数",
	"000051": "沪深300指数",
	"000055": "纳斯达克100指数",
	"000248": "中证主要消费指数",
	"000596": "中证军工指数",
	"000942": "中证全指信息技术指数",
	"000950": "沪深300非银行金融指数",
	"001051": "上证50指数",
	"001064": "中证环保产业指数",
	"001180": "中证全指医药卫生指数",
	"001481": "道琼斯美国石油开发与生产指数",
	"005827": "沪深300指数",
	"110011": "沪深300指数",
	"161725": "中证白酒指数",
	"161726": "中证生物医药指数",
	"163406": "中证红利指数",
	"260104": "沪深300指数",
	"320007": "中证军工指数",
	"519674": "中证白酒指数",
}

var seedSectorSecIDs = map[string]string{
	"上证综合指数":         "1.000001",
	"上证50指数":         "1.000016",
	"沪深300指数":        "1.000300",
	"中证500指数":        "1.000905",
	"中证800指数":        "1.000906",
	"中证1000指数":       "1.000852",
	"创业板指数":          "0.399006",
	"创业板指数(价格)":      "0.399006",
	"深证成份指数":         "0.399001",
	"深证成份指数(价格)":     "0.399001",
	"中证主要消费指数":       "1.000932",
	"中证白酒指数":         "2.399997",
	"中证医药100指数":      "1.000978",
	"中证全指医药卫生指数":     "1.000991",
	"沪深300医药卫生指数":    "1.000913",
	"中证全指信息技术指数":     "1.000993",
	"中证全指可选消费指数":     "1.000989",
	"中证全指金融地产指数":     "1.000992",
	"沪深300非银行金融指数":   "1.000849",
	"中证银行指数":         "0.399986",
	"中证军工指数":         "0.399967",
	"中证环保产业指数":       "1.000827",
	"中证养老产业指数":       "0.399812",
	"中证大农业指数":        "0.399814",
	"中证传媒指数":         "0.399971",
	"中证全指证券公司指数":     "0.399975",
	"中证红利指数":         "1.000922",
	"中证红利低波动指数":      "2.H30269",
	"纳斯达克100指数":      "100.NDX",
	"纳斯达克生物科技指数":     "251.NBI",
	"恒生指数":           "100.HSI",
	"恒生中国企业指数":       "100.HSCEI",
	"标普500指数":        "100.SPX",
	"黄金9999":         "118.AU9999",
	"道琼斯美国石油开发与生产指数": "107.IEO",
}

func (s *Server) relatedSectorMap(ctx context.Context, codes []string) (map[string]RelatedSector, error) {
	normalized := normalizeCodes(codes)
	if len(normalized) == 0 {
		return map[string]RelatedSector{}, nil
	}
	result := map[string]RelatedSector{}
	for _, code := range normalized {
		if sector := strings.TrimSpace(seedFundRelated[code]); sector != "" {
			result[code] = RelatedSector{FundCode: code, Sector: sector, SecID: seedSectorSecIDs[sector], Source: "seed"}
		}
	}
	if s.db == nil {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `select fund_code, related_sector from fund_research_related_sector where fund_code = any($1)`, pq.Array(normalized))
	if err != nil {
		if isUndefinedTable(err) {
			return result, nil
		}
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var code, sector string
		if err := rows.Scan(&code, &sector); err != nil {
			return result, err
		}
		code = strings.TrimSpace(code)
		sector = strings.TrimSpace(sector)
		if code == "" || sector == "" {
			continue
		}
		result[code] = RelatedSector{FundCode: code, Sector: sector, SecID: seedSectorSecIDs[sector], Source: "db"}
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	labels := make([]string, 0, len(result))
	for _, row := range result {
		if row.SecID == "" {
			labels = append(labels, row.Sector)
		}
	}
	if len(labels) > 0 {
		secids, err := s.sectorSecIDMap(ctx, labels)
		if err == nil {
			for code, row := range result {
				if row.SecID == "" {
					row.SecID = secids[row.Sector]
					result[code] = row
				}
			}
		}
	}
	return result, nil
}

func (s *Server) sectorSecIDMap(ctx context.Context, labels []string) (map[string]string, error) {
	normalized := normalizeLabels(labels)
	result := map[string]string{}
	for _, label := range normalized {
		if secid := strings.TrimSpace(seedSectorSecIDs[label]); secid != "" {
			result[label] = secid
		}
	}
	if s.db == nil {
		return result, nil
	}
	rows, err := s.db.QueryContext(ctx, `select sector_name, secid from fund_research_sector_secid where sector_name = any($1)`, pq.Array(normalized))
	if err != nil {
		if isUndefinedTable(err) {
			return result, nil
		}
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var label, secid string
		if err := rows.Scan(&label, &secid); err != nil {
			return result, err
		}
		label = strings.TrimSpace(label)
		secid = strings.TrimSpace(secid)
		if label != "" && secid != "" {
			result[label] = secid
		}
	}
	return result, rows.Err()
}

func (s *Server) recommendTagsForFund(ctx context.Context, code string) ([]FundTag, error) {
	related, err := s.relatedSectorMap(ctx, []string{code})
	if err != nil {
		return nil, err
	}
	row, ok := related[code]
	if !ok || row.Sector == "" {
		return []FundTag{}, nil
	}
	tags := []FundTag{{
		ID:       "sector_" + stableID(row.Sector),
		Name:     row.Sector,
		Theme:    "sector",
		Reason:   "关联板块",
		FundCode: code,
	}}
	for _, tag := range tagsFromSectorName(row.Sector, code) {
		tags = append(tags, tag)
	}
	return dedupeTags(tags), nil
}

func tagsFromSectorName(sector, code string) []FundTag {
	rules := []struct {
		Contains string
		Name     string
		Theme    string
	}{
		{"沪深300", "核心宽基", "broad"},
		{"中证500", "中盘宽基", "broad"},
		{"中证1000", "小盘宽基", "broad"},
		{"上证50", "大盘蓝筹", "broad"},
		{"纳斯达克", "海外科技", "global"},
		{"恒生", "港股", "global"},
		{"标普", "美股", "global"},
		{"黄金", "商品黄金", "commodity"},
		{"医药", "医药医疗", "industry"},
		{"消费", "消费", "industry"},
		{"白酒", "白酒消费", "industry"},
		{"军工", "军工", "industry"},
		{"信息技术", "科技", "industry"},
		{"传媒", "传媒", "industry"},
		{"银行", "金融", "industry"},
		{"证券", "金融", "industry"},
		{"红利", "红利低波", "style"},
	}
	tags := make([]FundTag, 0)
	for _, rule := range rules {
		if strings.Contains(sector, rule.Contains) {
			tags = append(tags, FundTag{
				ID:       rule.Theme + "_" + stableID(rule.Name),
				Name:     rule.Name,
				Theme:    rule.Theme,
				Reason:   "由关联板块推导",
				FundCode: code,
			})
		}
	}
	return tags
}

func (s *Server) syncSectorRows(ctx context.Context, rows []RelatedSector) (int, error) {
	if s.db == nil {
		return 0, errors.New("postgres unavailable")
	}
	if err := s.ensureMetadataTables(ctx); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	count := 0
	for _, row := range rows {
		code := strings.TrimSpace(row.FundCode)
		sector := strings.TrimSpace(row.Sector)
		if code == "" || sector == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			insert into fund_research_related_sector(fund_code, related_sector, updated_at)
			values($1, $2, now())
			on conflict(fund_code) do update set related_sector = excluded.related_sector, updated_at = now()
		`, code, sector); err != nil {
			return count, err
		}
		if secid := strings.TrimSpace(row.SecID); secid != "" {
			if _, err := tx.ExecContext(ctx, `
				insert into fund_research_sector_secid(sector_name, secid, updated_at)
				values($1, $2, now())
				on conflict(sector_name) do update set secid = excluded.secid, updated_at = now()
			`, sector, secid); err != nil {
				return count, err
			}
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}

func (s *Server) ensureMetadataTables(ctx context.Context) error {
	if s.db == nil {
		return errors.New("postgres unavailable")
	}
	_, err := s.db.ExecContext(ctx, `
		create table if not exists fund_research_related_sector (
			fund_code varchar primary key,
			related_sector text not null,
			updated_at timestamptz not null default now()
		);
		create table if not exists fund_research_sector_secid (
			sector_name text primary key,
			secid varchar not null,
			updated_at timestamptz not null default now()
		);
		create table if not exists fund_research_sync_runs (
			id bigserial primary key,
			sync_type varchar not null,
			item_count integer not null default 0,
			started_at timestamptz not null default now(),
			finished_at timestamptz,
			status varchar not null default 'running',
			message text
		);
	`)
	return err
}

func (s *Server) metadataSyncStatus(ctx context.Context) SyncStatus {
	status := SyncStatus{
		SeedRelatedCount: len(seedFundRelated),
		SeedSecIDCount:   len(seedSectorSecIDs),
		DBAvailable:      s.db != nil,
		Mode:             "seed",
	}
	if s.db == nil {
		return status
	}
	for _, table := range []string{"fund_research_related_sector", "fund_research_sector_secid", "fund_research_sync_runs"} {
		var exists bool
		err := s.db.QueryRowContext(ctx, `select to_regclass($1) is not null`, "public."+table).Scan(&exists)
		if err == nil && exists {
			status.DBTables = append(status.DBTables, table)
		}
	}
	if len(status.DBTables) > 0 {
		status.Mode = "db+seed"
	}
	return status
}

func normalizeCodes(codes []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func normalizeLabels(labels []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

func dedupeTags(tags []FundTag) []FundTag {
	seen := map[string]struct{}{}
	out := make([]FundTag, 0, len(tags))
	for _, tag := range tags {
		key := tag.ID
		if key == "" {
			key = tag.Name
		}
		if _, ok := seen[key]; ok || strings.TrimSpace(tag.Name) == "" {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func stableID(value string) string {
	replacer := strings.NewReplacer(" ", "_", "(", "", ")", "", "（", "", "）", "", "/", "_", "&", "and")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}

func intString(value int) string {
	return strconv.Itoa(value)
}

func isUndefinedTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "does not exist")
}
