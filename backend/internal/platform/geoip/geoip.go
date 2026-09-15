package geoip

import (
	"net"

	"github.com/oschwald/geoip2-golang"
)

func Lookup(reader *geoip2.Reader, ip net.IP) (string, string, string) {
	country, region, city := "unknown", "unknown", "unknown"
	if reader != nil && ip != nil {
		if record, e := reader.City(ip); e == nil {
			if record.Country.IsoCode != "" {
				country = record.Country.IsoCode
			}
			if len(record.Subdivisions) > 0 {
				region = record.Subdivisions[0].Names["en"]
			}
			if record.City.Names["en"] != "" {
				city = record.City.Names["en"]
			}
		}
	}
	return country, region, city
}
