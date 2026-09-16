package app

import (
	"context"
	"encoding/json"
	"testing"
)

// TestBatchDeleteLinksIsAtomicAndRemovesOwnedData catches partial deletion and
// orphaned analytics, CAPI events or visitor request logs after a link is removed.
func TestBatchDeleteLinksIsAtomicAndRemovesOwnedData(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	_, err := a.DB.Exec(ctx, `
		INSERT INTO short_links(id,code,name,target_url) VALUES
			(2,'delete-a','Delete A','https://wa.me/12345678'),
			(3,'delete-b','Delete B','https://wa.me/12345678');
		INSERT INTO meta_connections(id,name,account_id) VALUES(1,'Delete account','delete-account');
		INSERT INTO click_events(id,link_id,visitor_id,cookie_status,method,target_url,device,os,browser,country,region,city,source,campaign_id,adset_id,ad_id,referrer,classification,reason,event_type) VALUES
			('delete-visit-a',2,'person-a','issued','GET','https://wa.me/12345678','','','','','','','','','','','','normal','','landing'),
			('delete-visit-b',3,'person-b','issued','GET','https://wa.me/12345678','','','','','','','','','','','','normal','','landing');
		INSERT INTO meta_events(id,connection_id,visit_id,event_name,event_time,pixel_id) VALUES
			('delete-event-a',1,'delete-visit-a','PageView',now(),'pixel-a'),
			('delete-event-b',1,'delete-visit-b','PageView',now(),'pixel-b');
		INSERT INTO request_logs(id,method,path,client_ip,status,duration_ms,request_headers,query,request_body,response_headers,response_body,request_bytes,response_bytes,is_visitor) VALUES
			('delete-log-a','GET','/delete-a','192.0.2.1',200,1,'{}','{}','{}','{}','{}',0,0,true),
			('delete-log-a-contact','POST','/delete-a/contact','192.0.2.1',303,1,'{}','{}','{}','{}','{}',0,0,true),
			('delete-log-b-view','POST','/delete-b/view','192.0.2.1',200,1,'{}','{}','{}','{}','{}',0,0,true),
			('keep-log','GET','/hello','192.0.2.1',200,1,'{}','{}','{}','{}','{}',0,0,true);`)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/links/batch-delete"
	if w := call(a, "POST", path, `{"ids":[2]}`, nil); w.Code != 401 {
		t.Fatalf("anonymous delete: %d %s", w.Code, w.Body.String())
	}
	admin := login(t, a)
	for _, body := range []string{`{"ids":[]}`, `{"ids":[0]}`, `{"ids":[2,2]}`} {
		if w := call(a, "POST", path, body, admin); w.Code != 400 {
			t.Fatalf("invalid delete %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	if w := call(a, "POST", path, `{"ids":[2,999]}`, admin); w.Code != 404 {
		t.Fatalf("missing link delete: %d %s", w.Code, w.Body.String())
	}
	var before int
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM short_links WHERE id IN (2,3)").Scan(&before); err != nil || before != 2 {
		t.Fatalf("missing link caused partial delete: count=%d err=%v", before, err)
	}
	w := call(a, "POST", path, `{"ids":[2,3]}`, admin)
	if w.Code != 200 {
		t.Fatalf("batch delete: %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Deleted != 2 {
		t.Fatalf("delete response: %s err=%v", w.Body.String(), err)
	}
	for label, query := range map[string]string{
		"links":       "SELECT count(*) FROM short_links WHERE id IN (2,3)",
		"clicks":      "SELECT count(*) FROM click_events WHERE link_id IN (2,3)",
		"meta events": "SELECT count(*) FROM meta_events WHERE visit_id IN ('delete-visit-a','delete-visit-b')",
		"visit logs":  "SELECT count(*) FROM request_logs WHERE path LIKE '/delete-a%' OR path LIKE '/delete-b%'",
	} {
		var count int
		if err := a.DB.QueryRow(ctx, query).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s remain after delete: count=%d err=%v", label, count, err)
		}
	}
	var auditCount, keepLogs int
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE action='link.delete_batch'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("delete audit missing: count=%d err=%v", auditCount, err)
	}
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM request_logs WHERE id='keep-log'").Scan(&keepLogs); err != nil || keepLogs != 1 {
		t.Fatalf("unrelated request log deleted: count=%d err=%v", keepLogs, err)
	}
}
