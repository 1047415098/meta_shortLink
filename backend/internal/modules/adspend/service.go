package adspend

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"time"
)

func Parse(reader io.Reader) ([][]string, error) {
	r := csv.NewReader(reader)
	r.FieldsPerRecord = 5
	header, e := r.Read()
	if e != nil {
		return nil, errors.New("CSV 为空或格式无效")
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	if strings.Join(header, ",") != "date,ad_id,amount,currency,time_zone" {
		return nil, errors.New("表头必须是 date,ad_id,amount,currency,time_zone")
	}
	records := [][]string{}
	seen := map[string]bool{}
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, errors.New("CSV 列数或格式错误")
		}
		if len(records) >= 10000 {
			return nil, errors.New("单次最多导入 10000 行")
		}
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		if _, e = time.Parse("2006-01-02", row[0]); e != nil {
			return nil, errors.New("日期必须是 YYYY-MM-DD")
		}
		if row[1] == "" || len(row[1]) > 120 || !amountPattern.MatchString(row[2]) || !currencyPattern.MatchString(row[3]) {
			return nil, errors.New("广告 ID、金额或币种无效；金额必须非负且最多四位小数")
		}
		if _, e = time.LoadLocation(row[4]); e != nil {
			return nil, errors.New("时区无效")
		}
		key := row[0] + "|" + row[1] + "|" + row[3] + "|" + row[4]
		if seen[key] {
			return nil, errors.New("文件包含重复的日期／广告／币种／时区记录")
		}
		seen[key] = true
		records = append(records, row)
	}
	if len(records) == 0 {
		return nil, errors.New("没有可导入的数据")
	}
	return records, nil
}
