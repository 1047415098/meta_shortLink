package links

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Scan(row pgx.Row) (l Link, e error) {
	e = row.Scan(&l.ID, &l.Code, &l.Name, &l.TargetURL, &l.Enabled, &l.CampaignID, &l.AdsetID, &l.AdID, &l.Channel, &l.CreatedAt, &l.Mode, &l.LandingBrand, &l.LandingTitle, &l.LandingDescription, &l.LandingDetails, &l.LandingDelay, &l.MetaConnectionID, &l.AttributionMode, &l.MetaPixelID, &l.TimeSpentThreshold, &l.ProductType, &l.NovelID)
	return
}

// Repository owns link persistence and its atomic audit record.
type Repository struct{ DB *pgxpool.Pool }

// ErrLinksNotFound keeps stale list selections from producing a partial delete.
var ErrLinksNotFound = errors.New("one or more links do not exist")

func (r Repository) ByCode(ctx context.Context, code string) (Link, error) {
	return Scan(r.DB.QueryRow(ctx, "SELECT "+Columns+" FROM short_links WHERE code=$1", code))
}
func (r Repository) ByID(ctx context.Context, id int64) (Link, error) {
	return Scan(r.DB.QueryRow(ctx, "SELECT "+Columns+" FROM short_links WHERE id=$1", id))
}
func (r Repository) List(ctx context.Context) ([]Link, error) {
	// 普通短链接列表不混入小说项目创建的投放链接。
	rows, e := r.DB.Query(ctx, "SELECT "+Columns+" FROM short_links WHERE product_type IN ('legacy','short_link') ORDER BY id DESC LIMIT 10000")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		l, e := Scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (r Repository) Save(ctx context.Context, l Link, update bool, actor string) (Link, error) {
	if l.ProductType == "" {
		// Existing non-HTTP callers create ordinary links and should not need project metadata.
		l.ProductType = "short_link"
	}
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return Link{}, e
	}
	defer tx.Rollback(ctx)
	if l.AttributionMode == "" {
		// Internal callers share the same new-link fallback as the HTTP API.
		l.AttributionMode = "dynamic"
	}
	// Link availability is persisted only as enabled/disabled; scheduled expiry
	// no longer participates in the repository contract.
	args := []any{l.Code, l.Name, l.TargetURL, l.Enabled, l.CampaignID, l.AdsetID, l.AdID, l.Channel, l.Mode, l.LandingBrand, l.LandingTitle, l.LandingDescription, l.LandingDetails, l.LandingDelay, l.MetaConnectionID, l.AttributionMode, l.MetaPixelID, l.TimeSpentThreshold, l.ProductType, l.NovelID}
	query := "INSERT INTO short_links(code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,mode,landing_brand,landing_title,landing_description,landing_details,landing_delay,meta_connection_id,attribution_mode,meta_pixel_id,time_spent_threshold,product_type,novel_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20) RETURNING " + Columns
	action := "link.create"
	if update {
		action = "link.update"
		args = append(args, l.ID)
		query = "UPDATE short_links SET code=$1,name=$2,target_url=$3,enabled=$4,campaign_id=$5,adset_id=$6,ad_id=$7,channel=$8,mode=$9,landing_brand=$10,landing_title=$11,landing_description=$12,landing_details=$13,landing_delay=$14,meta_connection_id=$15,attribution_mode=$16,meta_pixel_id=$17,time_spent_threshold=$18,product_type=$19,novel_id=$20 WHERE id=$21 RETURNING " + Columns
	}

	saved, e := Scan(tx.QueryRow(ctx, query, args...))
	if e != nil {
		return Link{}, e
	}
	payload, e := json.Marshal(saved)
	if e != nil {
		return Link{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, payload); e != nil {
		return Link{}, e
	}
	return saved, tx.Commit(ctx)
}

// DeleteBatch validates and deletes the complete selection in one transaction,
// including visit-scoped records that would otherwise lose their link context.
func (r Repository) DeleteBatch(ctx context.Context, ids []int64, actor string) (int64, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	orderedIDs := append([]int64(nil), ids...)
	slices.Sort(orderedIDs)
	// Project links have their own retention rules and cannot be deleted through this generic endpoint.
	rows, err := tx.Query(ctx, "SELECT id,code,name FROM short_links WHERE id=ANY($1::bigint[]) AND product_type IN ('legacy','short_link') ORDER BY id FOR UPDATE", orderedIDs)
	if err != nil {
		return 0, err
	}
	type deletedLink struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	}
	deleted := make([]deletedLink, 0, len(orderedIDs))
	paths := make([]string, 0, len(orderedIDs)*7)
	codes := make([]string, 0, len(orderedIDs))
	for rows.Next() {
		var link deletedLink
		if err = rows.Scan(&link.ID, &link.Code, &link.Name); err != nil {
			rows.Close()
			return 0, err
		}
		deleted = append(deleted, link)
		codes = append(codes, link.Code)
		// Visitor logs have no link foreign key, so every exact action path is removed explicitly.
		paths = append(paths, "/"+link.Code, "/"+link.Code+"/contact", "/"+link.Code+"/view", "/"+link.Code+"/time-spent", "/audio-novel/"+link.Code, "/audio-novel/"+link.Code+"/contact", "/audio-novel/"+link.Code+"/view", "/audio-novel/"+link.Code+"/time-spent")
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if len(deleted) != len(orderedIDs) {
		return 0, ErrLinksNotFound
	}
	// CAPI rows reference visit IDs logically rather than through a foreign key,
	// so they must be removed before their owning click events disappear.
	if _, err = tx.Exec(ctx, "DELETE FROM meta_events WHERE visit_id IN (SELECT id FROM click_events WHERE link_id=ANY($1::bigint[]))", orderedIDs); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM click_events WHERE link_id=ANY($1::bigint[])", orderedIDs); err != nil {
		return 0, err
	}
	// Story list/detail paths share the audio novel prefix and are removed with their owning code.
	if _, err = tx.Exec(ctx, `DELETE FROM request_logs WHERE path=ANY($1::text[])
		OR EXISTS(SELECT 1 FROM unnest($2::text[]) code WHERE path LIKE '/audio-novel/'||code||'/%')`, paths, codes); err != nil {
		return 0, err
	}
	result, err := tx.Exec(ctx, "DELETE FROM short_links WHERE id=ANY($1::bigint[])", orderedIDs)
	if err != nil {
		return 0, err
	}
	if result.RowsAffected() != int64(len(orderedIDs)) {
		return 0, ErrLinksNotFound
	}
	detail, err := json.Marshal(map[string]any{"count": len(deleted), "links": deleted})
	if err != nil {
		return 0, err
	}
	// One audit entry records the exact batch without retaining deleted visit details.
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,'link.delete_batch',$2)", actor, detail); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int64(len(deleted)), nil
}
